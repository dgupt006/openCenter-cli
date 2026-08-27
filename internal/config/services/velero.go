package services

import (
	"github.com/opencenter-cloud/opencenter-cli/internal/config/registry"
)

// VeleroConfig extends BaseConfig with Velero-specific configuration.
type VeleroConfig struct {
	BaseConfig `yaml:",inline"`

	BackupBucket     string `yaml:"backup_bucket,omitempty" json:"backup_bucket,omitempty" jsonschema:"description=Velero backup bucket name"`
	Region           string `yaml:"region,omitempty" json:"region,omitempty" jsonschema:"description=Velero backup region"`
	S3Endpoint       string `yaml:"s3_endpoint,omitempty" json:"s3_endpoint,omitempty" jsonschema:"description=S3-compatible endpoint URL"`
	S3Region         string `yaml:"s3_region,omitempty" json:"s3_region,omitempty" jsonschema:"description=S3 region"`
	S3CredentialID   string `yaml:"s3_credential_id,omitempty" json:"s3_credential_id,omitempty" jsonschema:"description=OpenStack EC2 credential ID"`
	S3ForcePathStyle bool   `yaml:"s3_force_path_style,omitempty" json:"s3_force_path_style,omitempty" jsonschema:"description=Force S3 path style"`
	S3Insecure       bool   `yaml:"s3_insecure,omitempty" json:"s3_insecure,omitempty" jsonschema:"description=Allow insecure S3 connections"`
	StorageType      string `yaml:"storage_type,omitempty" json:"storage_type,omitempty" jsonschema:"description=Velero storage backend type,enum=s3,enum=swift,enum=gcs,enum=azure,default=s3"`
}

func init() {
	registry.RegisterServiceConfig("velero", VeleroConfig{})
}
