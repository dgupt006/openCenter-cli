/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package secrets

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeManifestForScanTest(t *testing.T, overlayPath, relativePath string) string {
	t.Helper()

	manifestPath := filepath.Join(overlayPath, filepath.FromSlash(relativePath))
	require.NoError(t, os.MkdirAll(filepath.Dir(manifestPath), 0o755))
	require.NoError(t, os.WriteFile(manifestPath, []byte("apiVersion: v1\n"), 0o644))
	return manifestPath
}

func TestFindManifestFilesManagedServicesOnly(t *testing.T) {
	overlayPath := t.TempDir()
	managedManifest := writeManifestForScanTest(t, overlayPath, "managed-services/cert-manager/secret.yaml")
	manager := &DefaultSecretsManager{}

	manifestFiles, err := manager.findManifestFiles(overlayPath)

	require.NoError(t, err)
	assert.Equal(t, []string{managedManifest}, manifestFiles)
}

func TestFindManifestFilesScansBothRootsInSortedOrder(t *testing.T) {
	overlayPath := t.TempDir()
	servicesManifest := writeManifestForScanTest(t, overlayPath, "services/zookeeper/secret.yaml")
	managedManifest := writeManifestForScanTest(t, overlayPath, "managed-services/alertmanager/secret.yaml")
	manager := &DefaultSecretsManager{}

	manifestFiles, err := manager.findManifestFiles(overlayPath)

	require.NoError(t, err)
	want := []string{servicesManifest, managedManifest}
	sort.Strings(want)
	assert.Equal(t, want, manifestFiles)
}

func TestFindManifestFilesPropagatesErrors(t *testing.T) {
	t.Run("overlay stat error", func(t *testing.T) {
		tmpDir := t.TempDir()
		nonDirectory := filepath.Join(tmpDir, "not-a-directory")
		require.NoError(t, os.WriteFile(nonDirectory, []byte("file"), 0o644))

		_, err := (&DefaultSecretsManager{}).findManifestFiles(filepath.Join(nonDirectory, "overlay"))

		require.Error(t, err)
		assert.Contains(t, err.Error(), "stat overlay directory")
	})

	t.Run("manifest root is not silently skipped", func(t *testing.T) {
		overlayPath := t.TempDir()
		servicesPath := filepath.Join(overlayPath, "services")
		require.NoError(t, os.WriteFile(servicesPath, []byte("not a directory"), 0o644))

		_, err := (&DefaultSecretsManager{}).findManifestFiles(overlayPath)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "manifest root is not a directory")
	})
}

func TestFindManifestFilesRefusesSymlinks(t *testing.T) {
	tests := []struct {
		name string
		call func(t *testing.T, overlayPath string) ([]string, error)
	}{
		{
			name: "overlay root",
			call: func(t *testing.T, overlayPath string) ([]string, error) {
				target := t.TempDir()
				link := filepath.Join(overlayPath, "overlay-link")
				if err := os.Symlink(target, link); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
				return (&DefaultSecretsManager{}).findManifestFiles(link)
			},
		},
		{
			name: "manifest root",
			call: func(t *testing.T, overlayPath string) ([]string, error) {
				target := filepath.Join(overlayPath, "managed-target")
				require.NoError(t, os.MkdirAll(target, 0o755))
				if err := os.Symlink(target, filepath.Join(overlayPath, "services")); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
				return (&DefaultSecretsManager{}).findManifestFiles(overlayPath)
			},
		},
		{
			name: "nested directory",
			call: func(t *testing.T, overlayPath string) ([]string, error) {
				servicesPath := filepath.Join(overlayPath, "services")
				require.NoError(t, os.MkdirAll(servicesPath, 0o755))
				target := filepath.Join(overlayPath, "outside")
				require.NoError(t, os.MkdirAll(target, 0o755))
				if err := os.Symlink(target, filepath.Join(servicesPath, "linked-service")); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
				return (&DefaultSecretsManager{}).findManifestFiles(overlayPath)
			},
		},
		{
			name: "manifest file",
			call: func(t *testing.T, overlayPath string) ([]string, error) {
				servicesPath := filepath.Join(overlayPath, "services", "service")
				require.NoError(t, os.MkdirAll(servicesPath, 0o755))
				target := filepath.Join(overlayPath, "outside-secret.yaml")
				require.NoError(t, os.WriteFile(target, []byte("secret"), 0o644))
				if err := os.Symlink(target, filepath.Join(servicesPath, "secret.yaml")); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
				return (&DefaultSecretsManager{}).findManifestFiles(overlayPath)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			overlayPath := t.TempDir()
			_, err := test.call(t, overlayPath)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "symlink")
		})
	}
}
