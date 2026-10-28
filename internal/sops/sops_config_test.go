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
	"os"
	"path/filepath"
	"strings"
	"testing"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/crypto"
	"gopkg.in/yaml.v3"
)

func writeAgeKeyFile(t *testing.T, dir string) (path, publicKey string) {
	t.Helper()
	km := crypto.NewDefaultKeyManager(dir)
	keyPair, err := km.GenerateAgeKey()
	if err != nil {
		t.Fatalf("generate age key: %v", err)
	}
	path = filepath.Join(dir, "cluster-key.txt")
	content := "# public key: " + keyPair.PublicKey + "\n" + keyPair.PrivateKey + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write age key: %v", err)
	}
	return path, keyPair.PublicKey
}

func TestEnsureSOPSConfigCreatesFileWithRecipients(t *testing.T) {
	dir := t.TempDir()
	keyPath, publicKey := writeAgeKeyFile(t, dir)

	cfg := &v2.Config{}
	cfg.Secrets.SopsAgeKeyFile = keyPath
	cfg.OpenCenter.Cluster.ClusterName = "demo"

	configPath := filepath.Join(dir, "gitops", "acme", ".sops.yaml")
	result, err := EnsureSOPSConfig(configPath, cfg, nil)
	if err != nil {
		t.Fatalf("EnsureSOPSConfig: %v", err)
	}
	if result != SOPSConfigCreated {
		t.Fatalf("result = %v, want SOPSConfigCreated", result)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read created config: %v", err)
	}
	text := string(data)
	if strings.Count(text, publicKey) != 4 {
		t.Fatalf("expected the public key in all four creation rules:\n%s", text)
	}
	if !strings.Contains(text, "encrypted_regex: \"^(data|stringData)$\"") {
		t.Fatalf("missing Secret regex:\n%s", text)
	}
	if !strings.Contains(text, "encrypted_regex: \"^(data|stringData|secret)$\"") {
		t.Fatalf("missing infrastructure regex:\n%s", text)
	}
	// The key-material trees must be matched only by their exact anchored
	// rules; a broad rule sweeping secrets/ would encrypt (or expose) the
	// private key files themselves.
	for _, rule := range strings.Split(text, "  - path_regex: ")[1:] {
		trimmed := strings.SplitN(rule, "\n", 2)[0]
		if strings.Contains(trimmed, "secrets/age/") || strings.Contains(trimmed, "secrets/ssh/") {
			continue
		}
		if strings.Contains(rule, "secrets/") {
			t.Fatalf("a non-anchored rule sweeps the secrets tree:\n%s", rule)
		}
	}
	if info, err := os.Stat(configPath); err != nil || info.Mode().Perm() != 0o644 {
		t.Fatalf("config mode = %v, %v; want 0644", info.Mode().Perm(), err)
	}

	// The creation rules must actually match the manifests the sync writes:
	// this file is what rotation/revocation re-encryption resolves against.
	var document struct {
		CreationRules []struct {
			PathRegex string `yaml:"path_regex"`
			Age       string `yaml:"age"`
		} `yaml:"creation_rules"`
	}
	if err := yaml.Unmarshal(data, &document); err != nil {
		t.Fatalf("created config is not valid YAML: %v", err)
	}
	if len(document.CreationRules) != 4 {
		t.Fatalf("creation rules = %d, want 4", len(document.CreationRules))
	}
}

func TestEnsureSOPSConfigNeverOverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	keyPath, _ := writeAgeKeyFile(t, dir)

	cfg := &v2.Config{}
	cfg.Secrets.SopsAgeKeyFile = keyPath
	cfg.OpenCenter.Cluster.ClusterName = "demo"

	configPath := filepath.Join(dir, ".sops.yaml")
	handEdited := "creation_rules:\n  - path_regex: .*\n    age: age1rotatedaddedrecipientzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz\n"
	if err := os.WriteFile(configPath, []byte(handEdited), 0o644); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	result, err := EnsureSOPSConfig(configPath, cfg, nil)
	if err != nil {
		t.Fatalf("EnsureSOPSConfig: %v", err)
	}
	if result != SOPSConfigUnchanged {
		t.Fatalf("result = %v, want SOPSConfigUnchanged", result)
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if string(data) != handEdited {
		t.Fatalf("existing config was rewritten:\n%s", data)
	}
}

func TestEnsureSOPSConfigSkipsWithoutAgeKey(t *testing.T) {
	dir := t.TempDir()
	cfg := &v2.Config{}
	configPath := filepath.Join(dir, ".sops.yaml")

	result, err := EnsureSOPSConfig(configPath, cfg, nil)
	if err != nil {
		t.Fatalf("EnsureSOPSConfig with empty key file: %v", err)
	}
	if result != SOPSConfigUnchanged {
		t.Fatalf("result = %v, want SOPSConfigUnchanged", result)
	}
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatalf("config created without an age key: %v", err)
	}
}

func TestEnsureSOPSConfigSkipsOnUnreadableKey(t *testing.T) {
	cfg := &v2.Config{}
	cfg.Secrets.SopsAgeKeyFile = filepath.Join(t.TempDir(), "missing-key.txt")

	configPath := filepath.Join(t.TempDir(), ".sops.yaml")
	result, err := EnsureSOPSConfig(configPath, cfg, nil)
	if err != nil {
		t.Fatalf("unreadable key must not fail the ensure: %v", err)
	}
	if result != SOPSConfigUnchanged {
		t.Fatalf("result = %v, want SOPSConfigUnchanged", result)
	}
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatalf("config created without a usable age key: %v", err)
	}
}

func TestAgePublicKeyDerivesFromIdentity(t *testing.T) {
	dir := t.TempDir()
	keyPair, err := crypto.NewDefaultKeyManager(dir).GenerateAgeKey()
	if err != nil {
		t.Fatalf("generate age key: %v", err)
	}
	// No "# public key:" comment: the recipient must still be derived.
	path := filepath.Join(dir, "bare-key.txt")
	if err := os.WriteFile(path, []byte(keyPair.PrivateKey+"\n"), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	got, err := AgePublicKey(path)
	if err != nil {
		t.Fatalf("AgePublicKey: %v", err)
	}
	if got != keyPair.PublicKey {
		t.Fatalf("public key = %s, want %s", got, keyPair.PublicKey)
	}
}
