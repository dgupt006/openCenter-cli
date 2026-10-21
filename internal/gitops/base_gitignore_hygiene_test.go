package gitops

// TestBaseGitIgnoreCoversDeployArtifacts guards lifecycle contract v2 hygiene:
// the embedded base GitOps tree's root .gitignore must exclude the deploy-time
// OpenTofu/Kubespray artifacts (legacy-mode inventory subtree, module cache,
// lock file) and the local cluster-config inputs, so a freshly generated
// repository never shows them in `git status` and `cluster init`'s
// required-pattern check passes on shared repositories.

import (
	"strings"
	"testing"
)

func TestBaseGitIgnoreCoversDeployArtifacts(t *testing.T) {
	t.Parallel()

	data, err := Files.ReadFile("gitops-base-dir/.gitignore")
	if err != nil {
		t.Fatalf("read embedded gitops-base-dir/.gitignore: %v", err)
	}
	contains := func(pattern string) bool {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.TrimSpace(line) == pattern {
				return true
			}
		}
		return false
	}

	for _, pattern := range []string{
		"infrastructure/clusters/*/inventory/",
		"infrastructure/clusters/*/.terraform/",
		"infrastructure/clusters/*/.terraform.lock.hcl",
		"/*-config.yaml",
		"/.*-config.yaml",
	} {
		if !contains(pattern) {
			t.Errorf("base .gitignore missing pattern %q", pattern)
		}
	}
}
