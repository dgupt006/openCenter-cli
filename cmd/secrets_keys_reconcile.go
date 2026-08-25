package cmd

import (
	"fmt"

	"github.com/opencenter-cloud/opencenter-cli/internal/secrets"
	"github.com/spf13/cobra"
)

func newSecretsKeysReconcileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reconcile",
		Short: "Reconcile SOPS recipients with the key registry",
		Args:  cobra.NoArgs,
		RunE:  runSecretsKeysReconcile,
	}
	cmd.Flags().String("cluster", "", "cluster name or organization/cluster")
	cmd.Flags().Bool("apply", false, "Import recipients missing from the active registry")
	_ = cmd.MarkFlagRequired("cluster")
	return cmd
}

func runSecretsKeysReconcile(cmd *cobra.Command, args []string) error {
	cluster, err := cmd.Flags().GetString("cluster")
	if err != nil {
		return err
	}
	apply, err := cmd.Flags().GetBool("apply")
	if err != nil {
		return err
	}

	ctx := cmd.Context()
	cfg, err := loadConfig(ctx, cluster)
	if err != nil {
		return fmt.Errorf("failed to load cluster configuration: %w", err)
	}
	if cfg.Secrets.SopsAgeKeyFile == "" {
		return fmt.Errorf("cluster %q does not configure a SOPS Age key file", cluster)
	}
	if err := setupSOPSKeyEnvironment(cfg.Secrets.SopsAgeKeyFile); err != nil {
		return fmt.Errorf("failed to setup key environment: %w", err)
	}

	logger := createSecretsLogger()
	configLoader := createConfigLoader()
	sopsManager := createSOPSManager(logger)
	registryPath, err := getSecretsRegistryPath()
	if err != nil {
		return err
	}
	registry := secrets.NewDefaultKeyRegistry(registryPath, &sopsEncryptorAdapter{manager: sopsManager}, logger)
	secretsManager := secrets.NewDefaultSecretsManager(configLoader, sopsManager, nil, logger)
	reconciler := secrets.NewDefaultKeyReconciler(registry, secretsManager, logger)

	report, err := reconciler.Reconcile(ctx, cluster, apply)
	if err != nil {
		return fmt.Errorf("failed to reconcile key registry: %w", err)
	}
	displayReconcileReport(cmd, report, apply)

	if !apply && report.HasDrift() {
		return fmt.Errorf("key registry drift detected; run 'opencenter secrets keys reconcile --cluster %s --apply' to import missing recipients", cluster)
	}
	return nil
}

func displayReconcileReport(cmd *cobra.Command, report *secrets.ReconcileReport, apply bool) {
	out := cmd.OutOrStdout()
	mode := "report-only"
	if apply {
		mode = "apply"
	}
	fmt.Fprintf(out, "Key registry reconciliation for cluster %s (%s):\n", report.Cluster, mode)
	fmt.Fprintf(out, "SOPS config: %s\n\n", report.SOPSConfigPath)
	fmt.Fprintln(out, "Summary:")
	fmt.Fprintf(out, "  In both: %d\n", len(report.InBoth))
	fmt.Fprintf(out, "  Only in SOPS config: %d\n", len(report.OnlyInSOPSConfig))
	fmt.Fprintf(out, "  Only in registry: %d\n", len(report.OnlyInRegistry))
	fmt.Fprintf(out, "  Duplicate fingerprints: %d\n", len(report.DuplicateFingerprints))
	fmt.Fprintf(out, "  Revoked/archived but still in SOPS config: %d\n", len(report.RecipientsRevokedButStillInSOPSConfig))
	fmt.Fprintf(out, "  Imported: %d\n", len(report.Imported))

	for _, recipient := range report.OnlyInSOPSConfig {
		if reconcileReportContains(report.Imported, recipient) {
			fmt.Fprintf(out, "  Unregistered recipient: %s (imported with --apply)\n", recipient)
		} else {
			fmt.Fprintf(out, "  Unregistered recipient: %s (will be imported with --apply)\n", recipient)
		}
	}
	for _, recipient := range report.OnlyInRegistry {
		fmt.Fprintf(out, "  Registry-only active recipient: %s (not removed)\n", recipient)
	}
	for _, recipient := range report.RecipientsRevokedButStillInSOPSConfig {
		fmt.Fprintf(out, "  Revoked/archived recipient still in SOPS config: %s\n", recipient)
	}
	for _, fingerprint := range report.DuplicateFingerprints {
		fmt.Fprintf(out, "  Duplicate registry fingerprint: %s\n", fingerprint)
	}
}

func reconcileReportContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
