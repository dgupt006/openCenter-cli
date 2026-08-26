package gitops

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSecretScannerDetectsPrivateKeysTokensAndPlaintextSecrets(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, root, "applications/overlays/demo/services/app/plain-secret.yaml", `apiVersion: v1
kind: ConfigMap
metadata:
  name: safe
---
apiVersion: v1
kind: Secret
metadata:
  name: unsafe
stringData:
  password: plaintext
`)
	writeFile(t, root, "notes/key.txt", "AGE-SECRET-KEY-1EXAMPLE")
	writeFile(t, root, "notes/token.txt", "remote=https://ghp_1234567890abcdefghijklmnopqrstuvwx@example.invalid/repo.git")

	findings, err := ScanGitOpsSecrets(root)
	if err != nil {
		t.Fatalf("ScanGitOpsSecrets() error = %v", err)
	}

	assertFinding(t, findings, "age-private-key")
	assertFinding(t, findings, "git-token")
	assertFinding(t, findings, "unencrypted-kubernetes-secret")
	assertFinding(t, findings, "plaintext-secret-field")
}

func TestSecretScannerAcceptsSOPSEncryptedSecret(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, root, "applications/overlays/demo/services/app/encrypted-secret.yaml", `apiVersion: v1
kind: Secret
metadata:
  name: encrypted
data:
  password: ENC[AES256_GCM,data:abc,iv:def,tag:ghi,type:str]
sops:
  mac: ENC[AES256_GCM,data:abc,iv:def,tag:ghi,type:str]
  age:
    - recipient: age1example
      enc: |
        -----BEGIN AGE ENCRYPTED FILE-----
        example
        -----END AGE ENCRYPTED FILE-----
`)

	findings, err := ScanGitOpsSecrets(root)
	if err != nil {
		t.Fatalf("ScanGitOpsSecrets() error = %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("ScanGitOpsSecrets() findings = %+v, want none", findings)
	}
}

func TestSecretScannerRejectsInvalidSOPSMetadata(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, root, "applications/overlays/demo/services/app/invalid-sops-secret.yaml", `apiVersion: v1
kind: Secret
metadata:
  name: invalid
data:
  password: ENC[AES256_GCM,data:abc,iv:def,tag:ghi,type:str]
sops:
  version: fake
`)

	findings, err := ScanGitOpsSecrets(root)
	if err != nil {
		t.Fatalf("ScanGitOpsSecrets() error = %v", err)
	}
	assertFinding(t, findings, "invalid-sops-metadata")
}

func TestSecretScannerTraversesNestedSecretValues(t *testing.T) {
	t.Parallel()

	const validSOPSMetadata = `sops:
  mac: ENC[AES256_GCM,data:abc,iv:def,tag:ghi,type:str]
  age:
    - recipient: age1example
      enc: |
        -----BEGIN AGE ENCRYPTED FILE-----
        example
        -----END AGE ENCRYPTED FILE-----`
	const encrypted = `ENC[AES256_GCM,data:abc,iv:def,tag:ghi,type:str]`

	tests := []struct {
		name         string
		secretFields string
		wantMessages []string
	}{
		{
			name: "fully encrypted nested leaves are accepted",
			secretFields: `data:
  database:
    username: ` + encrypted + `
    password: ` + encrypted + `
stringData:
  tls:
    certificate: ` + encrypted,
		},
		{
			name: "mixed nested leaves report only plaintext leaves",
			secretFields: `data:
  database:
    username: ` + encrypted + `
    password: plaintext-password
stringData:
  nested:
    api:
      token: plaintext-token`,
			wantMessages: []string{
				"Secret data.database.password is not SOPS-encrypted",
				"Secret stringData.nested.api.token is not SOPS-encrypted",
			},
		},
		{
			name: "non-string map keys are traversed",
			secretFields: `data:
  nested:
    7: plaintext-number-key
    safe: ` + encrypted,
			wantMessages: []string{
				"Secret data.nested[7] is not SOPS-encrypted",
			},
		},
		{
			name: "arrays are recursively checked",
			secretFields: `data:
  tokens:
    - ` + encrypted + `
    - plaintext-array-value
    - nested: plaintext-array-child`,
			wantMessages: []string{
				"Secret data.tokens[1] is not SOPS-encrypted",
				"Secret data.tokens[2].nested is not SOPS-encrypted",
			},
		},
		{
			name: "non-string leaves fail closed",
			secretFields: `data:
  null-value: null
  bool-value: true
  numeric-value: 42
  encrypted-value: ` + encrypted,
			wantMessages: []string{
				"Secret data.bool-value is not SOPS-encrypted",
				"Secret data.null-value is not SOPS-encrypted",
				"Secret data.numeric-value is not SOPS-encrypted",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, root, "secret.yaml", "apiVersion: v1\nkind: Secret\nmetadata:\n  name: nested\n"+tt.secretFields+"\n"+validSOPSMetadata+"\n")

			findings, err := ScanGitOpsSecrets(root)
			if err != nil {
				t.Fatalf("ScanGitOpsSecrets() error = %v", err)
			}

			var gotMessages []string
			for _, finding := range findings {
				if finding.Rule != "plaintext-secret-field" {
					t.Fatalf("unexpected finding = %+v", finding)
				}
				gotMessages = append(gotMessages, finding.Message)
			}
			if !reflect.DeepEqual(gotMessages, tt.wantMessages) {
				t.Fatalf("plaintext-secret-field messages = %#v, want %#v", gotMessages, tt.wantMessages)
			}
		})
	}
}

func TestSecretScannerScansStagedFilesWithSpaces(t *testing.T) {
	root := t.TempDir()
	runGitForScannerTest(t, root, "init")

	writeFile(t, root, "applications/overlays/demo/services/app/plain secret.yaml", `apiVersion: v1
kind: Secret
metadata:
  name: unsafe
stringData:
  password: plaintext
`)
	runGitForScannerTest(t, root, "add", ".")

	findings, err := ScanGitOpsSecretsWithOptions(context.Background(), SecretScanOptions{
		Root:   root,
		Staged: true,
	})
	if err != nil {
		t.Fatalf("ScanGitOpsSecretsWithOptions() error = %v", err)
	}
	assertFinding(t, findings, "unencrypted-kubernetes-secret")
	assertFinding(t, findings, "plaintext-secret-field")
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertFinding(t *testing.T, findings []SecretScanFinding, rule string) {
	t.Helper()
	for _, finding := range findings {
		if finding.Rule == rule {
			return
		}
	}
	var rules []string
	for _, finding := range findings {
		rules = append(rules, finding.Rule)
	}
	t.Fatalf("missing finding rule %q in %s", rule, strings.Join(rules, ", "))
}

func runGitForScannerTest(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(output))
	}
}

func TestSecretScannerDetectsStubSecretChangeme(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, root, "applications/overlays/demo/services/keycloak/secret.yaml", `apiVersion: v1
kind: Secret
metadata:
  name: keycloak-secret
stringData:
  admin_password: CHANGEME
  client_secret: CHANGEME
`)

	findings, err := ScanGitOpsSecrets(root)
	if err != nil {
		t.Fatalf("ScanGitOpsSecrets() error = %v", err)
	}
	assertFinding(t, findings, "stub-secret-changeme")
}

func TestSecretScannerDetectsStubSecretPlaceholder(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, root, "applications/overlays/demo/services/harbor/helm-values/override-values.yaml", `
accesskey: PLACEHOLDER-HARBOR-ACCESS-KEY
secretkey: PLACEHOLDER-HARBOR-SECRET-KEY
harborAdminPassword: PLACEHOLDER-HARBOR-ADMIN-PASSWORD
`)

	findings, err := ScanGitOpsSecrets(root)
	if err != nil {
		t.Fatalf("ScanGitOpsSecrets() error = %v", err)
	}
	assertFinding(t, findings, "stub-secret-placeholder")
}

func TestSecretScannerIgnoresNonSecretFieldsForStubs(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	// A YAML file with CHANGEME in a non-secret field should not trigger.
	writeFile(t, root, "applications/overlays/demo/services/app/config.yaml", `apiVersion: v1
kind: ConfigMap
metadata:
  name: app-config
data:
  description: "This is a CHANGEME example in docs"
  hostname: app.example.com
`)

	findings, err := ScanGitOpsSecrets(root)
	if err != nil {
		t.Fatalf("ScanGitOpsSecrets() error = %v", err)
	}
	// Should not find stub-secret-changeme because "description" is not a secret-related key
	for _, f := range findings {
		if f.Rule == "stub-secret-changeme" || f.Rule == "stub-secret-placeholder" {
			t.Fatalf("unexpected stub finding in non-secret field: %+v", f)
		}
	}
}

func TestSecretScannerDoesNotFlagListYAMLAsInvalid(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	// An Ansible-style playbook is a top-level YAML list — valid YAML, not a map.
	writeFile(t, root, "playbooks/install.yaml", `- name: Install packages
  hosts: all
  become: true
  tasks:
    - name: Install nginx
      apt:
        name: nginx
        state: present
`)

	findings, err := ScanGitOpsSecrets(root)
	if err != nil {
		t.Fatalf("ScanGitOpsSecrets() error = %v", err)
	}
	for _, f := range findings {
		if f.Rule == "invalid-yaml" {
			t.Fatalf("list-rooted YAML incorrectly flagged as invalid-yaml: %+v", f)
		}
	}
}

func TestSecretScannerDoesNotFlagMultiDocListYAMLAsInvalid(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	// Multi-document YAML where some documents are lists and some are maps.
	writeFile(t, root, "resources/mixed.yaml", `- item1
- item2
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: safe
data:
  key: value
---
- another list
`)

	findings, err := ScanGitOpsSecrets(root)
	if err != nil {
		t.Fatalf("ScanGitOpsSecrets() error = %v", err)
	}
	for _, f := range findings {
		if f.Rule == "invalid-yaml" {
			t.Fatalf("multi-doc YAML with list documents incorrectly flagged as invalid-yaml: %+v", f)
		}
	}
}

func TestSecretScannerStillDetectsSecretsAfterListDocument(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	// A list document followed by a Secret — the scanner should skip the list
	// and still inspect the Secret.
	writeFile(t, root, "resources/list-then-secret.yaml", `- preliminary
- data
---
apiVersion: v1
kind: Secret
metadata:
  name: unsafe
stringData:
  password: plaintext
`)

	findings, err := ScanGitOpsSecrets(root)
	if err != nil {
		t.Fatalf("ScanGitOpsSecrets() error = %v", err)
	}
	assertFinding(t, findings, "unencrypted-kubernetes-secret")
	assertFinding(t, findings, "plaintext-secret-field")
	for _, f := range findings {
		if f.Rule == "invalid-yaml" {
			t.Fatalf("should not flag invalid-yaml: %+v", f)
		}
	}
}

func TestSecretScannerStubDetectionSkipsListDocuments(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	// List document should not crash stub-secret scanning, and a map document
	// after the list should still be scanned for stubs.
	writeFile(t, root, "resources/list-then-values.yaml", `- hosts: all
  tasks: []
---
harborAdminPassword: PLACEHOLDER-HARBOR-ADMIN-PASSWORD
secretkey: PLACEHOLDER-HARBOR-SECRET-KEY
`)

	findings, err := ScanGitOpsSecrets(root)
	if err != nil {
		t.Fatalf("ScanGitOpsSecrets() error = %v", err)
	}
	assertFinding(t, findings, "stub-secret-placeholder")
	for _, f := range findings {
		if f.Rule == "invalid-yaml" {
			t.Fatalf("should not flag invalid-yaml: %+v", f)
		}
	}
}

func TestSecretScannerDetectsChangemeInSecretManifest(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	// CHANGEME in a Secret's stringData should always be caught regardless of key name.
	writeFile(t, root, "applications/overlays/demo/services/loki/secret.yaml", `apiVersion: v1
kind: Secret
metadata:
  name: loki-storage
stringData:
  swift_password: CHANGEME
`)

	findings, err := ScanGitOpsSecrets(root)
	if err != nil {
		t.Fatalf("ScanGitOpsSecrets() error = %v", err)
	}
	assertFinding(t, findings, "stub-secret-changeme")
}
