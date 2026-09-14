package services

import "github.com/opencenter-cloud/opencenter-cli/internal/config/registry"

// MimirConfig defines the S3-compatible blocks storage contract for Mimir.
// The selected storage profile determines whether these values point to an
// externally managed endpoint or the internal RustFS service.
type MimirConfig struct {
	BaseConfig `yaml:",inline"`

	S3Endpoint       string `yaml:"s3_endpoint,omitempty" json:"s3_endpoint,omitempty" jsonschema:"description=S3-compatible endpoint URL for Mimir blocks storage"`
	S3Region         string `yaml:"s3_region,omitempty" json:"s3_region,omitempty" jsonschema:"description=S3 region for Mimir blocks storage"`
	S3BucketName     string `yaml:"s3_bucket_name,omitempty" json:"s3_bucket_name,omitempty" jsonschema:"description=S3 bucket name for Mimir blocks storage"`
	S3ForcePathStyle bool   `yaml:"s3_force_path_style,omitempty" json:"s3_force_path_style,omitempty" jsonschema:"description=Force S3 path-style addressing"`
	S3Insecure       bool   `yaml:"s3_insecure,omitempty" json:"s3_insecure,omitempty" jsonschema:"description=Allow insecure HTTP S3 connections"`
}

func init() {
	registry.RegisterServiceConfig("mimir", MimirConfig{})
}
