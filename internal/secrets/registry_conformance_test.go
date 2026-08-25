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
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runKeyRegistryConformance(t *testing.T, name string, newRegistry func(t *testing.T) KeyRegistry) {
	t.Helper()
	ctx := context.Background()

	t.Run(name+"/registers_metadata_and_defaults", func(t *testing.T) {
		registry := newRegistry(t)
		entry := KeyEntry{Cluster: "cluster-a", KeyType: KeyTypeAge, Fingerprint: "age-a", PublicKey: "age-a"}
		require.NoError(t, registry.RegisterKey(ctx, entry))
		keys, err := registry.ListKeys(ctx, "cluster-a")
		require.NoError(t, err)
		require.Len(t, keys, 1)
		assert.False(t, keys[0].CreatedAt.IsZero())
		assert.False(t, keys[0].ExpiresAt.IsZero())
		assert.Equal(t, KeyStatusActive, keys[0].Status)
	})

	t.Run(name+"/rejects_duplicate_fingerprint", func(t *testing.T) {
		registry := newRegistry(t)
		entry := KeyEntry{Cluster: "cluster-a", KeyType: KeyTypeAge, Fingerprint: "age-a"}
		require.NoError(t, registry.RegisterKey(ctx, entry))
		err := registry.RegisterKey(ctx, entry)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
	})

	t.Run(name+"/rejects_archived_fingerprint", func(t *testing.T) {
		registry := newRegistry(t)
		entry := KeyEntry{Cluster: "cluster-a", KeyType: KeyTypeAge, Fingerprint: "age-a"}
		require.NoError(t, registry.RegisterKey(ctx, entry))
		require.NoError(t, registry.RegisterKey(ctx, KeyEntry{Cluster: "envelope-keeper", KeyType: KeyTypeAge, Fingerprint: "age-keeper", PublicKey: "age-keeper"}))
		entry.Status = KeyStatusArchived
		require.NoError(t, registry.UpdateKey(ctx, entry))
		entry.Status = KeyStatusActive
		err := registry.RegisterKey(ctx, entry)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
	})

	t.Run(name+"/allows_multiple_active_age_keys", func(t *testing.T) {
		registry := newRegistry(t)
		require.NoError(t, registry.RegisterKey(ctx, KeyEntry{Cluster: "cluster-a", KeyType: KeyTypeAge, Fingerprint: "age-a"}))
		require.NoError(t, registry.RegisterKey(ctx, KeyEntry{Cluster: "cluster-a", KeyType: KeyTypeAge, Fingerprint: "age-b"}))
		keys, err := registry.ListKeys(ctx, "cluster-a")
		require.NoError(t, err)
		activeAge := 0
		for _, key := range keys {
			if key.KeyType == KeyTypeAge && key.Status == KeyStatusActive {
				activeAge++
			}
		}
		assert.Equal(t, 2, activeAge)
	})

	t.Run(name+"/explicit_primary_and_lifecycle_invariants", func(t *testing.T) {
		registry := newRegistry(t)
		require.NoError(t, registry.RegisterKey(ctx, KeyEntry{Cluster: "cluster-a", KeyType: KeyTypeAge, Fingerprint: "age-a"}))
		require.NoError(t, registry.RegisterKey(ctx, KeyEntry{Cluster: "cluster-a", KeyType: KeyTypeAge, Fingerprint: "age-b"}))
		_, err := registry.GetPrimaryKey(ctx, "cluster-a", KeyTypeAge)
		require.Error(t, err)
		require.NoError(t, registry.SetPrimaryKey(ctx, "cluster-a", KeyTypeAge, "age-b"))
		require.NoError(t, registry.SetPrimaryKey(ctx, "cluster-a", KeyTypeAge, "age-b"))
		primary, err := registry.GetPrimaryKey(ctx, "cluster-a", KeyTypeAge)
		require.NoError(t, err)
		assert.Equal(t, "age-b", primary.Fingerprint)
		keys, err := registry.ListKeys(ctx, "cluster-a")
		require.NoError(t, err)
		for _, key := range keys {
			if key.KeyType == KeyTypeAge {
				assert.True(t, key.Primary == (key.Fingerprint == "age-b"))
			}
		}
		require.NoError(t, registry.RegisterKey(ctx, KeyEntry{Cluster: "cluster-a", KeyType: KeyTypeSSH, Fingerprint: "ssh-a"}))
		_, err = registry.GetPrimaryKey(ctx, "cluster-a", KeyTypeSSH)
		assert.Error(t, err)
		err = registry.RegisterKey(ctx, KeyEntry{Cluster: "cluster-a", KeyType: KeyTypeAge, Fingerprint: "age-inactive", Status: KeyStatusArchived, Primary: true})
		assert.Error(t, err)
		err = registry.UpdateKeyStatus(ctx, "cluster-a", KeyTypeAge, KeyStatusArchived)
		assert.Error(t, err)
		newEntry := KeyEntry{Cluster: "cluster-a", KeyType: KeyTypeAge, Fingerprint: "age-c", Status: KeyStatusActive}
		require.NoError(t, registry.ReplacePrimary(ctx, "age-b", newEntry))
		primary, err = registry.GetPrimaryKey(ctx, "cluster-a", KeyTypeAge)
		require.NoError(t, err)
		assert.Equal(t, "age-c", primary.Fingerprint)
	})

	t.Run(name+"/unknown_cluster_is_not_found", func(t *testing.T) {
		registry := newRegistry(t)
		err := func() error {
			_, err := registry.GetKey(ctx, "unknown", KeyTypeAge)
			return err
		}()
		require.Error(t, err)
		assert.True(t, IsKeyNotFoundError(err))
	})

	t.Run(name+"/wrong_key_type_is_not_found", func(t *testing.T) {
		registry := newRegistry(t)
		require.NoError(t, registry.RegisterKey(ctx, KeyEntry{Cluster: "cluster-a", KeyType: KeyTypeAge, Fingerprint: "age-a"}))
		_, err := registry.GetKey(ctx, "cluster-a", KeyTypeSSH)
		require.Error(t, err)
		assert.True(t, IsKeyNotFoundError(err))
	})

	t.Run(name+"/updates_exact_fingerprint", func(t *testing.T) {
		registry := newRegistry(t)
		first := KeyEntry{Cluster: "cluster-a", KeyType: KeyTypeAge, Fingerprint: "age-a"}
		second := KeyEntry{Cluster: "cluster-a", KeyType: KeyTypeAge, Fingerprint: "age-b"}
		require.NoError(t, registry.RegisterKey(ctx, first))
		require.NoError(t, registry.RegisterKey(ctx, second))
		second.Status = KeyStatusRevoked
		require.NoError(t, registry.UpdateKey(ctx, second))
		keys, err := registry.ListKeys(ctx, "cluster-a")
		require.NoError(t, err)
		require.Len(t, keys, 2)
		for _, key := range keys {
			switch key.Fingerprint {
			case "age-a":
				assert.Equal(t, KeyStatusActive, key.Status)
			case "age-b":
				assert.Equal(t, KeyStatusRevoked, key.Status)
			}
		}
	})

	t.Run(name+"/unknown_update_is_not_found", func(t *testing.T) {
		registry := newRegistry(t)
		err := registry.UpdateKey(ctx, KeyEntry{Cluster: "cluster-a", KeyType: KeyTypeAge, Fingerprint: "missing", Status: KeyStatusRevoked})
		require.Error(t, err)
		assert.True(t, IsKeyNotFoundError(err))
	})

	t.Run(name+"/list_filters_by_cluster", func(t *testing.T) {
		registry := newRegistry(t)
		require.NoError(t, registry.RegisterKey(ctx, KeyEntry{Cluster: "cluster-a", KeyType: KeyTypeAge, Fingerprint: "age-a"}))
		require.NoError(t, registry.RegisterKey(ctx, KeyEntry{Cluster: "cluster-b", KeyType: KeyTypeAge, Fingerprint: "age-b"}))
		all, err := registry.ListKeys(ctx, "")
		require.NoError(t, err)
		assert.Len(t, all, 2)
		filtered, err := registry.ListKeys(ctx, "cluster-a")
		require.NoError(t, err)
		require.Len(t, filtered, 1)
		assert.Equal(t, "cluster-a", filtered[0].Cluster)
	})
}

func TestKeyRegistryConformance_Default(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	runKeyRegistryConformance(t, "default", func(t *testing.T) KeyRegistry {
		return NewDefaultKeyRegistry(t.TempDir(), newMockSOPSEncryptor(), logger)
	})
}

func TestKeyRegistryConformance_Mock(t *testing.T) {
	runKeyRegistryConformance(t, "mock", func(t *testing.T) KeyRegistry {
		return NewMockKeyRegistry()
	})
}
