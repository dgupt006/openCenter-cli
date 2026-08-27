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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type doctorTestLookup struct {
	present map[string]bool
	calls   []string
}

func (f *doctorTestLookup) lookup(binary string) (string, error) {
	f.calls = append(f.calls, binary)
	if f.present[binary] {
		return "/fake/bin/" + binary, nil
	}
	return "", errors.New("not found")
}

func executeDoctorTest(t *testing.T, lookup doctorExecutableLookup, args []string, opts GlobalOptions) (string, string, error) {
	t.Helper()
	cmd := newClusterDoctorCmdWithLookup(lookup)
	cmd.SilenceUsage = true
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetContext(context.WithValue(context.Background(), globalOptionsContextKey{}, opts))
	cmd.SetArgs(args)
	err := cmd.Execute()
	return stdout.String(), stderr.String(), err
}

func allDoctorBinaries() map[string]bool {
	present := make(map[string]bool)
	for _, check := range doctorBinaryCatalog {
		for _, binary := range check.candidates {
			present[binary] = true
		}
	}
	return present
}

func hasDoctorTextRow(output, binary, status string) bool {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == binary && fields[1] == status {
			return true
		}
	}
	return false
}

func containsDoctorLookupSequence(calls, want []string) bool {
	for start := 0; start+len(want) <= len(calls); start++ {
		matched := true
		for offset := range want {
			if calls[start+offset] != want[offset] {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

func TestClusterDoctorUsesExactCatalogOrder(t *testing.T) {
	lookup := &doctorTestLookup{present: allDoctorBinaries()}
	stdout, _, err := executeDoctorTest(t, lookup.lookup, nil, GlobalOptions{Output: OutputText})
	if err != nil {
		t.Fatalf("doctor failed: %v", err)
	}

	var got []string
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		if strings.HasPrefix(line, "RESULT:") || strings.HasPrefix(line, "BINARY") || strings.TrimSpace(line) == "" {
			continue
		}
		got = append(got, strings.Fields(line)[0])
	}
	want := []string{"git", "kubectl", "helm", "flux", "sops", "tofu|terraform", "kind", "podman|docker", "ssh", "ssh-keyscan", "ssh-keygen"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("catalog order = %v, want %v\noutput:\n%s", got, want, stdout)
	}
	if !strings.Contains(stdout, "\n\nRESULT: ALL BINARIES PRESENT\n") {
		t.Fatalf("expected table followed by blank line and success result, got:\n%s", stdout)
	}
	if got := lookup.calls; fmt.Sprint(got) != fmt.Sprint([]string{"git", "kubectl", "helm", "flux", "sops", "tofu", "kind", "podman", "ssh", "ssh-keyscan", "ssh-keygen"}) {
		t.Fatalf("lookup order = %v", got)
	}
}

func TestClusterDoctorRejectsPositionalArguments(t *testing.T) {
	lookup := &doctorTestLookup{present: allDoctorBinaries()}
	_, _, err := executeDoctorTest(t, lookup.lookup, []string{"prod"}, GlobalOptions{Output: OutputText})
	if err == nil || !strings.Contains(err.Error(), "unknown command") && !strings.Contains(err.Error(), "accepts 0 arg") {
		t.Fatalf("positional argument error = %v", err)
	}
	if len(lookup.calls) != 0 {
		t.Fatalf("lookup calls after argument rejection = %v", lookup.calls)
	}
}

func TestClusterDoctorGroupedAlternatives(t *testing.T) {
	for _, test := range []struct {
		name    string
		present map[string]bool
		calls   []string
	}{
		{name: "tofu", present: map[string]bool{"tofu": true}, calls: []string{"tofu"}},
		{name: "terraform", present: map[string]bool{"terraform": true}, calls: []string{"tofu", "terraform"}},
		{name: "neither", present: map[string]bool{}, calls: []string{"tofu", "terraform"}},
		{name: "podman", present: map[string]bool{"podman": true}, calls: []string{"podman"}},
		{name: "docker", present: map[string]bool{"docker": true}, calls: []string{"podman", "docker"}},
		{name: "neither container", present: map[string]bool{}, calls: []string{"podman", "docker"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			lookup := &doctorTestLookup{present: test.present}
			stdout, _, err := executeDoctorTest(t, lookup.lookup, nil, GlobalOptions{Output: OutputText})
			if err == nil {
				t.Fatalf("expected missing binaries error")
			}
			if !hasDoctorTextRow(stdout, "tofu|terraform", "PRESENT") && (test.name == "tofu" || test.name == "terraform") {
				t.Fatalf("tofu alternative should pass:\n%s", stdout)
			}
			if !hasDoctorTextRow(stdout, "podman|docker", "PRESENT") && (test.name == "podman" || test.name == "docker") {
				t.Fatalf("container alternative should pass:\n%s", stdout)
			}
			if !hasDoctorTextRow(stdout, "tofu|terraform", "MISSING") && test.name == "neither" {
				t.Fatalf("both tofu alternatives should be missing:\n%s", stdout)
			}
			if !hasDoctorTextRow(stdout, "podman|docker", "MISSING") && test.name == "neither container" {
				t.Fatalf("both container alternatives should be missing:\n%s", stdout)
			}
			if !containsDoctorLookupSequence(lookup.calls, test.calls) {
				t.Fatalf("lookup sequence = %v, want sequence %v", lookup.calls, test.calls)
			}
		})
	}
}

func TestClusterDoctorMissingToolsRendersCompleteTextAndExitOne(t *testing.T) {
	lookup := &doctorTestLookup{present: allDoctorBinaries()}
	delete(lookup.present, "helm")
	delete(lookup.present, "flux")
	stdout, _, err := executeDoctorTest(t, lookup.lookup, nil, GlobalOptions{Output: OutputText})
	if err == nil || ExitCode(err) != 1 {
		t.Fatalf("error = %v, exit code = %d, want exit 1", err, ExitCode(err))
	}
	for _, binary := range []string{"git", "kubectl", "helm", "flux", "ssh-keygen"} {
		if !strings.Contains(stdout, binary) {
			t.Fatalf("complete text output missing %q:\n%s", binary, stdout)
		}
	}
	if !strings.Contains(stdout, "helm") || !strings.Contains(stdout, "flux") || !strings.Contains(stdout, "RESULT: MISSING 2 BINARIES") {
		t.Fatalf("missing result not rendered:\n%s", stdout)
	}
}

func TestClusterDoctorDoesNotLookUpOpenStackOrUseClusterState(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)
	t.Setenv("OPENCENTER_CLUSTER", "opencenter/prod")

	lookup := &doctorTestLookup{present: allDoctorBinaries()}
	stdout, _, err := executeDoctorTest(t, lookup.lookup, nil, GlobalOptions{Output: OutputText})
	if err != nil {
		t.Fatalf("doctor failed without config or active cluster: %v", err)
	}
	for _, binary := range lookup.calls {
		if binary == "openstack" || binary == "age" {
			t.Fatalf("unexpected excluded lookup %q", binary)
		}
	}
	if strings.Contains(stdout, "openstack") || strings.Contains(stdout, "auth_url") || strings.Contains(stdout, "Doctor checks complete") {
		t.Fatalf("output contains provider/config behavior:\n%s", stdout)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read isolated config directory: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("doctor mutated isolated config directory: %v", entries)
	}
}

func TestClusterDoctorQuietTextOnlyEmitsResult(t *testing.T) {
	lookup := &doctorTestLookup{present: allDoctorBinaries()}
	stdout, _, err := executeDoctorTest(t, lookup.lookup, nil, GlobalOptions{Output: OutputText, Quiet: true})
	if err != nil {
		t.Fatalf("quiet doctor failed: %v", err)
	}
	if stdout != "RESULT: ALL BINARIES PRESENT\n" {
		t.Fatalf("quiet output = %q", stdout)
	}
}

func TestClusterDoctorStructuredOutput(t *testing.T) {
	for _, format := range []OutputFormat{OutputJSON, OutputYAML} {
		t.Run(string(format), func(t *testing.T) {
			lookup := &doctorTestLookup{present: allDoctorBinaries()}
			delete(lookup.present, "kubectl")
			stdout, _, err := executeDoctorTest(t, lookup.lookup, nil, GlobalOptions{Output: format})
			if err == nil || ExitCode(err) != 1 {
				t.Fatalf("error = %v, exit code = %d, want exit 1", err, ExitCode(err))
			}
			var document struct {
				Status  string `json:"status" yaml:"status"`
				Present int    `json:"present" yaml:"present"`
				Missing int    `json:"missing" yaml:"missing"`
				Checks  []struct {
					Binary string `json:"binary" yaml:"binary"`
					Status string `json:"status" yaml:"status"`
				} `json:"checks" yaml:"checks"`
			}
			var decodeErr error
			if format == OutputJSON {
				decodeErr = json.Unmarshal([]byte(stdout), &document)
			} else {
				decodeErr = yaml.Unmarshal([]byte(stdout), &document)
			}
			if decodeErr != nil {
				t.Fatalf("decode %s: %v\n%s", format, decodeErr, stdout)
			}
			if document.Status != "missing" || document.Present != 10 || document.Missing != 1 || len(document.Checks) != 11 {
				t.Fatalf("document = %+v", document)
			}
			if document.Checks[5].Binary != "tofu|terraform" || document.Checks[5].Status != "present" {
				t.Fatalf("grouped check = %+v", document.Checks[5])
			}
			if document.Checks[1].Binary != "kubectl" || document.Checks[1].Status != "missing" {
				t.Fatalf("missing check = %+v", document.Checks[1])
			}
		})
	}
}

func TestClusterDoctorCommandUseIsExact(t *testing.T) {
	if got := newClusterDoctorCmdWithLookup((&doctorTestLookup{present: allDoctorBinaries()}).lookup).Use; got != "doctor" {
		t.Fatalf("Use = %q, want doctor", got)
	}
}
