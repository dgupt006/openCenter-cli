package plugins

import (
	"context"
	"fmt"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	svc "github.com/opencenter-cloud/opencenter-cli/internal/services"
)

// TempoPlugin implements the ServicePlugin interface for Tempo
// using composition with BaseServicePlugin
type TempoPlugin struct {
	*svc.BaseServicePlugin
}

// NewTempoPlugin creates a new TempoPlugin
func NewTempoPlugin() svc.ServicePlugin {
	// Create base plugin with metadata
	base := svc.NewBasePlugin(svc.PluginMetadata{
		Name:        "tempo",
		Version:     "1.0.0",
		Description: "Tempo distributed tracing backend service",
		Type:        svc.ServiceTypeMonitoring,
		Author:      "opencenter",
		License:     "Apache-2.0",
	})

	plugin := &TempoPlugin{
		BaseServicePlugin: base,
	}

	// Inject service-specific validation logic
	base.SetValidator(plugin.validate)

	// Inject service-specific rendering logic
	base.SetRenderer(plugin.render)

	// Inject service-specific status logic
	base.SetStatusFunc(plugin.status)

	return plugin
}

// validate implements tempo specific validation
func (p *TempoPlugin) validate(config interface{}) error {
	cfg, ok := config.(*services.TempoConfig)
	if !ok {
		return fmt.Errorf("invalid config type for tempo: expected *TempoConfig")
	}

	// Basic validation
	if cfg.IsEnabled() && cfg.StorageType != "" {
		if cfg.StorageType != "s3" && cfg.StorageType != "swift" {
			return fmt.Errorf("storage_type must be 's3' or 'swift', got '%s'", cfg.StorageType)
		}

		// Swift is not supported by Tempo. Tempo's binary has no Swift storage
		// backend upstream (it supports only S3/S3-compatible, GCS, and Azure Blob)
		// and rejects it at startup with "unknown backend swift". Fail fast with a
		// clear message rather than rendering a config Tempo will crash on. The
		// swift_* fields and the enum value are retained for backward-compatibility
		// (no schema removal); use storage_type: s3 against the Swift S3-compatible
		// endpoint instead.
		if cfg.StorageType == "swift" {
			return fmt.Errorf("tempo storage_type 'swift' is not supported: Tempo has no Swift storage backend upstream (supports s3/s3-compatible, gcs, azure). Use storage_type: s3 (e.g. against the Swift S3-compatible endpoint)")
		}

		// S3-specific validation
		if cfg.StorageType == "s3" {
			if cfg.S3Endpoint == "" {
				return fmt.Errorf("s3_endpoint is required when storage_type is 's3'")
			}
			if cfg.BucketName == "" {
				return fmt.Errorf("bucket_name is required when storage_type is 's3'")
			}
		}
	}

	return nil
}

// render implements tempo specific rendering
func (p *TempoPlugin) render(ctx context.Context, config interface{}, workspace interface{}) error {
	// Template rendering will be handled by the template system
	// This is a placeholder for future implementation
	return nil
}

// status implements tempo specific status logic
func (p *TempoPlugin) status(config interface{}) svc.ServiceStatus {
	cfg, ok := config.(*services.TempoConfig)
	if !ok {
		return svc.ServiceStatus{
			State:   "failed",
			Message: "Invalid configuration type",
		}
	}

	if !cfg.IsEnabled() {
		return svc.ServiceStatus{
			State:   "disabled",
			Message: "Service is disabled",
		}
	}

	// Get status from config, default to "pending" if not set
	state := cfg.GetStatus()
	if state == "" {
		state = "pending"
	}

	return svc.ServiceStatus{
		State:   state,
		Message: "Tempo distributed tracing service",
		Details: map[string]interface{}{
			"storage_type": cfg.StorageType,
			"bucket_name":  cfg.BucketName,
			"volume_size":  cfg.VolumeSize,
		},
	}
}
