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

	"github.com/spf13/cobra"
)

// TestClusterInitOpenStackBindingFlagRegistration verifies --os-cloud and the
// hidden provider selector flags exist on 'cluster init' and stay hidden.
func TestClusterInitOpenStackBindingFlagRegistration(t *testing.T) {
	cmd := newClusterInitCmd()
	if cmd.Flags().Lookup("os-cloud") == nil {
		t.Fatal("cluster init missing --os-cloud")
	}
	for _, name := range providerOpenStackSelectorFlagNames {
		flag := cmd.Flags().Lookup(name)
		if flag == nil {
			t.Fatalf("cluster init missing --%s", name)
		}
		if !flag.Hidden {
			t.Fatalf("--%s must be hidden on cluster init", name)
		}
	}
}

// TestClusterInitOpenStackBindingFlagValidation covers the flag-level contract
// of parseInitProviderBindingFlags: passthrough values, the --os-cloud
// requirement for selector flags, and the conflicting --type rejection.
func TestClusterInitOpenStackBindingFlagValidation(t *testing.T) {
	t.Run("passthrough", func(t *testing.T) {
		cmd := newClusterInitCmd()
		if err := cmd.Flags().Parse([]string{"--os-cloud", "prod",
			"--clouds-yaml", "/tmp/clouds.yaml", "--image-name", "Ubuntu 24.04",
			"--create-internal-network", "--import-auth", "--import-tls", "--replace"}); err != nil {
			t.Fatal(err)
		}
		flags, err := parseInitProviderBindingFlags(cmd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if flags == nil {
			t.Fatal("expected provider binding flags, got nil")
		}
		if flags.cloudName != "prod" || flags.cloudsYAML != "/tmp/clouds.yaml" ||
			flags.imageName != "Ubuntu 24.04" || !flags.createInternalNetwork ||
			!flags.importAuth || !flags.importTLS || !flags.replace {
			t.Fatalf("passthrough mismatch: %+v", *flags)
		}
	})

	t.Run("selector flag without os-cloud is rejected", func(t *testing.T) {
		cmd := newClusterInitCmd()
		if err := cmd.Flags().Parse([]string{"--create-internal-network"}); err != nil {
			t.Fatal(err)
		}
		if _, err := parseInitProviderBindingFlags(cmd); err == nil ||
			!strings.Contains(err.Error(), "--create-internal-network is only valid together with --os-cloud") {
			t.Fatalf("error = %v, want selector-without-os-cloud rejection", err)
		}
	})

	t.Run("conflicting type is rejected", func(t *testing.T) {
		cmd := newClusterInitCmd()
		if err := cmd.Flags().Parse([]string{"--os-cloud", "prod", "--type", "kind"}); err != nil {
			t.Fatal(err)
		}
		if _, err := parseInitProviderBindingFlags(cmd); err == nil ||
			!strings.Contains(err.Error(), "--os-cloud implies --type openstack") {
			t.Fatalf("error = %v, want conflicting --type rejection", err)
		}
	})

	t.Run("absent os-cloud yields nil", func(t *testing.T) {
		cmd := newClusterInitCmd()
		flags, err := parseInitProviderBindingFlags(cmd)
		if err != nil || flags != nil {
			t.Fatalf("flags = %v, err = %v, want nil, nil", flags, err)
		}
	})
}

// TestClusterInitOpenStackBindingPreflightWritesNothing proves an unusable
// cloud profile fails 'cluster init --os-cloud' before any cluster file is
// written: neither a missing clouds.yaml nor a name-only application
// credential with --import-auth may leave a half-created cluster behind.
func TestClusterInitOpenStackBindingPreflightWritesNothing(t *testing.T) {
	tests := []struct {
		name        string
		cloudsFile  string
		cloudsBody  string
		extraArgs   []string
		wantMessage string
	}{
		{
			name:        "missing clouds.yaml",
			cloudsFile:  "missing-clouds.yaml",
			extraArgs:   nil,
			wantMessage: "read clouds.yaml",
		},
		{
			name:        "name-only application credential with --import-auth",
			cloudsFile:  "clouds.yaml",
			cloudsBody:  "clouds:\n  prod:\n    region_name: IAD3\n    auth:\n      auth_url: https://keystone.invalid.local/v3\n      application_credential_name: only-name\n      application_credential_secret: s\n",
			extraArgs:   []string{"--import-auth"},
			wantMessage: "--import-auth requires an application credential ID and secret",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			prepareCommandTestEnv(t, dir)
			cloudsPath := filepath.Join(dir, tt.cloudsFile)
			if tt.cloudsBody != "" {
				if err := os.WriteFile(cloudsPath, []byte(tt.cloudsBody), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			original := runInitProviderOpenStackApply
			seamCalled := false
			runInitProviderOpenStackApply = func(*cobra.Command, string, bool, string, providerOpenStackFlags) error {
				seamCalled = true
				return nil
			}
			t.Cleanup(func() { runInitProviderOpenStackApply = original })

			cmd := newClusterInitCmd()
			var stdout, stderr bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			args := []string{"bind-me", "--org", "acme", "--no-keygen",
				"--os-cloud", "prod", "--clouds-yaml", cloudsPath}
			cmd.SetArgs(append(args, tt.extraArgs...))
			err := cmd.Execute()
			if err == nil || !strings.Contains(err.Error(), tt.wantMessage) {
				t.Fatalf("error = %v, want %q", err, tt.wantMessage)
			}
			if seamCalled {
				t.Fatal("provider apply pass must not run when preflight fails")
			}
			if strings.Contains(stdout.String(), "Created cluster configuration") {
				t.Fatalf("init must not run after preflight failure: %q", stdout.String())
			}
			configPath := filepath.Join(dir, "clusters", "blueprints", "acme", "bind-me", "bind-me-config.yaml")
			if _, statErr := os.Stat(configPath); !os.IsNotExist(statErr) {
				t.Fatalf("expected no cluster config after preflight failure, stat err = %v", statErr)
			}
		})
	}
}

// TestClusterInitOpenStackBindingInvokesApplyPass proves the one-step flow
// invokes the provider apply pass once, bound to the freshly created cluster
// with the forwarded selectors, after the config exists on disk.
func TestClusterInitOpenStackBindingInvokesApplyPass(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)
	cloudsPath := filepath.Join(dir, "clouds.yaml")
	if err := os.WriteFile(cloudsPath, []byte("clouds:\n  prod:\n    region_name: IAD3\n    auth:\n      auth_url: https://keystone.invalid.local/v3\n      application_credential_id: id\n      application_credential_secret: secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	original := runInitProviderOpenStackApply
	t.Cleanup(func() { runInitProviderOpenStackApply = original })

	type call struct {
		operation  string
		mutating   bool
		identifier string
		flags      providerOpenStackFlags
		configSeen bool
	}
	var calls []call
	runInitProviderOpenStackApply = func(cmd *cobra.Command, operation string, mutating bool, identifier string, flags providerOpenStackFlags) error {
		configPath := filepath.Join(dir, "clusters", "blueprints", "acme", "one-step", "one-step-config.yaml")
		_, statErr := os.Stat(configPath)
		calls = append(calls, call{operation, mutating, identifier, flags, statErr == nil})
		return nil
	}

	cmd := newClusterInitCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"one-step", "--org", "acme", "--no-keygen",
		"--os-cloud", "prod", "--clouds-yaml", cloudsPath,
		"--create-internal-network", "--import-auth"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v\nstderr: %s", err, stderr.String())
	}
	if len(calls) != 1 {
		t.Fatalf("provider apply calls = %d, want 1", len(calls))
	}
	got := calls[0]
	if got.operation != "apply" || !got.mutating || got.identifier != "acme/one-step" {
		t.Fatalf("apply call = %+v, want apply/mutating/acme/one-step", got)
	}
	if !got.configSeen {
		t.Fatal("provider apply ran before the cluster config was written")
	}
	if got.flags.cloudName != "prod" || got.flags.cloudsYAML != cloudsPath ||
		!got.flags.createInternalNetwork || !got.flags.importAuth {
		t.Fatalf("forwarded flags mismatch: %+v", got.flags)
	}
	if !strings.Contains(stdout.String(), "Created cluster configuration") {
		t.Fatalf("init result missing from output: %q", stdout.String())
	}
}
