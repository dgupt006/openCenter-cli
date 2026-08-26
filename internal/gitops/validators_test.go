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

package gitops

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManifestValidator_hasProperIndentation(t *testing.T) {
	v := &ManifestValidator{}

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{
			name: "proper 2-space indentation",
			content: `apiVersion: v1
kind: Service
metadata:
  name: test
  namespace: default
spec:
  ports:
    - port: 80
      targetPort: 8080`,
			want: true,
		},
		{
			name: "improper indentation with tabs",
			content: `apiVersion: v1
kind: Service
metadata:
	name: test`,
			want: false,
		},
		{
			name: "improper indentation with odd spaces",
			content: `apiVersion: v1
kind: Service
metadata:
 name: test`,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := v.hasProperIndentation(tt.content); got != tt.want {
				t.Errorf("hasProperIndentation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestManifestValidator_hasProperdependsOnIndentation(t *testing.T) {
	v := &ManifestValidator{}

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{
			name: "dependency with namespace",
			content: `spec:
  dependsOn:
    - name: sources
      namespace: flux-system`,
			want: true,
		},
		{
			name: "name-only dependency",
			content: `spec:
  dependsOn:
    - name: sources`,
			want: true,
		},
		{
			name: "mixed dependencies with and without namespace",
			content: `spec:
  dependsOn:
    - name: sources
    - name: platform
      namespace: flux-system`,
			want: true,
		},
		{
			name: "dependsOn is not a sequence",
			content: `spec:
  dependsOn: sources`,
			want: false,
		},
		{
			name: "dependency is missing name",
			content: `spec:
  dependsOn:
    - namespace: flux-system`,
			want: false,
		},
		{
			name: "dependency name is not a string",
			content: `spec:
  dependsOn:
    - name: 42`,
			want: false,
		},
		{
			name: "dependency name is empty",
			content: `spec:
  dependsOn:
    - name: ""`,
			want: false,
		},
		{
			name: "dependency namespace is empty",
			content: `spec:
  dependsOn:
    - name: sources
      namespace: ""`,
			want: false,
		},
		{
			name: "dependency namespace is not a string",
			content: `spec:
  dependsOn:
    - name: sources
      namespace: 42`,
			want: false,
		},
		{
			name: "later document cannot be masked by valid first document",
			content: `---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
spec:
  dependsOn:
    - name: sources
---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
spec:
  dependsOn:
    - name: sources
    - namespace: flux-system`,
			want: false,
		},
		{
			name: "improper dependsOn indentation",
			content: `spec:
  dependsOn:
  - name: sources
  namespace: flux-system`,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := v.hasProperdependsOnIndentation(tt.content); got != tt.want {
				t.Errorf("hasProperdependsOnIndentation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestManifestValidator_hasProperDecryptionIndentation(t *testing.T) {
	v := &ManifestValidator{}

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{
			name: "proper decryption indentation",
			content: `spec:
  decryption:
    provider: sops
    secretRef:
      name: sops-age`,
			want: true,
		},
		{
			name: "improper decryption indentation",
			content: `spec:
  decryption:
  provider: sops
  secretRef:
  name: sops-age`,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := v.hasProperDecryptionIndentation(tt.content); got != tt.want {
				t.Errorf("hasProperDecryptionIndentation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestManifestValidator_isBase64(t *testing.T) {
	v := &ManifestValidator{}

	tests := []struct {
		name string
		s    string
		want bool
	}{
		{
			name: "valid base64",
			s:    "YWJjZGVmZ2g=",
			want: true,
		},
		{
			name: "plaintext with spaces",
			s:    "my secret value",
			want: false,
		},
		{
			name: "plaintext without spaces",
			s:    "mysecretvalue",
			want: true, // Cannot distinguish from base64 without decoding
		},
		{
			name: "empty string",
			s:    "",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := v.isBase64(tt.s); got != tt.want {
				t.Errorf("isBase64() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestManifestValidator_isValidIPRange(t *testing.T) {
	v := &ManifestValidator{}

	tests := []struct {
		name string
		s    string
		want bool
	}{
		{
			name: "valid IP range",
			s:    "172.23.0.6-172.23.0.8",
			want: true,
		},
		{
			name: "valid CIDR",
			s:    "10.0.0.0/24",
			want: true,
		},
		{
			name: "valid single IP",
			s:    "192.168.1.1",
			want: true,
		},
		{
			name: "invalid IP range",
			s:    "172.23.0.6-172.23.0",
			want: false,
		},
		{
			name: "invalid format",
			s:    "not-an-ip",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := v.isValidIPRange(tt.s); got != tt.want {
				t.Errorf("isValidIPRange() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestManifestValidator_validateFluxCDKustomization(t *testing.T) {
	// Create temporary directory for test files
	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		content     string
		wantErrors  int
		errorSubstr string
	}{
		{
			name: "valid kustomization",
			content: `apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: test
  namespace: flux-system
spec:
  interval: 5m
  path: ./applications/overlays/k8s-qa/services/test
  sourceRef:
    kind: GitRepository
    name: test`,
			wantErrors: 0,
		},
		{
			name: "valid kustomization with 15m interval",
			content: `apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: test
  namespace: flux-system
spec:
  interval: 15m
  path: ./applications/overlays/k8s-qa/services/test
  sourceRef:
    kind: GitRepository
    name: test`,
			wantErrors: 0,
		},
		{
			name: "wrong interval",
			content: `apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: test
  namespace: flux-system
spec:
  interval: 30m
  path: ./applications/overlays/k8s-qa/services/test`,
			wantErrors:  1,
			errorSubstr: "interval should be 5m or 15m",
		},
		{
			name: "hardcoded cluster name",
			content: `apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: test
  namespace: flux-system
spec:
  interval: 5m
  path: ./applications/overlays/dev-cluster/services/test`,
			wantErrors:  1,
			errorSubstr: "hardcoded cluster name",
		},
		{
			name: "improper indentation",
			content: `apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
 name: test
 namespace: flux-system`,
			wantErrors:  1,
			errorSubstr: "improper indentation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test file
			testFile := filepath.Join(tmpDir, "test.yaml")
			if err := os.WriteFile(testFile, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to write test file: %v", err)
			}

			// Create validator
			v := NewManifestValidator(tmpDir)

			// Validate the file
			v.validateFluxCDKustomization(testFile)

			// Check error count
			if len(v.errors) != tt.wantErrors {
				t.Errorf("validateFluxCDKustomization() errors = %d, want %d", len(v.errors), tt.wantErrors)
				if len(v.errors) > 0 {
					t.Logf("Errors: %v", v.errors)
				}
			}

			// Check error message contains expected substring
			if tt.wantErrors > 0 && tt.errorSubstr != "" {
				found := false
				for _, err := range v.errors {
					if contains(err, tt.errorSubstr) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error containing %q, got errors: %v", tt.errorSubstr, v.errors)
				}
			}
		})
	}
}

func TestManifestValidator_validateGitRepository(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		content     string
		wantErrors  int
		errorSubstr string
	}{
		{
			name: "valid gitrepository",
			content: `apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: test
  namespace: flux-system
spec:
  interval: 15m
  url: ssh://git@github.com/opencenter-cloud/openCenter-gitops-base.git
  ref:
    branch: main`,
			wantErrors: 0,
		},
		{
			name: "wrong capitalization",
			content: `apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: test
  namespace: flux-system
spec:
  interval: 15m
  url: ssh://git@github.com/opencenter-cloud/opencenter-gitops-base.git
  ref:
    branch: main`,
			wantErrors:  1,
			errorSubstr: "openCenter-gitops-base",
		},
		{
			name: "wrong interval",
			content: `apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: test
  namespace: flux-system
spec:
  interval: 5m
  url: ssh://git@github.com/opencenter-cloud/openCenter-gitops-base.git
  ref:
    branch: main`,
			wantErrors:  1,
			errorSubstr: "interval should be 15m",
		},
		{
			name: "tag instead of branch",
			content: `apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: test
  namespace: flux-system
spec:
  interval: 15m
  url: ssh://git@github.com/opencenter-cloud/openCenter-gitops-base.git
  ref:
    tag: v0.1.0`,
			wantErrors: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testFile := filepath.Join(tmpDir, "test.yaml")
			if err := os.WriteFile(testFile, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to write test file: %v", err)
			}

			v := NewManifestValidator(tmpDir)
			v.validateGitRepository(testFile)

			if len(v.errors) != tt.wantErrors {
				t.Errorf("validateGitRepository() errors = %d, want %d", len(v.errors), tt.wantErrors)
				if len(v.errors) > 0 {
					t.Logf("Errors: %v", v.errors)
				}
			}

			if tt.wantErrors > 0 && tt.errorSubstr != "" {
				found := false
				for _, err := range v.errors {
					if contains(err, tt.errorSubstr) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error containing %q, got errors: %v", tt.errorSubstr, v.errors)
				}
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
