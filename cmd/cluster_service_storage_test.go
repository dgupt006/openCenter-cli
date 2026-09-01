package cmd

import (
	"bytes"
	"strings"
	"testing"

	storageopenstack "github.com/opencenter-cloud/opencenter-cli/internal/cluster/storage/openstack"
	"github.com/spf13/cobra"
)

func TestClusterServiceStorageHierarchy(t *testing.T) {
	root := newClusterServiceCmd()
	var storageCmdFound bool
	for _, child := range root.Commands() {
		if child.Name() == "storage" {
			storageCmdFound = true
		}
	}
	if !storageCmdFound {
		t.Fatal("cluster service storage is not registered")
	}
	storage := newClusterServiceStorageCmd()
	if got := len(storage.Commands()); got != 2 {
		t.Fatalf("storage children=%d, want 2", got)
	}
	for _, operation := range storage.Commands() {
		if operation.Args == nil {
			t.Errorf("%s has no argument validator", operation.Name())
		}
		for _, name := range []string{"cluster", "backend", "os-cloud", "clouds-yaml", "container", "rotate-credentials"} {
			if operation.Flags().Lookup(name) == nil {
				t.Errorf("%s missing --%s", operation.Name(), name)
			}
		}
	}
}

func TestHarborStorageOutputRedactsCredentialIdentifiers(t *testing.T) {
	result := storageopenstack.Result{
		Service:       "harbor",
		RemoteActions: []storageopenstack.RemoteAction{{Order: 2, Action: "reuse", Resource: "keystone-ec2-credential", ID: "access-1"}},
		Recovery:      &storageopenstack.RecoveryState{},
	}
	cmd := &cobra.Command{}
	var text bytes.Buffer
	cmd.SetOut(&text)
	cmd.SetErr(&text)
	renderStorageReview(cmd, result)
	if strings.Contains(text.String(), "access-1") {
		t.Fatalf("interactive Harbor output exposed access key: %s", text.String())
	}

	text.Reset()
	cmd.SetOut(&text)
	if err := writeStorageOutput(cmd, OutputJSON, result); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(text.String(), "access-1") {
		t.Fatalf("structured Harbor output exposed access key: %s", text.String())
	}
}
