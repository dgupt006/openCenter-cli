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

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// TestCalicoWindowsDataplaneIsNestedUnderCalicoNetwork guards OCTR-566.
//
// The Installation CRD (operator.tigera.io/v1) declares windowsDataplane only at
// .spec.calicoNetwork.windowsDataplane. Emitting it as a sibling of calicoNetwork
// produces ".spec.windowsDataplane: field not declared in schema" during
// server-side apply, which fails the Calico install.
func TestCalicoWindowsDataplaneIsNestedUnderCalicoNetwork(t *testing.T) {
	cases := []struct {
		name          string
		windowsWorker int
		want          string
	}{
		{name: "no windows workers", windowsWorker: 0, want: "Disabled"},
		{name: "windows workers present", windowsWorker: 2, want: "HNS"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dst := t.TempDir()
			cfg := newDefault("calico-windows-dataplane")
			cfg.OpenCenter.Cluster.ClusterName = "calico-windows-dataplane"
			cfg.OpenCenter.GitOps.Repository.LocalDir = dst
			cfg.OpenCenter.Infrastructure.Compute.WorkerCountWindows = tc.windowsWorker

			require.NoError(t, RenderClusterApps(cfg))

			overlay := filepath.Join(dst, "applications", "overlays", cfg.ClusterName(), "services", "calico")

			overrideValues := readYAMLMap(t, filepath.Join(overlay, "helm-values", "override_values.yaml"))
			assertWindowsDataplaneNesting(t, mapAt(t, overrideValues, "installation"), tc.want)

			helmRelease := readYAMLMap(t, filepath.Join(overlay, "helmrelease.yaml"))
			values := mapAt(t, mapAt(t, helmRelease, "spec"), "values")
			assertWindowsDataplaneNesting(t, mapAt(t, values, "installation"), tc.want)
		})
	}
}

func assertWindowsDataplaneNesting(t *testing.T, installation map[string]any, want string) {
	t.Helper()

	_, topLevel := installation["windowsDataplane"]
	require.False(t, topLevel,
		"windowsDataplane must not be set at installation (.spec) level; the Installation CRD does not declare it there")

	calicoNetwork := mapAt(t, installation, "calicoNetwork")
	require.Equal(t, want, calicoNetwork["windowsDataplane"],
		"expected installation.calicoNetwork.windowsDataplane to be %q", want)
}

func readYAMLMap(t *testing.T, path string) map[string]any {
	t.Helper()

	data, err := os.ReadFile(path) //nolint:gosec // test reads a file it just rendered
	require.NoError(t, err, "failed to read rendered %s", path)

	out := map[string]any{}
	require.NoError(t, yaml.Unmarshal(data, &out), "failed to parse rendered %s", path)

	return out
}

func mapAt(t *testing.T, parent map[string]any, key string) map[string]any {
	t.Helper()

	child, ok := parent[key].(map[string]any)
	require.True(t, ok, "expected %q to be a mapping, got %T", key, parent[key])

	return child
}
