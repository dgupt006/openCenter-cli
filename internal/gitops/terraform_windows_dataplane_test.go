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
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// windowsDataplaneAssignment matches a quoted windows_dataplane literal in
// rendered Terraform, e.g. `windows_dataplane = "HNS"`. Assignments that
// reference a local (windows_dataplane = local.windows_dataplane) carry no
// literal and are intentionally not matched.
var windowsDataplaneAssignment = regexp.MustCompile(`windows_dataplane\s*=\s*"([^"]*)"`)

// windowsDataplaneLine matches any template or Terraform line mentioning
// windows_dataplane. Used for source-level scanning, where the value is often
// produced by a conditional and so is not adjacent to the `=`.
var windowsDataplaneLine = regexp.MustCompile(`(?m)^.*windows_dataplane.*$`)

// quotedLiteral matches a double-quoted literal.
var quotedLiteral = regexp.MustCompile(`"([^"]*)"`)

// TestTerraformWindowsDataplaneValues guards the Calico Windows dataplane value
// emitted into infrastructure/clusters/<cluster>/main.tf.
//
// The Calico Installation CRD accepts only HNS, VXLAN, Disabled or the empty
// string for windowsDataplane. main-default.tf.tpl carried the transposition
// "HSN", which is not a valid value and reaches Terraform/kubespray rather than
// failing at generation time. This test renders every provider variant and
// asserts each quoted assignment is the expected value, so a typo in any one
// template cannot ship again.
func TestTerraformWindowsDataplaneValues(t *testing.T) {
	providers := []string{"openstack", "baremetal", "vmware"}

	cases := []struct {
		name          string
		windowsWorker int
		want          string
	}{
		{name: "no windows workers", windowsWorker: 0, want: "Disabled"},
		{name: "windows workers present", windowsWorker: 3, want: "HNS"},
	}

	for _, provider := range providers {
		for _, tc := range cases {
			t.Run(provider+"/"+tc.name, func(t *testing.T) {
				dst := t.TempDir()
				cfg := newDefault("tf-windows-dataplane")
				cfg.OpenCenter.Cluster.ClusterName = "tf-windows-dataplane"
				cfg.OpenCenter.GitOps.Repository.LocalDir = dst
				cfg.OpenCenter.Infrastructure.Provider = provider
				cfg.OpenCenter.Infrastructure.Compute.WorkerCountWindows = tc.windowsWorker
				// The openstack/default template only emits a quoted
				// windows_dataplane inside the kubespray-managed calico module;
				// with the default helm install method that block is skipped and
				// nothing would be asserted.
				cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.InstallMethod = "kubespray"

				require.NoError(t, RenderInfrastructureCluster(cfg))

				mainTf := filepath.Join(dst, "infrastructure", "clusters", cfg.ClusterName(), "main.tf")
				data, err := os.ReadFile(mainTf) //nolint:gosec // test reads a file it just rendered
				require.NoError(t, err, "failed to read rendered %s", mainTf)
				rendered := string(data)

				matches := windowsDataplaneAssignment.FindAllStringSubmatch(rendered, -1)
				require.NotEmpty(t, matches,
					"provider %q rendered no quoted windows_dataplane assignment; the template no longer sets it", provider)

				for _, match := range matches {
					require.Equal(t, tc.want, match[1],
						"provider %q: windows_dataplane must be %q for %d windows workers, got %q",
						provider, tc.want, tc.windowsWorker, match[1])
				}

				// Explicit guard against the original transposition, independent of
				// the assertion above, so the failure names the actual bug.
				require.NotContains(t, rendered, "HSN",
					"provider %q: %q is not a valid Calico windowsDataplane value (did you mean HNS?)", provider, "HSN")
			})
		}
	}
}

// TestTerraformTemplatesUseNoInvalidWindowsDataplaneLiteral is a source-level
// backstop. It scans the embedded provider templates directly rather than their
// rendered output, because a bad literal can sit in a branch that no render case
// reaches -- the original "HSN" typo lived inside a `provider == "baremetal"`
// guard in main-default.tf.tpl, which copy.go never selects for baremetal, so it
// was unreachable and invisible to a render-only test.
func TestTerraformTemplatesUseNoInvalidWindowsDataplaneLiteral(t *testing.T) {
	templates := []string{
		"templates/infrastructure-cluster-template/main-default.tf.tpl",
		"templates/infrastructure-cluster-template/main-baremetal.tf.tpl",
		"templates/infrastructure-cluster-template/main-vmware.tf.tpl",
	}

	// Values the Calico Installation CRD accepts for windowsDataplane.
	valid := map[string]bool{"HNS": true, "VXLAN": true, "Disabled": true, "": true}

	for _, name := range templates {
		t.Run(filepath.Base(name), func(t *testing.T) {
			data, err := Files.ReadFile(name)
			require.NoError(t, err, "failed to read embedded template %s", name)

			lines := windowsDataplaneLine.FindAllString(string(data), -1)
			require.NotEmpty(t, lines, "%s: no windows_dataplane reference found", name)

			checked := 0
			for _, line := range lines {
				for _, match := range quotedLiteral.FindAllStringSubmatch(line, -1) {
					value := match[1]
					checked++
					require.True(t, valid[value],
						"%s: %q is not a valid Calico windowsDataplane value (expected one of HNS, VXLAN, Disabled)\n  line: %s",
						name, value, strings.TrimSpace(line))
				}
			}
			require.Positive(t, checked,
				"%s: found windows_dataplane lines but no quoted literal to validate", name)
		})
	}
}
