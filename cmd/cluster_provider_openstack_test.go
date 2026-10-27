package cmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cloudopenstack "github.com/opencenter-cloud/opencenter-cli/internal/cloud/openstack"
	provideropenstack "github.com/opencenter-cloud/opencenter-cli/internal/cluster/provider/openstack"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	testhelpers "github.com/opencenter-cloud/opencenter-cli/internal/testing"

	"github.com/spf13/cobra"
)

func TestClusterProviderOpenStackCommandHierarchy(t *testing.T) {
	cluster := NewClusterCmd()
	provider, _, err := cluster.Find([]string{"provider"})
	if err != nil || provider == nil {
		t.Fatalf("cluster provider command not registered: command=%v err=%v", provider, err)
	}
	openstack, _, err := cluster.Find([]string{"provider", "openstack"})
	if err != nil || openstack == nil {
		t.Fatalf("cluster provider openstack command not registered: command=%v err=%v", openstack, err)
	}
	for _, operation := range []string{"plan", "apply"} {
		operationCmd, _, err := cluster.Find([]string{"provider", "openstack", operation})
		if err != nil || operationCmd == nil {
			t.Fatalf("cluster provider openstack %s command not registered: command=%v err=%v", operation, operationCmd, err)
		}
		for _, flag := range []string{"os-cloud", "clouds-yaml", "image-name", "windows-image-name", "network-id", "external-network-id", "subnet-id", "availability-zone", "replace", "import-auth", "import-tls", "create-application-credential"} {
			if operationCmd.Flags().Lookup(flag) == nil {
				t.Fatalf("%s missing --%s", operation, flag)
			}
		}
	}
}

func TestClusterProviderOpenStackGuardRunsBeforeProfileAccess(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)
	_, clusterPaths := saveKindConfigForCommandTest(t, dir, "prod", "acme")

	root := NewBuiltinRootCmd()
	root.SetArgs([]string{"cluster", "provider", "openstack", "plan", "acme/prod", "--os-cloud", "missing", "--clouds-yaml", clusterPaths.ConfigPath + ".missing"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "requires provider openstack") {
		t.Fatalf("error = %v, want provider guard before profile access", err)
	}
}

func TestConfirmProviderApplyUsesStderrAndYesBypassesPrompt(t *testing.T) {
	cmd := &cobra.Command{}
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetIn(strings.NewReader("yes\n"))
	cmd.SetContext(context.WithValue(context.Background(), globalOptionsContextKey{}, GlobalOptions{Output: OutputText}))
	confirmed, err := confirmProviderApply(cmd)
	if err != nil {
		t.Fatal(err)
	}
	if !confirmed || !strings.Contains(stderr.String(), "Apply the provider-only OpenStack patch?") {
		t.Fatalf("confirmation = %v, stderr = %q", confirmed, stderr.String())
	}

	yesCmd := &cobra.Command{}
	yesCmd.SetContext(context.WithValue(context.Background(), globalOptionsContextKey{}, GlobalOptions{Yes: true, Output: OutputText}))
	confirmed, err = confirmProviderApply(yesCmd)
	if err != nil || !confirmed {
		t.Fatalf("--yes confirmation = %v, err = %v", confirmed, err)
	}
}

func TestProviderApplyReviewIsStderrOnlyAndRedacted(t *testing.T) {
	cmd := &cobra.Command{}
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	renderProviderApplyReview(cmd, provideropenstack.Result{
		Changes:  []provideropenstack.Change{{Path: "secret", Old: "<redacted>", New: "<redacted>"}},
		Warnings: []string{"profile secret was redacted"},
	})
	if stdout.Len() != 0 || !strings.Contains(stderr.String(), "Provider-only OpenStack apply review") || strings.Contains(stderr.String(), "super-secret") {
		t.Fatalf("review stream or redaction incorrect: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestProviderExitClassification(t *testing.T) {
	if got := ExitCode(NewExitError(2, "blocked", nil)); got != 2 {
		t.Fatalf("blocked exit code = %d, want 2", got)
	}
	missing := v2.NewConfigNotFoundError("prod", errors.New("not found"))
	if got := ExitCode(missing); got != 3 {
		t.Fatalf("missing config exit code = %d, want 3", got)
	}
	if got := ExitCode(errors.New("operational failure")); got != 1 {
		t.Fatalf("operational exit code = %d, want 1", got)
	}
}

func TestProviderMissingClusterIsConfigNotFound(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)
	root := NewBuiltinRootCmd()
	root.SetArgs([]string{"cluster", "provider", "openstack", "plan", "missing", "--os-cloud", "prod", "--clouds-yaml", filepath.Join(dir, "clouds.yaml")})
	err := root.Execute()
	var missing *v2.ConfigNotFoundError
	if !errors.As(err, &missing) || ExitCode(err) != 3 {
		t.Fatalf("missing cluster error = %v, type=%T, exit=%d", err, err, ExitCode(err))
	}
}

func TestStructuredProviderApplyRequiresYes(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetContext(context.WithValue(context.Background(), globalOptionsContextKey{}, GlobalOptions{Output: OutputJSON}))
	if err := validateProviderApplyInteraction(cmd); err == nil || ExitCode(err) != 2 || !strings.Contains(err.Error(), "requires --yes") {
		t.Fatalf("structured apply guard = %v, exit=%d", err, ExitCode(err))
	}
}

func TestClusterProviderOpenStackCreateInternalNetworkFlagIsShared(t *testing.T) {
	cluster := NewClusterCmd()
	for _, operation := range []string{"plan", "apply"} {
		operationCmd, _, err := cluster.Find([]string{"provider", "openstack", operation})
		if err != nil || operationCmd == nil {
			t.Fatalf("find %s: command=%v err=%v", operation, operationCmd, err)
		}
		flag := operationCmd.Flags().Lookup("create-internal-network")
		// The flag defaults ON: OpenTofu-managed internal networking is the
		// recommended mode, and existing-network selection is the opt-out.
		if flag == nil || flag.DefValue != "true" {
			t.Fatalf("%s missing shared --create-internal-network flag: %#v", operation, flag)
		}
	}
}

func TestWriteProviderOpenStackOutputIncludesInternalNetworkMode(t *testing.T) {
	cmd := &cobra.Command{}
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	if err := writeProviderOpenStackOutput(cmd, OutputText, provideropenstack.Result{
		Operation:           "cluster.provider.openstack.plan",
		Cluster:             "acme/prod",
		Status:              provideropenstack.StatusNoOp,
		InternalNetworkMode: "tofu-managed",
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "Internal network mode: tofu-managed") {
		t.Fatalf("text output = %q", stdout.String())
	}
}

// saveOpenStackConfigForCommandTest writes a minimal openstack-provider
// cluster config so runClusterProviderOpenStack can plan against it.
func saveOpenStackConfigForCommandTest(t *testing.T, dir, clusterName, organization string) {
	t.Helper()
	resolver, clusterPaths := createClusterDirectoriesForTest(t, dir, clusterName, organization)
	cfgPtr, err := v2.NewV2Default(clusterName, "openstack")
	if err != nil {
		t.Fatalf("create native v2 openstack config: %v", err)
	}
	cfg := *cfgPtr
	cfg.OpenCenter.Meta.Name = clusterName
	cfg.OpenCenter.Meta.Organization = organization
	cfg.OpenCenter.Infrastructure.Provider = "openstack"
	cfg.OpenCenter.GitOps.Repository.LocalDir = clusterPaths.GitOpsDir
	testhelpers.SaveConfigWithPathResolver(t, cfg, resolver)
}

// TestProviderOpenStackEnsuresCredentialBeforeDiscovery pins the auth half of
// the --os-cloud contract: a profile without an application credential pair
// gets one minted (through the EnsureProfileAuth seam) BEFORE any remote
// call, so a bare 'cluster init --os-cloud' must not leave the readiness gate
// complaining about missing application_credential_id/secret.
func TestProviderOpenStackEnsuresCredentialBeforeDiscovery(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)
	saveOpenStackConfigForCommandTest(t, dir, "prod", "acme")
	cloudsPath := filepath.Join(dir, "clouds.yaml")
	body := "clouds:\n  prod:\n    auth:\n      auth_url: https://keystone.invalid.local/v3\n      username: u\n      password: p\n"
	if err := os.WriteFile(cloudsPath, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	original := provideropenstack.EnsureProfileAuth
	ensureCalls := 0
	provideropenstack.EnsureProfileAuth = func(_ context.Context, profile cloudopenstack.Profile, name string) (cloudopenstack.AppCredential, cloudopenstack.Profile, error) {
		ensureCalls++
		if name == "" {
			return cloudopenstack.AppCredential{}, profile, errors.New("default credential name not passed")
		}
		profile.AppCredID = "minted-id"
		profile.AppCredSecret = "minted-secret"
		return cloudopenstack.AppCredential{ID: profile.AppCredID, Secret: profile.AppCredSecret}, profile, nil
	}
	t.Cleanup(func() { provideropenstack.EnsureProfileAuth = original })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	err := runClusterProviderOpenStack(cmd, "plan", false, "acme/prod", providerOpenStackFlags{
		cloudName: "prod", cloudsYAML: cloudsPath,
	})
	// Discovery fails against the invalid auth_url; what matters is that the
	// credential step ran first and did not block the flow.
	if ensureCalls != 1 {
		t.Fatalf("EnsureProfileAuth calls = %d, want 1 (err=%v)", ensureCalls, err)
	}
	if err == nil || !strings.Contains(err.Error(), "discover OpenStack provider inventory") {
		t.Fatalf("error = %v, want discovery to be attempted after minting", err)
	}
	// The plan pass itself must never rewrite clouds.yaml.
	data, readErr := os.ReadFile(cloudsPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(data) != body {
		t.Fatalf("plan modified clouds.yaml:\n%s", data)
	}
}

// TestProviderOpenStackMintsOnCreateApplicationCredentialEvenWithPair proves
// --create-application-credential forces rotation: a profile that already has
// a pair still goes through the minting seam, and without the flag an
// existing pair must not trigger minting.
func TestProviderOpenStackMintsOnCreateApplicationCredentialEvenWithPair(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)
	saveOpenStackConfigForCommandTest(t, dir, "prod", "acme")
	cloudsPath := filepath.Join(dir, "clouds.yaml")
	if err := os.WriteFile(cloudsPath, []byte("clouds:\n  prod:\n    auth:\n      auth_url: https://keystone.invalid.local/v3\n      application_credential_id: old-id\n      application_credential_secret: old-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	original := provideropenstack.EnsureProfileAuth
	ensureCalls := 0
	provideropenstack.EnsureProfileAuth = func(_ context.Context, profile cloudopenstack.Profile, _ string) (cloudopenstack.AppCredential, cloudopenstack.Profile, error) {
		ensureCalls++
		return cloudopenstack.AppCredential{ID: "fresh", Secret: "fresh"}, profile, nil
	}
	t.Cleanup(func() { provideropenstack.EnsureProfileAuth = original })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	err := runClusterProviderOpenStack(cmd, "plan", false, "acme/prod", providerOpenStackFlags{
		cloudName: "prod", cloudsYAML: cloudsPath, createApplicationCredential: true,
	})
	if ensureCalls != 1 {
		t.Fatalf("EnsureProfileAuth calls = %d, want 1 (err=%v)", ensureCalls, err)
	}

	ensureCalls = 0
	err = runClusterProviderOpenStack(cmd, "plan", false, "acme/prod", providerOpenStackFlags{
		cloudName: "prod", cloudsYAML: cloudsPath,
	})
	if ensureCalls != 0 {
		t.Fatalf("EnsureProfileAuth calls = %d, want 0 without the flag (err=%v)", ensureCalls, err)
	}
}
