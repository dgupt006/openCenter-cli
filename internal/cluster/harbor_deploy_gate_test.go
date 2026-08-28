package cluster

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	testhelpers "github.com/opencenter-cloud/opencenter-cli/internal/testing"
)

type harborGateRunner struct {
	calls int
}

func (r *harborGateRunner) Run(context.Context, string, map[string]string, string, ...string) ([]byte, error) {
	r.calls++
	return nil, nil
}

func harborBootstrapConfig(t *testing.T, clusterName, organization, target string, configured bool) v2.Config {
	t.Helper()
	cfg := mustNewClusterTestConfig(clusterName, "kind")
	cfg.OpenCenter.Meta.Organization = organization
	cfg.OpenCenter.GitOps.Repository.LocalDir = filepath.Join(t.TempDir(), "gitops")
	cfg.OpenCenter.Services["harbor"].(*services.HarborConfig).Enabled = false
	cfg.OpenCenter.ManagedServices = make(v2.ServiceMap)

	harbor := &services.HarborConfig{BaseConfig: services.BaseConfig{Enabled: true}}
	switch target {
	case "regular":
		cfg.OpenCenter.Services["harbor"] = harbor
	case "managed":
		cfg.OpenCenter.ManagedServices["harbor"] = harbor
	case "disabled":
	default:
		t.Fatalf("unknown Harbor target %q", target)
	}
	if configured {
		cfg.Secrets.Harbor = v2.HarborSecrets{
			AdminPassword:    "admin-configured",
			RegistryPassword: "registry-configured",
			DatabasePassword: "database-configured",
		}
	}
	return cfg
}

func TestBootstrapRejectsInvalidRegularHarborBeforeLifecycleRunner(t *testing.T) {
	root := t.TempDir()
	clusterName := "harbor-gate-regular"
	organization := "test-org"
	resolver := paths.NewPathResolver(root)
	if err := resolver.CreateClusterDirectories(context.Background(), clusterName, organization); err != nil {
		t.Fatalf("create cluster directories: %v", err)
	}
	cfg := harborBootstrapConfig(t, clusterName, organization, "regular", false)
	testhelpers.SaveConfigWithPathResolver(t, cfg, resolver)

	service := createTestBootstrapService(resolver)
	runner := &harborGateRunner{}
	service.runner = runner
	result, err := service.Bootstrap(context.Background(), BootstrapOptions{
		ClusterName:    clusterName,
		Organization:   organization,
		SkipValidation: true,
	})
	if err == nil {
		t.Fatal("Bootstrap() accepted enabled Harbor placeholders")
	}
	if result != nil {
		t.Fatalf("Bootstrap() result = %#v, want nil before runtime initialization", result)
	}
	for _, path := range []string{
		"secrets.harbor.admin_password",
		"secrets.harbor.registry_password",
		"secrets.harbor.database_password",
	} {
		if !strings.Contains(err.Error(), path) {
			t.Fatalf("Bootstrap() error = %v, missing %q", err, path)
		}
	}
	if runner.calls != 0 {
		t.Fatalf("lifecycle runner calls = %d, want 0", runner.calls)
	}
}

func TestBootstrapHarborGateAcceptsDisabledOrConfigured(t *testing.T) {
	tests := []struct {
		name       string
		target     string
		configured bool
	}{
		{name: "disabled placeholders", target: "disabled", configured: false},
		{name: "regular configured", target: "regular", configured: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			clusterName := "harbor-accept-" + strings.ReplaceAll(tt.name, " ", "-")
			organization := "test-org"
			resolver := paths.NewPathResolver(root)
			if err := resolver.CreateClusterDirectories(context.Background(), clusterName, organization); err != nil {
				t.Fatalf("create cluster directories: %v", err)
			}
			cfg := harborBootstrapConfig(t, clusterName, organization, tt.target, tt.configured)
			testhelpers.SaveConfigWithPathResolver(t, cfg, resolver)

			service := createTestBootstrapService(resolver)
			result, err := service.Bootstrap(context.Background(), BootstrapOptions{
				ClusterName:    clusterName,
				Organization:   organization,
				DryRun:         true,
				SkipValidation: true,
			})
			if err != nil {
				t.Fatalf("Bootstrap() rejected valid Harbor state: %v", err)
			}
			if result == nil || result.Plan == nil {
				t.Fatalf("Bootstrap() result = %#v, want dry-run plan", result)
			}
		})
	}
}

func TestBootstrapRejectsManagedHarborBeforeLifecycleRunner(t *testing.T) {
	root := t.TempDir()
	clusterName := "managed-harbor-unsupported"
	organization := "test-org"
	resolver := paths.NewPathResolver(root)
	if err := resolver.CreateClusterDirectories(context.Background(), clusterName, organization); err != nil {
		t.Fatalf("create cluster directories: %v", err)
	}
	cfg := harborBootstrapConfig(t, clusterName, organization, "managed", true)
	testhelpers.SaveConfigWithPathResolver(t, cfg, resolver)

	service := createTestBootstrapService(resolver)
	runner := &harborGateRunner{}
	service.runner = runner
	result, err := service.Bootstrap(context.Background(), BootstrapOptions{
		ClusterName:    clusterName,
		Organization:   organization,
		SkipValidation: true,
	})
	if err == nil || !strings.Contains(err.Error(), "managed Harbor is not supported") {
		t.Fatalf("Bootstrap() error = %v", err)
	}
	if result != nil {
		t.Fatalf("Bootstrap() result = %#v, want nil before runtime initialization", result)
	}
	if runner.calls != 0 {
		t.Fatalf("lifecycle runner calls = %d, want 0", runner.calls)
	}
}
