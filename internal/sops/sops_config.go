/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sops

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/crypto"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/fs"
)

// SOPSConfigFileName is the SOPS creation-rule file that governs every
// manifest inside a GitOps organization repository.
const SOPSConfigFileName = ".sops.yaml"

// SOPSConfigResult describes what EnsureSOPSConfig did.
type SOPSConfigResult int

const (
	// SOPSConfigUnchanged means the file already existed and was left alone.
	SOPSConfigUnchanged SOPSConfigResult = iota
	// SOPSConfigCreated means the file was missing and has been written.
	SOPSConfigCreated
)

// EnsureSOPSConfig writes the GitOps-repository .sops.yaml at configPath when
// it is missing, deriving recipients from the cluster's Age key. It never
// overwrites an existing file: rotation, revocation, and hand edits own the
// file after creation, and a repair path that clobbered them would destroy
// added recipients.
//
// The file is the input to creation-rule consumers (key rotation and
// revocation re-encryption via reencryptManifestUsingCreationRule, path
// matching via SOPSPathMatcher, and the render validation stage). Those
// consumers silently treat a missing file as "no rules", so every flow that
// encrypts into the repository must ensure the config exists first, not only
// cluster init.
//
// An empty cfg.Secrets.SopsAgeKeyFile is treated as "SOPS not configured for
// this cluster": Ensure returns SOPSConfigUnchanged without error. Callers
// that require encryption surface their own key-not-found error separately.
func EnsureSOPSConfig(configPath string, cfg *v2.Config, fileSystem fs.FileSystem) (SOPSConfigResult, error) {
	if cfg == nil {
		return SOPSConfigUnchanged, fmt.Errorf("cannot ensure SOPS config with nil configuration")
	}
	if strings.TrimSpace(configPath) == "" {
		return SOPSConfigUnchanged, fmt.Errorf("SOPS config path cannot be empty")
	}
	if fileSystem == nil {
		fileSystem = fs.NewDefaultFileSystem(nil)
	}

	if strings.TrimSpace(cfg.Secrets.SopsAgeKeyFile) == "" {
		return SOPSConfigUnchanged, nil
	}

	// os.Stat (not the wrapped FileSystem) so os.IsNotExist sees the raw
	// errno: fs.DefaultFileSystem.Stat wraps it in a StructuredError whose Is
	// matches only other StructuredErrors, which would mask ENOENT/ENOTDIR
	// and turn a missing config into a spurious hard error.
	if _, err := os.Stat(configPath); err == nil {
		return SOPSConfigUnchanged, nil
	} else if !os.IsNotExist(err) {
		return SOPSConfigUnchanged, fmt.Errorf("stat SOPS config %s: %w", configPath, err)
	}

	// A missing/unreadable key file must not fail the ensure on its own: the
	// config is a repair, not a prerequisite. The caller's own key resolution
	// (e.g. getAgeKey) produces the authoritative key-not-found error.
	publicKey, err := AgePublicKey(cfg.Secrets.SopsAgeKeyFile)
	if err != nil {
		return SOPSConfigUnchanged, nil
	}

	content := fmt.Sprintf(`creation_rules:
  - path_regex: 'secrets/age/keys/.*-key\.txt$'
    age: >-
      %s
  - path_regex: 'secrets/ssh/[^.]+$'
    age: >-
      %s
  - path_regex: 'applications/overlays/[^/]+/(managed-services|services)/.*/.*\.ya?ml$'
    encrypted_regex: "^(data|stringData)$"
    age: >-
      %s
  - path_regex: '^infrastructure/clusters/%s/.*\.ya?ml$'
    encrypted_regex: "^(data|stringData|secret)$"
    age: >-
      %s
`, publicKey, publicKey, publicKey, cfg.ClusterName(), publicKey)

	if err := fileSystem.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return SOPSConfigUnchanged, fmt.Errorf("creating SOPS config directory: %w", err)
	}
	if err := fileSystem.WriteFileAtomic(configPath, []byte(content), 0o644); err != nil {
		return SOPSConfigUnchanged, fmt.Errorf("writing SOPS config %s: %w", configPath, err)
	}
	return SOPSConfigCreated, nil
}

// AgePublicKey derives the Age public key recipient from the private key file
// at keyPath. The public key is derived from the identity rather than read
// from a comment line, so a hand-edited or comment-less key file still yields
// the correct recipient.
func AgePublicKey(keyPath string) (string, error) {
	if strings.TrimSpace(keyPath) == "" {
		return "", fmt.Errorf("SOPS age key path cannot be empty")
	}

	data, err := os.ReadFile(keyPath)
	if err != nil {
		return "", fmt.Errorf("reading SOPS age key: %w", err)
	}

	privateKey := ExtractAgePrivateKey(string(data))
	if privateKey == "" {
		return "", fmt.Errorf("no valid age private key found in %s", keyPath)
	}

	keyPair, err := crypto.ParseAgeKey(privateKey)
	if err != nil {
		return "", fmt.Errorf("parsing SOPS age key: %w", err)
	}

	return strings.TrimSpace(keyPair.PublicKey), nil
}

// ExtractAgePrivateKey returns the first AGE-SECRET-KEY- line in content.
func ExtractAgePrivateKey(content string) string {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "AGE-SECRET-KEY-") {
			return trimmed
		}
	}
	return ""
}
