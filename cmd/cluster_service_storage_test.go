package cmd

import "testing"

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
