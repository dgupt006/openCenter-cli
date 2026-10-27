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
	"fmt"
	"strings"
	"time"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

// UpdateStatus updates the cluster's stage and status in the configuration file.
// It loads the configuration, updates the values, and saves it back.
func UpdateStatus(clusterName, stage, status string) error {
	// Use ConfigurationManager for load/save
	mgr, err := NewConfigurationManager()
	if err != nil {
		return fmt.Errorf("failed to create configuration manager: %w", err)
	}

	ctx := context.Background()
	cfg, err := mgr.Load(ctx, clusterName)
	if err != nil {
		return fmt.Errorf("failed to load cluster configuration for status update: %w", err)
	}

	cfg.OpenCenter.Meta.Stage = stage
	cfg.OpenCenter.Meta.Status = status
	cfg.Metadata.UpdatedAt = time.Now().Format(time.RFC3339Nano)

	// Deployed = pinned. Once a cluster is bootstrapping, its DNS identity is
	// live infrastructure: a settings cluster_defaults.base_domain change must
	// never silently re-point a running cluster's FQDN, certificates, or
	// gateway hostnames. Pin the field so load-time hydration stops following
	// the settings file; a rename afterwards is an explicit operation.
	if stage == v2.StageBootstrap {
		cfg.OpenCenter.Cluster.Managed.PinBaseDomain()
	}

	if err := mgr.Save(context.Background(), cfg); err != nil {
		return fmt.Errorf("failed to save cluster configuration with new status: %w", err)
	}

	return nil
}

// StampGeneratedBaseDomain records the base_domain the most recent successful
// 'cluster generate' rendered with. The stamp lets a later generate tell a
// settings-file change (cluster value unchanged since generation) from a
// deliberate cluster-side domain edit, and is a no-op when the value already
// matches so steady-state generates never rewrite the config file.
func StampGeneratedBaseDomain(clusterName, baseDomain string) error {
	baseDomain = strings.ToLower(strings.TrimSpace(baseDomain))
	if baseDomain == "" {
		return nil
	}

	mgr, err := NewConfigurationManager()
	if err != nil {
		return fmt.Errorf("failed to create configuration manager: %w", err)
	}

	ctx := context.Background()
	cfg, err := mgr.Load(ctx, clusterName)
	if err != nil {
		return fmt.Errorf("failed to load cluster configuration for generate stamp: %w", err)
	}
	if strings.EqualFold(strings.TrimSpace(cfg.OpenCenter.Cluster.LastGeneratedBaseDomain), baseDomain) {
		return nil
	}

	cfg.OpenCenter.Cluster.LastGeneratedBaseDomain = baseDomain
	cfg.Metadata.UpdatedAt = time.Now().Format(time.RFC3339Nano)
	if err := mgr.Save(ctx, cfg); err != nil {
		return fmt.Errorf("failed to save cluster configuration with generate stamp: %w", err)
	}
	return nil
}
