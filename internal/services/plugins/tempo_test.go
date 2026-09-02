package plugins

import (
	"context"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	svc "github.com/opencenter-cloud/opencenter-cli/internal/services"
)

func TestTempoPlugin(t *testing.T) {
	t.Run("plugin metadata", func(t *testing.T) {
		plugin := NewTempoPlugin()

		if plugin.Name() != "tempo" {
			t.Errorf("expected name 'tempo', got '%s'", plugin.Name())
		}

		if plugin.Type() != svc.ServiceTypeMonitoring {
			t.Errorf("expected type 'monitoring', got '%s'", plugin.Type())
		}
	})

	t.Run("validation - valid s3 config", func(t *testing.T) {
		plugin := NewTempoPlugin()
		cfg := &services.TempoConfig{
			BaseConfig: services.BaseConfig{
				Enabled: true,
			},
			StorageType: "s3",
			S3Endpoint:  "https://s3.example.com",
			BucketName:  "tempo-traces",
		}

		err := plugin.Validate(cfg)
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
	})

	t.Run("validation - swift is rejected (unsupported by Tempo)", func(t *testing.T) {
		plugin := NewTempoPlugin()
		cfg := &services.TempoConfig{
			BaseConfig: services.BaseConfig{
				Enabled: true,
			},
			StorageType:        "swift",
			SwiftAuthURL:       "https://keystone.example.com/v3",
			SwiftContainerName: "tempo-traces",
		}

		err := plugin.Validate(cfg)
		if err == nil {
			t.Error("expected error: Tempo does not support storage_type 'swift', got nil")
		}
	})

	t.Run("validation - invalid storage type", func(t *testing.T) {
		plugin := NewTempoPlugin()
		cfg := &services.TempoConfig{
			BaseConfig: services.BaseConfig{
				Enabled: true,
			},
			StorageType: "invalid",
		}

		err := plugin.Validate(cfg)
		if err == nil {
			t.Error("expected error for invalid storage type, got nil")
		}
	})

	t.Run("validation - missing s3 endpoint", func(t *testing.T) {
		plugin := NewTempoPlugin()
		cfg := &services.TempoConfig{
			BaseConfig: services.BaseConfig{
				Enabled: true,
			},
			StorageType: "s3",
			BucketName:  "tempo-traces",
		}

		err := plugin.Validate(cfg)
		if err == nil {
			t.Error("expected error for missing s3_endpoint, got nil")
		}
	})

	t.Run("validation - missing s3 bucket name", func(t *testing.T) {
		plugin := NewTempoPlugin()
		cfg := &services.TempoConfig{
			BaseConfig: services.BaseConfig{
				Enabled: true,
			},
			StorageType: "s3",
			S3Endpoint:  "https://s3.example.com",
		}

		err := plugin.Validate(cfg)
		if err == nil {
			t.Error("expected error for missing bucket_name, got nil")
		}
	})

	t.Run("validation - swift rejected even with partial fields set", func(t *testing.T) {
		plugin := NewTempoPlugin()
		cfg := &services.TempoConfig{
			BaseConfig: services.BaseConfig{
				Enabled: true,
			},
			StorageType:        "swift",
			SwiftContainerName: "tempo-traces",
		}

		err := plugin.Validate(cfg)
		if err == nil {
			t.Error("expected error: swift unsupported by Tempo, got nil")
		}
	})

	t.Run("validation - invalid config type", func(t *testing.T) {
		plugin := NewTempoPlugin()
		cfg := &services.LokiConfig{} // Wrong config type

		err := plugin.Validate(cfg)
		if err == nil {
			t.Error("expected error for invalid config type, got nil")
		}
	})

	t.Run("render - placeholder implementation", func(t *testing.T) {
		plugin := NewTempoPlugin()
		cfg := &services.TempoConfig{
			BaseConfig: services.BaseConfig{
				Enabled: true,
			},
			StorageType: "s3",
			S3Endpoint:  "https://s3.example.com",
			BucketName:  "tempo-traces",
		}

		ctx := context.Background()
		err := plugin.Render(ctx, cfg, nil)
		if err != nil {
			t.Errorf("expected no error from render, got: %v", err)
		}
	})

	t.Run("status - enabled service", func(t *testing.T) {
		plugin := NewTempoPlugin()
		cfg := &services.TempoConfig{
			BaseConfig: services.BaseConfig{
				Enabled: true,
			},
			StorageType: "s3",
			BucketName:  "tempo-traces",
			VolumeSize:  100,
		}

		status := plugin.Status(cfg)
		if status.State != "pending" {
			t.Errorf("expected state 'pending', got '%s'", status.State)
		}

		if status.Message != "Tempo distributed tracing service" {
			t.Errorf("expected message 'Tempo distributed tracing service', got '%s'", status.Message)
		}

		// Check details
		if details, ok := status.Details["storage_type"].(string); !ok || details != "s3" {
			t.Errorf("expected storage_type 's3', got '%v'", status.Details["storage_type"])
		}

		if details, ok := status.Details["bucket_name"].(string); !ok || details != "tempo-traces" {
			t.Errorf("expected bucket_name 'tempo-traces', got '%v'", status.Details["bucket_name"])
		}

		if details, ok := status.Details["volume_size"].(int); !ok || details != 100 {
			t.Errorf("expected volume_size 100, got '%v'", status.Details["volume_size"])
		}
	})

	t.Run("status - disabled service", func(t *testing.T) {
		plugin := NewTempoPlugin()
		cfg := &services.TempoConfig{
			BaseConfig: services.BaseConfig{
				Enabled: false,
			},
		}

		status := plugin.Status(cfg)
		if status.State != "disabled" {
			t.Errorf("expected state 'disabled', got '%s'", status.State)
		}

		if status.Message != "Service is disabled" {
			t.Errorf("expected message 'Service is disabled', got '%s'", status.Message)
		}
	})

	t.Run("status - invalid config type", func(t *testing.T) {
		plugin := NewTempoPlugin()
		cfg := &services.LokiConfig{} // Wrong config type

		status := plugin.Status(cfg)
		if status.State != "failed" {
			t.Errorf("expected state 'failed', got '%s'", status.State)
		}

		if status.Message != "Invalid configuration type" {
			t.Errorf("expected message 'Invalid configuration type', got '%s'", status.Message)
		}
	})
}
