package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	testhelpers "github.com/opencenter-cloud/opencenter-cli/internal/testing"
)

func setupTestClusterWithFiles(t *testing.T, dir, clusterName, organization string) (v2.Config, *clusterEnvTestFiles) {
	t.Helper()

	cfg, clusterPaths := saveKindConfigForCommandTest(t, dir, clusterName, organization)

	// Create test files
	if err := os.MkdirAll(filepath.Dir(clusterPaths.KubeconfigPath), 0o700); err != nil {
		t.Fatalf("mkdir kubeconfig parent: %v", err)
	}
	if err := os.WriteFile(clusterPaths.KubeconfigPath, []byte("apiVersion: v1\nclusters: []"), 0o600); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}

	if err := os.MkdirAll(clusterPaths.InventoryPath, 0o755); err != nil {
		t.Fatalf("mkdir inventory: %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(clusterPaths.SOPSKeyPath), 0o700); err != nil {
		t.Fatalf("mkdir sops key parent: %v", err)
	}
	if err := os.WriteFile(clusterPaths.SOPSKeyPath, []byte("AGE-SECRET-KEY-1TESTKEY"), 0o600); err != nil {
		t.Fatalf("write sops key: %v", err)
	}

	venvBin := filepath.Join(clusterPaths.VenvPath, "bin")
	if err := os.MkdirAll(venvBin, 0o755); err != nil {
		t.Fatalf("mkdir venv bin: %v", err)
	}
	activateBash := filepath.Join(venvBin, "activate")
	if err := os.WriteFile(activateBash, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write activate bash: %v", err)
	}
	activateFish := filepath.Join(venvBin, "activate.fish")
	if err := os.WriteFile(activateFish, []byte("#!/usr/bin/env fish\n"), 0o755); err != nil {
		t.Fatalf("write activate fish: %v", err)
	}

	venvScripts := filepath.Join(clusterPaths.VenvPath, "Scripts")
	if err := os.MkdirAll(venvScripts, 0o755); err != nil {
		t.Fatalf("mkdir venv Scripts: %v", err)
	}
	activatePs1 := filepath.Join(venvScripts, "Activate.ps1")
	if err := os.WriteFile(activatePs1, []byte("# PowerShell activate\n"), 0o755); err != nil {
		t.Fatalf("write activate ps1: %v", err)
	}

	if err := os.MkdirAll(clusterPaths.BinPath, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}

	return cfg, &clusterEnvTestFiles{
		KubeconfigPath: clusterPaths.KubeconfigPath,
		InventoryPath:  clusterPaths.InventoryPath,
		SOPSKeyPath:    clusterPaths.SOPSKeyPath,
		ActivateBash:   activateBash,
		ActivateFish:   activateFish,
		ActivatePs1:    activatePs1,
		BinPath:        clusterPaths.BinPath,
	}
}

type clusterEnvTestFiles struct {
	KubeconfigPath string
	InventoryPath  string
	SOPSKeyPath    string
	ActivateBash   string
	ActivateFish   string
	ActivatePs1    string
	BinPath        string
}

func TestClusterEnvBashExport(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	clusterName := "dev-cluster"
	org := "000002-project-argus"
	_, testFiles := setupTestClusterWithFiles(t, dir, clusterName, org)

	cmd := newClusterEnvCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{clusterName, "--shell", "bash"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v\nstderr: %s", err, stderr.String())
	}

	out := stdout.String()

	// Verify all exports
	expectedExports := []string{
		"export OPENCENTER_CLUSTER=" + clusterName,
		"export KUBECONFIG=" + testFiles.KubeconfigPath,
		"export ANSIBLE_INVENTORY=" + testFiles.InventoryPath,
		"export SOPS_AGE_KEY_FILE=" + testFiles.SOPSKeyPath,
		"source " + testFiles.ActivateBash,
		"export PATH=" + testFiles.BinPath + ":$PATH",
	}

	for _, expected := range expectedExports {
		if !strings.Contains(out, expected) {
			t.Errorf("expected output to contain %q, but got:\n%s", expected, out)
		}
	}

	// Verify kubeconfig is in secrets directory, not in gitops
	if strings.Contains(testFiles.KubeconfigPath, "/gitops/") {
		t.Errorf("kubeconfig path %q must not be in gitops directory", testFiles.KubeconfigPath)
	}
	if !strings.Contains(testFiles.KubeconfigPath, "/secrets/") {
		t.Errorf("kubeconfig path %q must be in secrets directory", testFiles.KubeconfigPath)
	}

	// Verify inventory, bin, and venv are in gitops infrastructure
	if !strings.Contains(testFiles.InventoryPath, "/infrastructure/clusters/") {
		t.Errorf("inventory path %q must be under infrastructure/clusters/", testFiles.InventoryPath)
	}
	if !strings.Contains(testFiles.BinPath, "/infrastructure/clusters/") {
		t.Errorf("bin path %q must be under infrastructure/clusters/", testFiles.BinPath)
	}
}

func TestClusterEnvFishExport(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	clusterName := "fish-cluster"
	org := "test-org"
	_, testFiles := setupTestClusterWithFiles(t, dir, clusterName, org)

	cmd := newClusterEnvCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{clusterName, "--shell", "fish"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v\nstderr: %s", err, stderr.String())
	}

	out := stdout.String()

	expectedFish := []string{
		"set -gx OPENCENTER_CLUSTER " + clusterName,
		"set -gx KUBECONFIG " + testFiles.KubeconfigPath,
		"set -gx ANSIBLE_INVENTORY " + testFiles.InventoryPath,
		"set -gx SOPS_AGE_KEY_FILE " + testFiles.SOPSKeyPath,
		"source " + testFiles.ActivateFish,
		"set -gx PATH " + testFiles.BinPath + " $PATH",
	}

	for _, expected := range expectedFish {
		if !strings.Contains(out, expected) {
			t.Errorf("expected fish output to contain %q, but got:\n%s", expected, out)
		}
	}
}

func TestClusterEnvPowerShellExport(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	clusterName := "ps-cluster"
	org := "test-org"
	_, testFiles := setupTestClusterWithFiles(t, dir, clusterName, org)

	cmd := newClusterEnvCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{clusterName, "--shell", "powershell"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v\nstderr: %s", err, stderr.String())
	}

	out := stdout.String()

	expectedPS := []string{
		"$env:OPENCENTER_CLUSTER = '" + clusterName + "'",
		"$env:KUBECONFIG = '" + testFiles.KubeconfigPath + "'",
		"$env:ANSIBLE_INVENTORY = '" + testFiles.InventoryPath + "'",
		"$env:SOPS_AGE_KEY_FILE = '" + testFiles.SOPSKeyPath + "'",
		". " + testFiles.ActivatePs1,
		"$env:PATH = '" + testFiles.BinPath + ";' + $env:PATH",
	}

	for _, expected := range expectedPS {
		if !strings.Contains(out, expected) {
			t.Errorf("expected powershell output to contain %q, but got:\n%s", expected, out)
		}
	}
}

func TestClusterEnvActiveClusterFallback(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	clusterName := "active-cluster"
	org := "my-org"
	setupTestClusterWithFiles(t, dir, clusterName, org)

	if err := setActiveCluster(clusterName); err != nil {
		t.Fatalf("set active cluster: %v", err)
	}

	cmd := newClusterEnvCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{}) // No args -> should resolve active cluster

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v\nstderr: %s", err, stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "export OPENCENTER_CLUSTER="+clusterName) {
		t.Fatalf("expected export of active cluster %s, got:\n%s", clusterName, out)
	}
}

func TestClusterEnvOpenStackCredentials(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	clusterName := "os-cluster"
	org := "my-org"

	resolver, _ := createClusterDirectoriesForTest(t, dir, clusterName, org)
	cfgPtr, err := v2.NewV2Default(clusterName, "openstack")
	if err != nil {
		t.Fatalf("create openstack config: %v", err)
	}
	cfg := *cfgPtr
	cfg.OpenCenter.Meta.Name = clusterName
	cfg.OpenCenter.Meta.Organization = org
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL = "https://identity.example.com:5000/v3"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialID = "app-cred-id-123"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialSecret = "secret123"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ProjectName = "demo"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.Region = "RegionOne"

	testhelpers.SaveConfigWithPathResolver(t, cfg, resolver)

	cmd := newClusterEnvCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{clusterName, "--shell", "bash"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v\nstderr: %s", err, stderr.String())
	}

	out := stdout.String()
	expectedCreds := []string{
		`export OS_AUTH_URL="https://identity.example.com:5000/v3"`,
		`export OS_APPLICATION_CREDENTIAL_ID="app-cred-id-123"`,
		`export OS_APPLICATION_CREDENTIAL_SECRET="secret123"`,
		`export OS_PROJECT_NAME="demo"`,
		`export OS_REGION_NAME="RegionOne"`,
		"export OPENCENTER_CLUSTER=" + clusterName,
	}

	for _, expected := range expectedCreds {
		if !strings.Contains(out, expected) {
			t.Errorf("expected openstack output to contain %q, but got:\n%s", expected, out)
		}
	}
}

func TestClusterEnvMissingActiveCluster(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	cmd := newClusterEnvCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{}) // No args and no active cluster

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when no active cluster is set")
	}
	if !strings.Contains(err.Error(), "no active cluster") {
		t.Fatalf("expected 'no active cluster' error, got: %v", err)
	}
}

func TestClusterEnvInvalidShell(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	clusterName := "sh-cluster"
	org := "my-org"
	setupTestClusterWithFiles(t, dir, clusterName, org)

	cmd := newClusterEnvCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{clusterName, "--shell", "invalid-shell"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid shell")
	}
	if !strings.Contains(err.Error(), "invalid shell") {
		t.Fatalf("expected 'invalid shell' error, got: %v", err)
	}
}
