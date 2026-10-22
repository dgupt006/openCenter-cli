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
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	testhelpers "github.com/opencenter-cloud/opencenter-cli/internal/testing"
)

// setupKubeconfigPathTest isolates the config zones in a temp dir, creates the
// cluster directories, and persists a kind config so the settings-based path
// resolver can resolve the cluster.
func setupKubeconfigPathTest(t *testing.T, clusterName, organization string) (v2.Config, string) {
	t.Helper()

	dir := t.TempDir()
	t.Setenv("OPENCENTER_CONFIG_DIR", dir)

	resolver, clusterPaths := createClusterDirectoriesForTest(t, dir, clusterName, organization)

	cfgPtr, err := v2.NewV2Default(clusterName, "kind")
	if err != nil {
		t.Fatalf("NewV2Default() error = %v", err)
	}
	cfg := *cfgPtr
	cfg.OpenCenter.Meta.Name = clusterName
	cfg.OpenCenter.Meta.Organization = organization
	cfg.OpenCenter.GitOps.Repository.LocalDir = filepath.Join(dir, "gitops", organization)

	testhelpers.SaveConfigWithPathResolver(t, cfg, resolver)
	return cfg, clusterPaths.KubeconfigPath
}

func TestGetKubeconfigPathUsesSecretsZone(t *testing.T) {
	cfg, wantKubeconfig := setupKubeconfigPathTest(t, "kubeconfig-path-cluster", "acme")

	got := getKubeconfigPath(context.Background(), &cfg)
	if got != wantKubeconfig {
		t.Fatalf("getKubeconfigPath() = %s, want secrets-zone path %s", got, wantKubeconfig)
	}
}

func TestGetKubeconfigPathIgnoresLegacyGitOpsKubeconfig(t *testing.T) {
	cfg, wantKubeconfig := setupKubeconfigPathTest(t, "kubeconfig-legacy-cluster", "acme")

	// Plant a kubeconfig at the pre-state-zone GitOps location. Resolution must
	// never pick it up.
	legacyDir := filepath.Join(cfg.OpenCenter.GitOps.Repository.LocalDir, "infrastructure", "clusters", cfg.ClusterName())
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatalf("mkdir legacy kubeconfig dir: %v", err)
	}
	legacyPath := filepath.Join(legacyDir, "kubeconfig.yaml")
	if err := os.WriteFile(legacyPath, []byte("apiVersion: v1\nkind: Config\n"), 0o600); err != nil {
		t.Fatalf("write legacy kubeconfig: %v", err)
	}

	for _, fn := range []struct {
		name    string
		collect func(*v2.Config) string
	}{
		{"getKubeconfigPath", func(c *v2.Config) string { return getKubeconfigPath(context.Background(), c) }},
		{"getKubeconfigPathForSync", func(c *v2.Config) string { return getKubeconfigPathForSync(context.Background(), c) }},
	} {
		got := fn.collect(&cfg)
		if got == legacyPath {
			t.Fatalf("%s() resolved the legacy GitOps kubeconfig %s", fn.name, got)
		}
		if got != wantKubeconfig {
			t.Fatalf("%s() = %s, want state-zone path %s", fn.name, got, wantKubeconfig)
		}
	}
}
