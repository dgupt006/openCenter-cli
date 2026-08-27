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
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

type doctorExecutableLookup func(string) (string, error)

type doctorBinaryCheck struct {
	binary     string
	candidates []string
}

type doctorCheckResult struct {
	Binary string `json:"binary" yaml:"binary"`
	Status string `json:"status" yaml:"status"`
}

type doctorReport struct {
	Status  string              `json:"status" yaml:"status"`
	Present int                 `json:"present" yaml:"present"`
	Missing int                 `json:"missing" yaml:"missing"`
	Checks  []doctorCheckResult `json:"checks" yaml:"checks"`
}

var doctorBinaryCatalog = []doctorBinaryCheck{
	{binary: "git", candidates: []string{"git"}},
	{binary: "kubectl", candidates: []string{"kubectl"}},
	{binary: "helm", candidates: []string{"helm"}},
	{binary: "flux", candidates: []string{"flux"}},
	{binary: "sops", candidates: []string{"sops"}},
	{binary: "tofu|terraform", candidates: []string{"tofu", "terraform"}},
	{binary: "kind", candidates: []string{"kind"}},
	{binary: "podman|docker", candidates: []string{"podman", "docker"}},
	{binary: "ssh", candidates: []string{"ssh"}},
	{binary: "ssh-keyscan", candidates: []string{"ssh-keyscan"}},
	{binary: "ssh-keygen", candidates: []string{"ssh-keygen"}},
}

func newClusterDoctorCmd() *cobra.Command {
	return newClusterDoctorCmdWithLookup(exec.LookPath)
}

func newClusterDoctorCmdWithLookup(lookup doctorExecutableLookup) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Audit local executable prerequisites",
		Long: `Audit the local host's required executables without loading cluster configuration,
resolving an active cluster, contacting a provider, or changing state.

The audit checks a fixed all-provider catalog. tofu or terraform satisfies the
OpenTofu row, and podman or docker satisfies the container row. The openstack
CLI and external age executable are intentionally not checked.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts := getGlobalOptions(cmd)
			if opts.Output == "" {
				opts.Output = OutputText
			}
			report := runClusterDoctor(lookup)

			if opts.Output == OutputText {
				writeClusterDoctorText(cmd, report, opts.Quiet)
			} else if err := writeStructuredOutput(cmd, opts.Output, report); err != nil {
				return err
			}

			if report.Missing > 0 {
				return NewExitError(1, "cluster doctor found missing binaries", nil)
			}
			return nil
		},
	}
	markReadOnlyCommand(cmd)
	return cmd
}

func runClusterDoctor(lookup doctorExecutableLookup) doctorReport {
	report := doctorReport{Checks: make([]doctorCheckResult, 0, len(doctorBinaryCatalog))}
	for _, check := range doctorBinaryCatalog {
		status := "missing"
		for _, candidate := range check.candidates {
			if _, err := lookup(candidate); err == nil {
				status = "present"
				break
			}
		}
		report.Checks = append(report.Checks, doctorCheckResult{Binary: check.binary, Status: status})
		if status == "present" {
			report.Present++
		} else {
			report.Missing++
		}
	}
	if report.Missing == 0 {
		report.Status = "present"
	} else {
		report.Status = "missing"
	}
	return report
}

func writeClusterDoctorText(cmd *cobra.Command, report doctorReport, quiet bool) {
	result := doctorResultText(report)
	if quiet {
		fmt.Fprintln(cmd.OutOrStdout(), result)
		return
	}

	fmt.Fprintln(cmd.OutOrStdout(), "BINARY             STATUS")
	for _, check := range report.Checks {
		fmt.Fprintf(cmd.OutOrStdout(), "%-18s %s\n", check.Binary, strings.ToUpper(check.Status))
	}
	fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", result)
}

func doctorResultText(report doctorReport) string {
	if report.Missing == 0 {
		return "RESULT: ALL BINARIES PRESENT"
	}
	word := "BINARY"
	if report.Missing != 1 {
		word = "BINARIES"
	}
	return fmt.Sprintf("RESULT: MISSING %d %s", report.Missing, word)
}
