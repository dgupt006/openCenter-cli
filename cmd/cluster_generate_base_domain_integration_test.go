// Copyright 2025 Victor Palma <victor.palma@rackspace.com>
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeSettingsBaseDomain writes cluster_defaults.base_domain into the CLI
// settings file inside the test config dir.
func writeSettingsBaseDomain(t *testing.T, baseDomain string) {
	t.Helper()
	dir := os.Getenv("OPENCENTER_CONFIG_DIR")
	if dir == "" {
		t.Fatal("OPENCENTER_CONFIG_DIR not set")
	}
	content := "cluster_defaults:\n  base_domain: " + baseDomain + "\n"
	if err := os.WriteFile(filepath.Join(dir, "settings.yaml"), []byte(content), 0o600); err != nil {
		t.Fatalf("write settings.yaml: %v", err)
	}
}

func readClusterConfigText(t *testing.T, clusterName string) string {
	t.Helper()
	dir := os.Getenv("OPENCENTER_CONFIG_DIR")
	data, err := os.ReadFile(filepath.Join(dir, "clusters", "blueprints", "opencenter", clusterName, clusterName+"-config.yaml"))
	if err != nil {
		t.Fatalf("read cluster config: %v", err)
	}
	return string(data)
}

// TestClusterGenerateBaseDomainSyncLifecycle pins the long-term base_domain
// contract end to end:
//  1. a placeholder cluster adopts the settings domain and follows it
//     (managed: default), generating with the settings-derived FQDN;
//  2. after a successful generate stamps the domain, changing settings is
//     reported without a silent stale render;
//  3. --sync-base-domain keeps base_domain and the derived cluster_fqdn
//     aligned with settings on disk;
//  4. a hand-pinned cluster with a different domain stays silent.
func TestClusterGenerateBaseDomainSyncLifecycle(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OPENCENTER_CONFIG_DIR", dir)

	binDir := t.TempDir()
	installFakeGitBinary(t, binDir)
	prependTestPath(t, binDir)

	initCmd := newClusterInitCmd()
	initCmd.SetOut(&bytes.Buffer{})
	initCmd.SetErr(&bytes.Buffer{})
	initCmd.SetArgs([]string{"domsync-int", "--type", "kind"})
	if err := initCmd.Execute(); err != nil {
		t.Fatalf("cluster init failed: %v", err)
	}
	resetCommandStateForTests()

	// Settings gain a real base domain after init baked the placeholder.
	writeSettingsBaseDomain(t, "k8s.example.com")

	// 1. Placeholder adoption: generate must render the settings domain.
	genCmd := newClusterGenerateCmd()
	var stdout, stderr bytes.Buffer
	genCmd.SetOut(&stdout)
	genCmd.SetErr(&stderr)
	genCmd.SetArgs([]string{"domsync-int", "--skip-validation"})
	if err := genCmd.Execute(); err != nil {
		t.Fatalf("generate failed: %v\nstderr: %s", err, stderr.String())
	}
	configText := readClusterConfigText(t, "domsync-int")
	if !strings.Contains(configText, "base_domain: k8s.example.com") {
		t.Fatalf("config did not adopt settings domain:\n%s", configText)
	}
	if !strings.Contains(configText, "cluster_fqdn: domsync-int.dfw3.k8s.example.com") {
		t.Fatalf("cluster_fqdn was not re-derived:\n%s", configText)
	}
	if !strings.Contains(configText, "base_domain: default") {
		t.Fatalf("adopted field must be tagged managed: default:\n%s", configText)
	}

	// The success path stamps the rendered domain.
	if !strings.Contains(configText, "last_generated_base_domain: k8s.example.com") {
		t.Fatalf("generate did not stamp last_generated_base_domain:\n%s", configText)
	}

	// 2. Settings change after generation: warn, offer sync, do not rewrite.
	writeSettingsBaseDomain(t, "new.example.org")
	stdout.Reset()
	stderr.Reset()
	genCmd = newClusterGenerateCmd()
	genCmd.SetOut(&stdout)
	genCmd.SetErr(&stderr)
	genCmd.SetArgs([]string{"domsync-int", "--skip-validation"})
	if err := genCmd.Execute(); err != nil {
		t.Fatalf("second generate failed: %v\nstderr: %s", err, stderr.String())
	}
	// Managed adoption happened at load; the follow-settings notice names the
	// change, and the on-disk config is rewritten with the propagated value.
	notice := stdout.String() + stderr.String()
	if !strings.Contains(notice, "new.example.org") {
		t.Fatalf("generate must report the settings change:\n%s", notice)
	}

	// 3. --sync-base-domain rewrites the config (and stays quiet when already
	// aligned after the managed propagation rewrote it).
	stdout.Reset()
	stderr.Reset()
	genCmd = newClusterGenerateCmd()
	genCmd.SetOut(&stdout)
	genCmd.SetErr(&stderr)
	genCmd.SetArgs([]string{"domsync-int", "--skip-validation", "--sync-base-domain"})
	if err := genCmd.Execute(); err != nil {
		t.Fatalf("sync generate failed: %v\nstderr: %s", err, stderr.String())
	}
	configText = readClusterConfigText(t, "domsync-int")
	if !strings.Contains(configText, "base_domain: new.example.org") {
		t.Fatalf("sync did not persist the settings domain:\n%s", configText)
	}
	if !strings.Contains(configText, "cluster_fqdn: domsync-int.dfw3.new.example.org") {
		t.Fatalf("sync did not re-derive cluster_fqdn:\n%s", configText)
	}
	if strings.Contains(stdout.String(), "Base domain synced") {
		t.Fatalf("aligned managed cluster must not report a rewrite:\n%s", stdout.String())
	}

	// 4. Hand-pinned different domain: silent.
	setCmd := newClusterSetCmd()
	setCmd.SetOut(&bytes.Buffer{})
	setCmd.SetErr(&bytes.Buffer{})
	setCmd.SetArgs([]string{"domsync-int", "opencenter.cluster.base_domain=pinned.example.net"})
	if err := setCmd.Execute(); err != nil {
		t.Fatalf("cluster set failed: %v", err)
	}
	resetCommandStateForTests()

	stdout.Reset()
	stderr.Reset()
	genCmd = newClusterGenerateCmd()
	genCmd.SetOut(&stdout)
	genCmd.SetErr(&stderr)
	genCmd.SetArgs([]string{"domsync-int", "--skip-validation"})
	if err := genCmd.Execute(); err != nil {
		t.Fatalf("pinned generate failed: %v\nstderr: %s", err, stderr.String())
	}
	notice = stdout.String() + stderr.String()
	if strings.Contains(notice, "base_domain changed") {
		t.Fatalf("pinned cluster must not be nagged about settings:\n%s", notice)
	}
	configText = readClusterConfigText(t, "domsync-int")
	if !strings.Contains(configText, "base_domain: pinned.example.net") {
		t.Fatalf("pinned domain must survive generation:\n%s", configText)
	}
}
