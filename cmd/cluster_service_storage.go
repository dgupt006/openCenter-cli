package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	cloudopenstack "github.com/opencenter-cloud/opencenter-cli/internal/cloud/openstack"
	storageopenstack "github.com/opencenter-cloud/opencenter-cli/internal/cluster/storage/openstack"
	"github.com/opencenter-cloud/opencenter-cli/internal/config/defaults"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/security"
	"github.com/spf13/cobra"
)

type clusterServiceStorageFlags struct {
	cluster           string
	cloudName         string
	cloudsYAML        string
	backend           string
	container         string
	s3Endpoint        string
	rotateCredentials bool
}

func newClusterServiceStorageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "storage",
		Short: "Manage one service's OpenStack storage",
		Args:  cobra.NoArgs,
		RunE:  func(cmd *cobra.Command, args []string) error { return cmd.Help() },
	}
	cmd.AddCommand(newClusterServiceStorageOperationCmd("plan", false))
	cmd.AddCommand(newClusterServiceStorageOperationCmd("apply", true))
	return cmd
}

func newClusterServiceStorageOperationCmd(operation string, mutating bool) *cobra.Command {
	var flags clusterServiceStorageFlags
	cmd := &cobra.Command{
		Use:   operation + " <service>",
		Short: operation + " one service's OpenStack storage",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runClusterServiceStorage(cmd, operation, mutating, args[0], flags)
		},
	}
	cmd.Flags().StringVar(&flags.cluster, "cluster", "", "cluster identifier")
	cmd.Flags().StringVar(&flags.cloudName, "os-cloud", "", "clouds.yaml profile to use")
	cmd.Flags().StringVar(&flags.cloudsYAML, "clouds-yaml", "", "path to clouds.yaml")
	cmd.Flags().StringVar(&flags.backend, "backend", "", "OpenStack storage backend: swift or s3")
	cmd.Flags().StringVar(&flags.container, "container", "", "container or bucket name")
	cmd.Flags().StringVar(&flags.s3Endpoint, "s3-endpoint", "", "distinct S3-compatible endpoint URL")
	cmd.Flags().BoolVar(&flags.rotateCredentials, "rotate-credentials", false, "replace an existing credential pair")
	_ = cmd.MarkFlagRequired("cluster")
	_ = cmd.MarkFlagRequired("backend")
	return cmd
}

func runClusterServiceStorage(cmd *cobra.Command, operation string, mutating bool, service string, flags clusterServiceStorageFlags) error {
	opts := storageopenstack.Options{Service: service, Backend: flags.backend, Cluster: flags.cluster, Container: flags.container, S3Endpoint: flags.s3Endpoint, RotateCredentials: flags.rotateCredentials}
	if err := storageopenstack.ValidateOptions(opts); err != nil {
		return NewExitError(2, "invalid storage selection", err)
	}
	cfg, raw, configPath, err := loadStorageConfig(cmd.Context(), flags.cluster)
	if err != nil {
		if isMissingConfigError(err) {
			return v2.NewConfigNotFoundError(flags.cluster, err)
		}
		return NewExitError(2, "load cluster configuration", err)
	}
	if strings.ToLower(strings.TrimSpace(cfg.OpenCenter.Infrastructure.Provider)) != "openstack" {
		return NewExitError(2, fmt.Sprintf("cluster provider is %q; OpenStack storage requires provider openstack", cfg.OpenCenter.Infrastructure.Provider), nil)
	}
	if _, err := storageServiceForCommand(cfg, service); err != nil {
		return NewExitError(2, "invalid storage service", err)
	}
	if strings.TrimSpace(flags.cloudName) == "" {
		return NewExitError(2, "--os-cloud is required", nil)
	}
	cloudsPath := strings.TrimSpace(flags.cloudsYAML)
	if cloudsPath == "" {
		cloudsPath = cloudopenstack.DefaultCloudsYAMLPath()
	}
	profile, err := cloudopenstack.LoadProfile(cloudsPath, flags.cloudName)
	if err != nil {
		return NewExitError(1, "load OpenStack cloud profile", err)
	}
	adapter := cloudopenstack.NewStorageAdapter(profile)
	planned, err := storageopenstack.Plan(cmd.Context(), storageopenstack.PlanInput{Config: cfg, Options: opts, Adapter: adapter})
	if err != nil {
		return NewExitError(1, "plan OpenStack storage", err)
	}
	planned.Result.Operation = "cluster.service.storage." + operation
	if planned.Result.Status == storageopenstack.StatusBlocked {
		if err := writeStorageOutput(cmd, getGlobalOptions(cmd).Output, planned.Result); err != nil {
			return err
		}
		return NewExitError(2, "storage plan is blocked", nil)
	}
	if !mutating || getGlobalOptions(cmd).DryRun {
		return writeStorageOutput(cmd, getGlobalOptions(cmd).Output, planned.Result)
	}
	if getGlobalOptions(cmd).Output != OutputText && !getGlobalOptions(cmd).Yes {
		return NewExitError(2, "structured apply requires --yes; refusing interactive confirmation", nil)
	}
	renderStorageReview(cmd, planned.Result)
	confirmed, err := confirmStorageApply(cmd)
	if err != nil {
		return err
	}
	if !confirmed {
		planned.Result.Status = storageopenstack.StatusDeclined
		planned.Result.Warnings = append(planned.Result.Warnings, "apply declined; configuration and remote resources were not changed")
		return writeStorageOutput(cmd, getGlobalOptions(cmd).Output, planned.Result)
	}
	validate := func(candidate *v2.Config) error {
		data, err := v2.MarshalPublicConfig(candidate)
		if err != nil {
			return err
		}
		_, err = v2.NewConfigLoader(defaults.NewRegistry()).LoadFromBytes(data)
		return err
	}
	result, applyErr := storageopenstack.Apply(cmd.Context(), storageopenstack.ApplyInput{
		ConfigPath: configPath, RecoveryPath: configPath + ".storage-recovery.json", OriginalBytes: raw,
		Options: opts, Adapter: adapter, FileSystem: storageopenstack.OSFileSystem{}, Validate: validate,
	})
	if applyErr != nil {
		if result.Status == storageopenstack.StatusPartial {
			_ = writeStorageOutput(cmd, getGlobalOptions(cmd).Output, result)
			return NewExitError(4, "OpenStack storage apply partially completed", sanitizeStorageError(applyErr))
		}
		return NewExitError(1, "apply OpenStack storage", sanitizeStorageError(applyErr))
	}
	return writeStorageOutput(cmd, getGlobalOptions(cmd).Output, result)
}

func loadStorageConfig(ctx context.Context, identifier string) (*v2.Config, []byte, string, error) {
	_, _, _, paths, err := loadNativeV2ConfigWithIdentifier(ctx, identifier)
	if err != nil {
		return nil, nil, "", err
	}
	raw, err := os.ReadFile(paths.ConfigPath)
	if err != nil {
		return nil, nil, "", err
	}
	public, err := v2.DecodePublicConfig(raw)
	if err != nil {
		return nil, nil, "", err
	}
	return public, raw, paths.ConfigPath, nil
}

func storageServiceForCommand(cfg *v2.Config, service string) (any, error) {
	value, ok := cfg.OpenCenter.Services[service]
	if !ok || value == nil {
		return nil, fmt.Errorf("service %q is not configured", service)
	}
	return value, nil
}

func isMissingConfigError(err error) bool {
	return errors.Is(err, os.ErrNotExist) || strings.Contains(strings.ToLower(err.Error()), "not found")
}

func renderStorageReview(cmd *cobra.Command, result storageopenstack.Result) {
	stderr := cmd.ErrOrStderr()
	fmt.Fprintln(stderr, "OpenStack storage apply review:")
	fmt.Fprintf(stderr, "  service: %s\n  backend: %s\n  container: %s\n", result.Service, result.Backend, result.Container)
	for _, change := range result.Changes {
		fmt.Fprintf(stderr, "  %s: %s -> %s\n", change.Path, change.Old, change.New)
	}
	for _, action := range result.RemoteActions {
		fmt.Fprintf(stderr, "  remote[%d]: %s %s", action.Order, action.Action, action.Resource)
		if action.Name != "" {
			fmt.Fprintf(stderr, " %s", action.Name)
		}
		if action.ID != "" {
			fmt.Fprintf(stderr, " id=%s", action.ID)
		}
		fmt.Fprintln(stderr)
	}
	for _, warning := range result.Warnings {
		fmt.Fprintf(stderr, "  warning: %s\n", warning)
	}
}

func confirmStorageApply(cmd *cobra.Command) (bool, error) {
	if getGlobalOptions(cmd).Yes {
		return true, nil
	}
	fmt.Fprint(cmd.ErrOrStderr(), "Apply this OpenStack storage plan? [y/N] ")
	answer, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if err != nil && len(answer) == 0 {
		return false, err
	}
	answer = strings.TrimSpace(answer)
	return strings.EqualFold(answer, "y") || strings.EqualFold(answer, "yes"), nil
}

func writeStorageOutput(cmd *cobra.Command, format OutputFormat, result storageopenstack.Result) error {
	if format != OutputText {
		return writeStructuredOutput(cmd, format, result)
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "OpenStack storage %s for %s (%s): %s\n", result.Operation, result.Service, result.Backend, result.Status)
	fmt.Fprintf(out, "Container: %s\nChanges:\n", result.Container)
	for _, change := range result.Changes {
		fmt.Fprintf(out, "  %s: %s -> %s\n", change.Path, change.Old, change.New)
	}
	fmt.Fprintln(out, "Remote actions:")
	for _, action := range result.RemoteActions {
		fmt.Fprintf(out, "  %d. %s %s", action.Order, action.Action, action.Resource)
		if action.Name != "" {
			fmt.Fprintf(out, " %s", action.Name)
		}
		fmt.Fprintln(out)
	}
	for _, warning := range result.Warnings {
		fmt.Fprintf(out, "warning: %s\n", warning)
	}
	return nil
}

func sanitizeStorageError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s", security.MaskSecrets(err.Error(), "[generated]"))
}
