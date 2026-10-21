package cmd

import (
	"errors"
	"strings"
	"testing"
)

func TestCheckDeployPrerequisitesAcceptsHelm4AndSOPS(t *testing.T) {
	err := checkDeployPrerequisitesWith(deployPrerequisiteRunner{
		lookPath:      func(name string) (string, error) { return "/usr/bin/" + name, nil },
		commandOutput: func(string, ...string) (string, error) { return "v4.0.0+g1234567", nil },
	})
	if err != nil {
		t.Fatalf("expected prerequisites to pass: %v", err)
	}
}

func TestCheckDeployPrerequisitesRejectsOldHelmAndMissingSOPS(t *testing.T) {
	err := checkDeployPrerequisitesWith(deployPrerequisiteRunner{
		lookPath: func(name string) (string, error) {
			if name == "sops" {
				return "", errors.New("not found")
			}
			return "/usr/bin/helm", nil
		},
		commandOutput: func(string, ...string) (string, error) { return "v3.19.0+g1234567", nil },
	})
	if err == nil || !strings.Contains(err.Error(), "Helm 4 is required") || !strings.Contains(err.Error(), "sops was not found") {
		t.Fatalf("expected Helm and SOPS failures, got %v", err)
	}
}

func TestCheckDeployPrerequisitesReportsHelmVersionFailure(t *testing.T) {
	err := checkDeployPrerequisitesWith(deployPrerequisiteRunner{
		lookPath:      func(string) (string, error) { return "/usr/bin/helm", nil },
		commandOutput: func(string, ...string) (string, error) { return "", errors.New("command failed") },
	})
	if err == nil || !strings.Contains(err.Error(), "could not determine Helm version") {
		t.Fatalf("expected Helm version failure, got %v", err)
	}
}
