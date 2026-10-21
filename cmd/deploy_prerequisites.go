package cmd

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

type deployPrerequisiteRunner struct {
	lookPath      func(string) (string, error)
	commandOutput func(string, ...string) (string, error)
}

var helmMajorVersionPattern = regexp.MustCompile(`(?:^|[^0-9])v?([0-9]+)\.`)

func checkDeployPrerequisites() error {
	return checkDeployPrerequisitesWith(deployPrerequisiteRunner{
		lookPath: exec.LookPath,
		commandOutput: func(name string, args ...string) (string, error) {
			output, err := exec.Command(name, args...).CombinedOutput()
			return string(output), err
		},
	})
}

func checkDeployPrerequisitesWith(runner deployPrerequisiteRunner) error {
	var failures []string

	if _, err := runner.lookPath("sops"); err != nil {
		failures = append(failures, "sops was not found in PATH")
	}

	helmPath, err := runner.lookPath("helm")
	if err != nil {
		failures = append(failures, "Helm 4 was not found in PATH")
	} else {
		output, versionErr := runner.commandOutput(helmPath, "version", "--short")
		if versionErr != nil {
			failures = append(failures, fmt.Sprintf("could not determine Helm version: %v", versionErr))
		} else {
			match := helmMajorVersionPattern.FindStringSubmatch(output)
			if len(match) != 2 {
				failures = append(failures, fmt.Sprintf("could not determine Helm major version from %q", strings.TrimSpace(output)))
			} else if match[1] != "4" {
				failures = append(failures, fmt.Sprintf("Helm 4 is required, found major version %s", match[1]))
			}
		}
	}

	if len(failures) == 0 {
		return nil
	}
	return fmt.Errorf("deploy prerequisites not met:\n- %s\nInstall the missing prerequisite(s), then retry", strings.Join(failures, "\n- "))
}
