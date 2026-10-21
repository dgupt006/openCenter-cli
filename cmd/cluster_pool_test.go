package cmd

import (
	"bytes"
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

func TestClusterPoolAdd_Linux(t *testing.T) {
	cfgDir := t.TempDir()
	writeTestConfig(t, cfgDir, "demo", "openstack", "")
	prepareCommandTestEnv(t, cfgDir)

	cmd := newClusterPoolCmd()
	cmd.SetContext(context.WithValue(context.Background(), globalOptionsContextKey{}, GlobalOptions{DryRun: true, Output: OutputText}))
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"add", "gpu-pool", "--cluster=demo", "--flavor=gpu.0.4.16", "--count=3"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("pool add failed: %v", err)
	}
	if !strings.Contains(out.String(), "Would add linux pool") {
		t.Fatalf("expected dry-run output, got: %s", out.String())
	}
}

func TestClusterPoolAdd_Windows(t *testing.T) {
	cfgDir := t.TempDir()
	writeTestConfig(t, cfgDir, "demo", "openstack", "")
	prepareCommandTestEnv(t, cfgDir)

	cmd := newClusterPoolCmd()
	cmd.SetContext(context.WithValue(context.Background(), globalOptionsContextKey{}, GlobalOptions{DryRun: true, Output: OutputText}))
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"add", "win-pool", "--cluster=demo", "--flavor=gp.0.4.16", "--os=windows", "--count=2"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("pool add windows failed: %v", err)
	}
	if !strings.Contains(out.String(), "Would add windows pool") {
		t.Fatalf("expected dry-run output mentioning windows, got: %s", out.String())
	}
}

func TestClusterPoolAdd_DuplicateName(t *testing.T) {
	cfgDir := t.TempDir()
	writeTestConfig(t, cfgDir, "demo", "openstack", "")
	prepareCommandTestEnv(t, cfgDir)

	// First add (non-dry-run to persist)
	cmd1 := newClusterPoolCmd()
	cmd1.SetContext(context.WithValue(context.Background(), globalOptionsContextKey{}, GlobalOptions{DryRun: false, Output: OutputText}))
	cmd1.SetOut(&bytes.Buffer{})
	cmd1.SetArgs([]string{"add", "my-pool", "--cluster=demo", "--flavor=m1.large"})
	if err := cmd1.Execute(); err != nil {
		t.Fatalf("first pool add failed: %v", err)
	}

	// Second add same name should fail
	cmd2 := newClusterPoolCmd()
	cmd2.SetContext(context.WithValue(context.Background(), globalOptionsContextKey{}, GlobalOptions{DryRun: false, Output: OutputText}))
	cmd2.SetOut(&bytes.Buffer{})
	cmd2.SetArgs([]string{"add", "my-pool", "--cluster=demo", "--flavor=m1.large"})
	err := cmd2.Execute()
	if err == nil {
		t.Fatal("expected error for duplicate pool name, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClusterPoolRemove_BlocksNonZeroCount(t *testing.T) {
	cfgDir := t.TempDir()
	writeTestConfig(t, cfgDir, "demo", "openstack", "")
	prepareCommandTestEnv(t, cfgDir)

	// Add a pool with count > 0
	cmdAdd := newClusterPoolCmd()
	cmdAdd.SetContext(context.WithValue(context.Background(), globalOptionsContextKey{}, GlobalOptions{DryRun: false, Output: OutputText}))
	cmdAdd.SetOut(&bytes.Buffer{})
	cmdAdd.SetArgs([]string{"add", "active-pool", "--cluster=demo", "--flavor=m1.large", "--count=2"})
	if err := cmdAdd.Execute(); err != nil {
		t.Fatalf("pool add failed: %v", err)
	}

	// Try to remove without scale-to-zero
	cmdRm := newClusterPoolCmd()
	cmdRm.SetContext(context.WithValue(context.Background(), globalOptionsContextKey{}, GlobalOptions{DryRun: false, Output: OutputText}))
	cmdRm.SetOut(&bytes.Buffer{})
	cmdRm.SetArgs([]string{"remove", "active-pool", "--cluster=demo"})
	err := cmdRm.Execute()
	if err == nil {
		t.Fatal("expected error for removing pool with count > 0")
	}
	if !strings.Contains(err.Error(), "Scale to zero first") {
		t.Fatalf("expected scale-to-zero guidance, got: %v", err)
	}
}

func TestClusterPoolRemove_AllowsZeroCount(t *testing.T) {
	cfgDir := t.TempDir()
	writeTestConfig(t, cfgDir, "demo", "openstack", "")
	prepareCommandTestEnv(t, cfgDir)

	// Add a pool with count=0
	cmdAdd := newClusterPoolCmd()
	cmdAdd.SetContext(context.WithValue(context.Background(), globalOptionsContextKey{}, GlobalOptions{DryRun: false, Output: OutputText}))
	cmdAdd.SetOut(&bytes.Buffer{})
	cmdAdd.SetArgs([]string{"add", "drained-pool", "--cluster=demo", "--flavor=m1.large", "--count=0"})
	if err := cmdAdd.Execute(); err != nil {
		t.Fatalf("pool add failed: %v", err)
	}

	// Remove should succeed
	cmdRm := newClusterPoolCmd()
	cmdRm.SetContext(context.WithValue(context.Background(), globalOptionsContextKey{}, GlobalOptions{DryRun: false, Output: OutputText}))
	out := &bytes.Buffer{}
	cmdRm.SetOut(out)
	cmdRm.SetArgs([]string{"remove", "drained-pool", "--cluster=demo"})
	if err := cmdRm.Execute(); err != nil {
		t.Fatalf("pool remove failed: %v", err)
	}
	if !strings.Contains(out.String(), "removed") {
		t.Fatalf("expected removal confirmation, got: %s", out.String())
	}
}

func TestClusterPoolRemove_ForceBypassesCheck(t *testing.T) {
	cfgDir := t.TempDir()
	writeTestConfig(t, cfgDir, "demo", "openstack", "")
	prepareCommandTestEnv(t, cfgDir)

	// Add with count > 0
	cmdAdd := newClusterPoolCmd()
	cmdAdd.SetContext(context.WithValue(context.Background(), globalOptionsContextKey{}, GlobalOptions{DryRun: false, Output: OutputText}))
	cmdAdd.SetOut(&bytes.Buffer{})
	cmdAdd.SetArgs([]string{"add", "force-pool", "--cluster=demo", "--flavor=m1.large", "--count=5"})
	if err := cmdAdd.Execute(); err != nil {
		t.Fatalf("pool add failed: %v", err)
	}

	// Force remove
	cmdRm := newClusterPoolCmd()
	cmdRm.SetContext(context.WithValue(context.Background(), globalOptionsContextKey{}, GlobalOptions{DryRun: false, Output: OutputText}))
	out := &bytes.Buffer{}
	cmdRm.SetOut(out)
	cmdRm.SetArgs([]string{"remove", "force-pool", "--cluster=demo", "--force"})
	if err := cmdRm.Execute(); err != nil {
		t.Fatalf("force remove failed: %v", err)
	}
	if !strings.Contains(out.String(), "removed") {
		t.Fatalf("expected removal confirmation, got: %s", out.String())
	}
}

func TestClusterPoolList(t *testing.T) {
	cfgDir := t.TempDir()
	writeTestConfig(t, cfgDir, "demo", "openstack", "")
	prepareCommandTestEnv(t, cfgDir)

	// Add a pool
	cmdAdd := newClusterPoolCmd()
	cmdAdd.SetContext(context.WithValue(context.Background(), globalOptionsContextKey{}, GlobalOptions{DryRun: false, Output: OutputText}))
	cmdAdd.SetOut(&bytes.Buffer{})
	cmdAdd.SetArgs([]string{"add", "extra-pool", "--cluster=demo", "--flavor=m1.xlarge", "--count=4"})
	if err := cmdAdd.Execute(); err != nil {
		t.Fatalf("pool add failed: %v", err)
	}

	// List
	cmdList := newClusterPoolCmd()
	cmdList.SetContext(context.WithValue(context.Background(), globalOptionsContextKey{}, GlobalOptions{DryRun: false, Output: OutputText}))
	out := &bytes.Buffer{}
	cmdList.SetOut(out)
	cmdList.SetArgs([]string{"list", "--cluster=demo"})
	if err := cmdList.Execute(); err != nil {
		t.Fatalf("pool list failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "default") {
		t.Fatal("expected default pool in list")
	}
	if !strings.Contains(output, "extra-pool") {
		t.Fatal("expected extra-pool in list")
	}
	if !strings.Contains(output, "m1.xlarge") {
		t.Fatal("expected flavor in list output")
	}
}

func TestClusterPoolScale(t *testing.T) {
	cfgDir := t.TempDir()
	writeTestConfig(t, cfgDir, "demo", "openstack", "")
	prepareCommandTestEnv(t, cfgDir)

	// Add a pool
	cmdAdd := newClusterPoolCmd()
	cmdAdd.SetContext(context.WithValue(context.Background(), globalOptionsContextKey{}, GlobalOptions{DryRun: false, Output: OutputText}))
	cmdAdd.SetOut(&bytes.Buffer{})
	cmdAdd.SetArgs([]string{"add", "scale-pool", "--cluster=demo", "--flavor=m1.large", "--count=1"})
	if err := cmdAdd.Execute(); err != nil {
		t.Fatalf("pool add failed: %v", err)
	}

	// Scale
	cmdScale := newClusterPoolCmd()
	cmdScale.SetContext(context.WithValue(context.Background(), globalOptionsContextKey{}, GlobalOptions{DryRun: true, Output: OutputText}))
	out := &bytes.Buffer{}
	cmdScale.SetOut(out)
	cmdScale.SetArgs([]string{"scale", "scale-pool", "--cluster=demo", "--count=5"})
	if err := cmdScale.Execute(); err != nil {
		t.Fatalf("pool scale failed: %v", err)
	}
	if !strings.Contains(out.String(), "Would scale pool") {
		t.Fatalf("expected dry-run scale output, got: %s", out.String())
	}
}
func TestClusterPoolUpdate_ReplacesLabelsAndTaints(t *testing.T) {
	cfgDir := t.TempDir()
	writeTestConfig(t, cfgDir, "demo", "openstack", "")
	prepareCommandTestEnv(t, cfgDir)

	runPool(t, "add", "gpu-pool", "--cluster=demo", "--flavor=gpu.0.4.16", "--count=2",
		"--label=old=true", "--taint=old.io/x=keep:NoExecute")

	runPool(t, "update", "gpu-pool", "--cluster=demo",
		"--label=workload=gpu", "--label=gpu=true",
		"--taint=nvidia.com/gpu=present:NoSchedule")

	pool := mustPool(t, cfgDir, "demo", "gpu-pool")
	if want := map[string]string{"workload": "gpu", "gpu": "true"}; !reflect.DeepEqual(pool.Labels, want) {
		t.Fatalf("labels = %v, want %v (replacement must drop old keys)", pool.Labels, want)
	}
	wantTaints := []v2.TaintConfig{{Key: "nvidia.com/gpu", Value: "present", Effect: "NoSchedule"}}
	if !reflect.DeepEqual(pool.Taints, wantTaints) {
		t.Fatalf("taints = %v, want %v (replacement must drop old taints)", pool.Taints, wantTaints)
	}
}

func TestClusterPoolUpdate_OmittedFlagsPreserveLabelsAndTaints(t *testing.T) {
	cfgDir := t.TempDir()
	writeTestConfig(t, cfgDir, "demo", "openstack", "")
	prepareCommandTestEnv(t, cfgDir)

	runPool(t, "add", "keep-pool", "--cluster=demo", "--flavor=m1.large", "--count=1",
		"--label=workload=gpu", "--taint=nvidia.com/gpu=present:NoSchedule")

	runPool(t, "update", "keep-pool", "--cluster=demo", "--count=4")

	pool := mustPool(t, cfgDir, "demo", "keep-pool")
	if pool.Count != 4 {
		t.Fatalf("count = %d, want 4", pool.Count)
	}
	if want := map[string]string{"workload": "gpu"}; !reflect.DeepEqual(pool.Labels, want) {
		t.Fatalf("labels = %v, want %v (omitted --label must preserve)", pool.Labels, want)
	}
	if len(pool.Taints) != 1 || pool.Taints[0].Key != "nvidia.com/gpu" || pool.Taints[0].Effect != "NoSchedule" {
		t.Fatalf("taints = %v, want single nvidia.com/gpu:NoSchedule (omitted --taint must preserve)", pool.Taints)
	}
}

func TestClusterPoolUpdate_EmptyFlagsClearLabelsAndTaints(t *testing.T) {
	cfgDir := t.TempDir()
	writeTestConfig(t, cfgDir, "demo", "openstack", "")
	prepareCommandTestEnv(t, cfgDir)

	runPool(t, "add", "clear-pool", "--cluster=demo", "--flavor=m1.large", "--count=1",
		"--label=workload=gpu", "--taint=nvidia.com/gpu=present:NoSchedule")

	runPool(t, "update", "clear-pool", "--cluster=demo", "--label=", "--taint=")

	pool := mustPool(t, cfgDir, "demo", "clear-pool")
	if len(pool.Labels) != 0 {
		t.Fatalf("labels = %v, want cleared", pool.Labels)
	}
	if len(pool.Taints) != 0 {
		t.Fatalf("taints = %v, want cleared", pool.Taints)
	}
}

func TestClusterPoolUpdate_InvalidTaintDoesNotPersist(t *testing.T) {
	cfgDir := t.TempDir()
	writeTestConfig(t, cfgDir, "demo", "openstack", "")
	prepareCommandTestEnv(t, cfgDir)

	runPool(t, "add", "safe-pool", "--cluster=demo", "--flavor=m1.large", "--count=2",
		"--label=workload=gpu")

	cmd := newClusterPoolCmd()
	cmd.SetContext(context.WithValue(context.Background(), globalOptionsContextKey{}, GlobalOptions{DryRun: false, Output: OutputText}))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"update", "safe-pool", "--cluster=demo", "--count=9", "--taint=broken-no-effect"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for malformed taint")
	}

	pool := mustPool(t, cfgDir, "demo", "safe-pool")
	if pool.Count != 2 {
		t.Fatalf("count = %d, want 2 (failed update must not persist)", pool.Count)
	}
	if want := map[string]string{"workload": "gpu"}; !reflect.DeepEqual(pool.Labels, want) {
		t.Fatalf("labels = %v, want %v (failed update must not persist)", pool.Labels, want)
	}
}

func TestClusterPoolUpdate_WindowsPoolReplacesLabelsAndTaints(t *testing.T) {
	cfgDir := t.TempDir()
	writeTestConfig(t, cfgDir, "demo", "openstack", "")
	prepareCommandTestEnv(t, cfgDir)

	runPool(t, "add", "win-pool", "--cluster=demo", "--os=windows", "--flavor=w1.large", "--count=1",
		"--image=win-2022-img", "--label=old=true")

	runPool(t, "update", "win-pool", "--cluster=demo",
		"--label=os=windows", "--taint=windows2019=true:NoSchedule")

	cfg := mustLoadTestConfig(t, cfgDir, "demo")
	for _, p := range cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorkerWindows {
		if p.Name == "win-pool" {
			if want := map[string]string{"os": "windows"}; !reflect.DeepEqual(p.Labels, want) {
				t.Fatalf("windows pool labels = %v, want %v", p.Labels, want)
			}
			wantTaints := []v2.TaintConfig{{Key: "windows2019", Value: "true", Effect: "NoSchedule"}}
			if !reflect.DeepEqual(p.Taints, wantTaints) {
				t.Fatalf("windows pool taints = %v, want %v", p.Taints, wantTaints)
			}
			return
		}
	}
	t.Fatal("windows pool win-pool not found after update")
}

// runPool executes a pool subcommand with persistence enabled and fails on error.
func runPool(t *testing.T, args ...string) {
	t.Helper()
	cmd := newClusterPoolCmd()
	cmd.SetContext(context.WithValue(context.Background(), globalOptionsContextKey{}, GlobalOptions{DryRun: false, Output: OutputText}))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("pool %v failed: %v", args, err)
	}
}

func mustLoadTestConfig(t *testing.T, cfgDir, cluster string) v2.Config {
	t.Helper()
	configPath := filepath.Join(cfgDir, "clusters", "blueprints", "opencenter", cluster, cluster+"-config.yaml")
	cfg := loadV2ConfigForTest(t, configPath)
	return *cfg
}

func mustPool(t *testing.T, cfgDir, cluster, poolName string) v2.WorkerPoolConfig {
	t.Helper()
	cfg := mustLoadTestConfig(t, cfgDir, cluster)
	for _, p := range cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorker {
		if p.Name == poolName {
			return p
		}
	}
	t.Fatalf("pool %q not found in config", poolName)
	return v2.WorkerPoolConfig{}
}

// Suppress unused import warning for v2 — used by other test files in same package.
var _ = v2.Config{}
