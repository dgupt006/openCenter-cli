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

package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/validation"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/errors"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/fs"
)

// TestListSkipsPhantomBlueprintDirectories is a regression test for phantom
// clusters: destroy removes <cluster>-config.yaml but can leave the blueprint
// directory behind when sidecar files (e.g. <cluster>-config.yaml.backup from
// Save, or recovery JSON files) survive the removal. Such directories must not
// be listed, or every other command rejects them with "cluster configuration
// not found" while cluster list still shows them.
func TestListSkipsPhantomBlueprintDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := fs.NewDefaultFileSystem(errorHandler)
	pathResolver := paths.NewPathResolver(tmpDir)
	manager := NewConfigurationManagerWithDeps(
		v2.NewConfigIOHandler(fileSystem),
		validation.NewValidationEngine(),
		v2.NewConfigCache(),
		pathResolver,
		fileSystem,
	)

	// Real cluster: blueprint directory with its config file.
	realDir := filepath.Join(tmpDir, "blueprints", "snacks", "chips")
	if err := os.MkdirAll(realDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(realDir, "chips-config.yaml"), []byte("schema_version: \"2.0\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Phantom cluster: blueprint directory left behind after destroy removed
	// the config file, with only sidecar files remaining.
	phantomDir := filepath.Join(tmpDir, "blueprints", "snacks", "doritos")
	if err := os.MkdirAll(phantomDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(phantomDir, "doritos-config.yaml.backup"), []byte("schema_version: \"2.0\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	clusters, err := manager.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(clusters) != 1 || clusters[0] != "snacks/chips" {
		t.Fatalf("List() = %v, want [snacks/chips]", clusters)
	}

	// The phantom must be invisible to every existence check, matching Load.
	if _, err := manager.Load(ctx, "snacks/doritos"); err == nil {
		t.Fatal("Load() of phantom cluster error = nil, want not-found error")
	}
	if _, err := pathResolver.ResolveWithFallback(ctx, "doritos"); err == nil {
		t.Fatal("ResolveWithFallback() of phantom cluster error = nil, want not-found error")
	}
	org, err := pathResolver.GetOrganization(ctx, "doritos")
	if err != nil {
		t.Fatalf("GetOrganization() error = %v", err)
	}
	if org != "" {
		t.Fatalf("GetOrganization() = %q, want empty for phantom cluster", org)
	}
}
