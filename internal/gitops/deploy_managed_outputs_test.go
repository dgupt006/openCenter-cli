// Copyright 2025 Victor Palma <victor.palma@rackspace.com>
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gitops

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Deploy-time Kubespray/OpenTofu outputs land in the cluster infrastructure
// directory, which is also the cluster state zone. A later `cluster generate`
// must treat them as deploy-managed (like the inventory subtree and terraform
// state, OCTR-786) instead of refusing promotion with "user-authored files
// found in generator-owned paths".

func TestIsDeployManagedPathCoversLifecycleOutputs(t *testing.T) {
	t.Parallel()
	managed := []string{
		"infrastructure/clusters/kit/kubespray-outputs.json",
		"infrastructure/clusters/kit/.kubeconfig.fetch.tmp",
		"infrastructure/clusters/kit/ansible-local-tmp/ansible_tmpX",
		"infrastructure/clusters/kit/inventory/group_vars/all/all.yaml",
		"infrastructure/clusters/kit/.terraform/modules/modules.json",
		"infrastructure/clusters/kit/.terraform.lock.hcl",
		"infrastructure/clusters/kit/id_rsa",
		"infrastructure/clusters/kit/id_rsa.pub",
	}
	for _, path := range managed {
		if !isDeployManagedPath("kit", path) {
			t.Errorf("deploy-managed path %q not exempted", path)
		}
	}
	policed := []string{
		"infrastructure/clusters/kit/main.tf",
		"infrastructure/clusters/kit/Makefile",
		"infrastructure/clusters/kit/inventory.gitkeep",
		"infrastructure/clusters/kit/kubespray-outputs.json.bak",
		"infrastructure/clusters/kit/ansible-local-tmp.txt",
		"infrastructure/clusters/other/kubespray-outputs.json",
		"applications/overlays/kit/kubespray-outputs.json",
	}
	for _, path := range policed {
		if isDeployManagedPath("kit", path) {
			t.Errorf("generator-owned path %q wrongly exempted", path)
		}
	}
}

// TestGenerateClusterTreeAllowsDeployOutputsOnRegenerate reproduces the
// reported failure: after a deploy writes kubespray-outputs.json (plus the
// ansible scratch dir and the kubeconfig fetch temp file) into the cluster
// dir, a second generate must still promote.
func TestGenerateClusterTreeAllowsDeployOutputsOnRegenerate(t *testing.T) {
	repo, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("evalsymlinks: %v", err)
	}
	cfg := newDefault("regen-after-deploy")
	cfg.OpenCenter.GitOps.Repository.LocalDir = repo
	options := StagedGenerationOptions{IncludeInfrastructure: true, IncludeFluxBridge: true}

	if _, _, err := GenerateClusterTree(context.Background(), cfg, options); err != nil {
		t.Fatalf("initial GenerateClusterTree() error = %v", err)
	}

	clusterDir := filepath.Join(repo, "infrastructure", "clusters", cfg.ClusterName())
	for rel, content := range map[string]string{
		"kubespray-outputs.json":              `{"kubeconfig_path":"/tmp/kc"}` + "\n",
		".kubeconfig.fetch.tmp":               "stale fetch\n",
		"ansible-local-tmp/ansible_facts.tmp": "{}\n",
	} {
		full := filepath.Join(clusterDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	if _, _, err := GenerateClusterTree(context.Background(), cfg, options); err != nil {
		t.Fatalf("regenerate after deploy error = %v (deploy output misclassified as user-authored)", err)
	}
	for rel := range map[string]string{
		"kubespray-outputs.json":              "",
		".kubeconfig.fetch.tmp":               "",
		"ansible-local-tmp/ansible_facts.tmp": "",
	} {
		if _, statErr := os.Stat(filepath.Join(clusterDir, filepath.FromSlash(rel))); statErr != nil {
			t.Errorf("deploy artifact %s must survive regenerate: %v", rel, statErr)
		}
	}
}

// TestGenerateClusterTreeHealsInitAppendedGitIgnore covers the shared-repo
// hygiene drift: the tracked root .gitignore was extended in place by
// `cluster init`'s machine-managed deploy-artifact append after the ledger
// recorded the pre-append file. The on-disk drift is openCenter's own edit, so
// promotion must heal by adopting the staged file instead of refusing the run
// forever.
func TestGenerateClusterTreeHealsInitAppendedGitIgnore(t *testing.T) {
	repo, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("evalsymlinks: %v", err)
	}
	cfg := newDefault("heal-gitignore")
	cfg.OpenCenter.GitOps.Repository.LocalDir = repo
	options := StagedGenerationOptions{IncludeInfrastructure: true, IncludeFluxBridge: true}

	if _, _, err := GenerateClusterTree(context.Background(), cfg, options); err != nil {
		t.Fatalf("initial GenerateClusterTree() error = %v", err)
	}

	ignorePath := filepath.Join(repo, ".gitignore")
	tracked, err := os.ReadFile(ignorePath)
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	// Simulate the shared-repo drift: the ledger records the file as promoted,
	// and `cluster init` later appended the managed deploy-artifact exclusions
	// in place (the same lines the staged base copy carries).
	appended := string(tracked)
	if !strings.HasSuffix(appended, "\n") {
		appended += "\n"
	}
	appended += "\n# Deploy-time OpenTofu/Kubespray artifacts (managed by openCenter).\n"
	for _, pattern := range DeployArtifactIgnorePatterns {
		appended += pattern + "\n"
	}
	if err := os.WriteFile(ignorePath, []byte(appended), 0o644); err != nil {
		t.Fatalf("write appended .gitignore: %v", err)
	}

	if _, _, err := GenerateClusterTree(context.Background(), cfg, options); err != nil {
		t.Fatalf("regenerate after init append error = %v (hygiene drift refused instead of healed)", err)
	}
	healed, err := os.ReadFile(ignorePath)
	if err != nil {
		t.Fatalf("read healed .gitignore: %v", err)
	}
	for _, pattern := range DeployArtifactIgnorePatterns {
		if !strings.Contains(string(healed), pattern+"\n") {
			t.Errorf("healed .gitignore missing pattern %q:\n%s", pattern, healed)
		}
	}

	// A third generate must be a clean no-op: the ledger now matches disk.
	if _, _, err := GenerateClusterTree(context.Background(), cfg, options); err != nil {
		t.Fatalf("regenerate after heal error = %v", err)
	}
}

// TestGenerateClusterTreeStillRefusesForeignGitIgnore guards the heal's
// boundary: a tracked .gitignore edited by the user with content the managed
// deploy-artifact set cannot explain is still a conflict.
func TestGenerateClusterTreeStillRefusesForeignGitIgnore(t *testing.T) {
	repo, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("evalsymlinks: %v", err)
	}
	cfg := newDefault("foreign-gitignore")
	cfg.OpenCenter.GitOps.Repository.LocalDir = repo
	options := StagedGenerationOptions{IncludeInfrastructure: true, IncludeFluxBridge: true}

	if _, _, err := GenerateClusterTree(context.Background(), cfg, options); err != nil {
		t.Fatalf("initial GenerateClusterTree() error = %v", err)
	}
	ignorePath := filepath.Join(repo, ".gitignore")
	current, err := os.ReadFile(ignorePath)
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	if err := os.WriteFile(ignorePath, []byte(string(current)+"\n!infrastructure/clusters/kit/kubespray-outputs.json\n"), 0o644); err != nil {
		t.Fatalf("write user .gitignore: %v", err)
	}
	if _, _, err := GenerateClusterTree(context.Background(), cfg, options); err == nil {
		t.Fatal("regenerate with user-edited tracked .gitignore must refuse promotion")
	}
}
