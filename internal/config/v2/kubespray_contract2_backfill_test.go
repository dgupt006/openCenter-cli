package v2

import (
	"strings"
	"testing"
)

// Legacy config: deployment.kubespray block without module sources must get
// the contract v2 pin backfilled at load, so a re-render resolves a module
// that accepts inventory_output_dir.
func TestNormalizeBackfillsKubesprayModuleSource(t *testing.T) {
	legacy := `---
schema_version: "2.0"
opencenter:
  meta:
    name: argus
deployment:
  auto_deploy: true
  method: kubespray
  kubespray:
    version: 2.31.0
`
	cfg, err := DecodePublicConfig([]byte(legacy))
	if err != nil {
		t.Fatal(err)
	}
	cl := NewConfigLoader(nil)
	if err := cl.normalize(cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Deployment.Kubespray == nil {
		t.Fatal("nil kubespray")
	}
	if !strings.Contains(cfg.Deployment.Kubespray.KubesprayCluster.Source, "2026.10.21-contract2") {
		t.Fatalf("kubespray_cluster source not backfilled: %q", cfg.Deployment.Kubespray.KubesprayCluster.Source)
	}
	if !strings.Contains(cfg.Deployment.Kubespray.Modules["kubespray"].Source, "2026.10.21-contract2") {
		t.Fatalf("modules.kubespray source not backfilled: %#v", cfg.Deployment.Kubespray.Modules)
	}
}

// Explicit user pin must survive untouched.
func TestNormalizeKeepsExplicitKubesprayModuleSource(t *testing.T) {
	cfg, err := DecodePublicConfig([]byte(`---
schema_version: "2.0"
opencenter:
  meta:
    name: argus
deployment:
  auto_deploy: true
  method: kubespray
  kubespray:
    version: 2.31.0
    kubespray_cluster:
      source: github.com/example/fork.git//iac/provider/kubespray?ref=my-branch
      enabled: true
`))
	if err != nil {
		t.Fatal(err)
	}
	cl := NewConfigLoader(nil)
	if err := cl.normalize(cfg); err != nil {
		t.Fatal(err)
	}
	if got := cfg.Deployment.Kubespray.KubesprayCluster.Source; got != "github.com/example/fork.git//iac/provider/kubespray?ref=my-branch" {
		t.Fatalf("explicit source overwritten: %q", got)
	}
}
