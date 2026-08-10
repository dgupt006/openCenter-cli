package gitops

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderSourceAuthBlock_TokenActive(t *testing.T) {
	params := SourceAuthParams{
		AuthMethod: gitopsAuthMethodToken,
		TokenURL:   "https://github.com/opencenter-cloud/openCenter-gitops-base.git",
		SSHURL:     "ssh://git@github.com/opencenter-cloud/openCenter-gitops-base.git",
		RefType:    "tag",
		RefValue:   "2026.01",
		SecretName: "opencenter-base",
	}

	result := RenderSourceAuthBlock(params)

	assert.Contains(t, result, "# --- token auth (active) ---")
	assert.Contains(t, result, "url: https://github.com/opencenter-cloud/openCenter-gitops-base.git")
	assert.Contains(t, result, `tag: "2026.01"`)
	assert.Contains(t, result, "  secretRef:\n    name: opencenter-base")
	assert.Contains(t, result, "# --- ssh auth (alternative) ---")
	assert.Contains(t, result, "# url: ssh://git@github.com/opencenter-cloud/openCenter-gitops-base.git")
	assert.Contains(t, result, `#   tag: "2026.01"`)
	assert.Contains(t, result, "# secretRef:")
	assert.Contains(t, result, "#   name: opencenter-base")

	for _, line := range strings.Split(result, "\n") {
		if strings.Contains(line, "url: https://") {
			assert.False(t, strings.HasPrefix(strings.TrimSpace(line), "#"), "active URL should not be commented: %s", line)
		}
	}
}

func TestRenderSourceAuthBlock_SSHActiveWithCustomerSecret(t *testing.T) {
	params := SourceAuthParams{
		AuthMethod: gitopsAuthMethodSSH,
		TokenURL:   "https://github.com/customer/cluster-config.git",
		SSHURL:     "ssh://git@github.com/customer/cluster-config.git",
		RefType:    "branch",
		RefValue:   "main",
		SecretName: "flux-system",
	}

	result := RenderSourceAuthBlock(params)

	assert.Contains(t, result, "# --- ssh auth (active) ---")
	assert.Contains(t, result, "url: ssh://git@github.com/customer/cluster-config.git")
	assert.Contains(t, result, `branch: "main"`)
	assert.Contains(t, result, "  secretRef:\n    name: flux-system")
	assert.Contains(t, result, "# --- token auth (alternative) ---")
	assert.Contains(t, result, "# url: https://github.com/customer/cluster-config.git")
	assert.Contains(t, result, `#   name: flux-system`)
}

func TestGitRepositoryURLVariants(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantToken string
		wantSSH   string
	}{
		{
			name:      "https",
			input:     "https://github.com/example/service.git",
			wantToken: "https://github.com/example/service.git",
			wantSSH:   "ssh://git@github.com/example/service.git",
		},
		{
			name:      "ssh URL",
			input:     "ssh://git@gitlab.example.com:2222/group/service.git",
			wantToken: "https://gitlab.example.com:2222/group/service.git",
			wantSSH:   "ssh://git@gitlab.example.com:2222/group/service.git",
		},
		{
			name:      "scp style",
			input:     "git@github.com:example/service.git",
			wantToken: "https://github.com/example/service.git",
			wantSSH:   "ssh://git@github.com/example/service.git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenURL, sshURL, err := GitRepositoryURLVariants(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.wantToken, tokenURL)
			assert.Equal(t, tt.wantSSH, sshURL)
		})
	}
}

func TestBuildSourceAuthParamsUsesExplicitMethod(t *testing.T) {
	params, err := BuildSourceAuthParams(gitopsAuthMethodSSH, "https://github.com/example/service.git", "branch", "develop", "custom-secret")
	require.NoError(t, err)
	assert.Equal(t, gitopsAuthMethodSSH, params.AuthMethod)
	assert.Equal(t, "https://github.com/example/service.git", params.TokenURL)
	assert.Equal(t, "ssh://git@github.com/example/service.git", params.SSHURL)
	assert.Equal(t, "custom-secret", params.SecretName)
}

// TestRenderSourceAuthBlock_NumericTagIsQuoted is a regression test ensuring
// that numeric-looking tag values (e.g. "2026.01") are rendered with YAML
// quotes so they are parsed as strings, not floats. Without quoting, Flux's
// GitRepository validation rejects the value:
//
//	.spec.ref.tag: expected string, got &value.valueUnstructured{Value:2026.01}
func TestRenderSourceAuthBlock_NumericTagIsQuoted(t *testing.T) {
	tests := []struct {
		name     string
		refValue string
	}{
		{name: "release looks like float", refValue: "2026.01"},
		{name: "release is pure integer", refValue: "2026"},
		{name: "release is semver-like", refValue: "1.0"},
		{name: "release with patch version", refValue: "1.2.3"},
		{name: "release is normal string", refValue: "main"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := SourceAuthParams{
				AuthMethod: gitopsAuthMethodToken,
				TokenURL:   "https://github.com/example/repo.git",
				SSHURL:     "ssh://git@github.com/example/repo.git",
				RefType:    "tag",
				RefValue:   tt.refValue,
				SecretName: "opencenter-base",
			}

			result := RenderSourceAuthBlock(params)

			// The active ref line must always be quoted.
			expectedActive := fmt.Sprintf(`    tag: "%s"`, tt.refValue)
			assert.Contains(t, result, expectedActive,
				"active tag value must be quoted to prevent YAML float interpretation")

			// The commented-out alternative ref line must also be quoted.
			expectedAlt := fmt.Sprintf(`  #   tag: "%s"`, tt.refValue)
			assert.Contains(t, result, expectedAlt,
				"commented-out tag value must be quoted to prevent YAML float interpretation")
		})
	}
}
