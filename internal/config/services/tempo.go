package services

import "github.com/opencenter-cloud/opencenter-cli/internal/config/registry"

// TempoConfig extends BaseConfig with Tempo-specific configuration.
type TempoConfig struct {
	BaseConfig `yaml:",inline"`

	// Storage
	// NOTE: 'swift' is NOT supported by Tempo — its binary has no Swift storage
	// backend upstream and rejects it at startup ("unknown backend swift"). The enum
	// value is retained only for backward compatibility; configuration validation
	// rejects storage_type: swift. Use 's3' (e.g. against the Swift S3-compatible endpoint).
	StorageType  string `yaml:"storage_type,omitempty" json:"storage_type,omitempty" jsonschema:"description=Tempo storage backend type (use s3; swift is unsupported by Tempo and rejected at validation),enum=s3,enum=swift,default=s3"`
	BucketName   string `yaml:"bucket_name,omitempty" json:"bucket_name,omitempty" jsonschema:"description=Storage bucket/container name"`
	VolumeSize   int    `yaml:"volume_size,omitempty" json:"volume_size,omitempty" jsonschema:"description=Persistent volume size in GB"`
	StorageClass string `yaml:"storage_class,omitempty" json:"storage_class,omitempty" jsonschema:"description=Storage class for PVCs"`

	// S3 backend
	S3Endpoint       string `yaml:"s3_endpoint,omitempty" json:"s3_endpoint,omitempty" jsonschema:"description=S3 endpoint URL"`
	S3Region         string `yaml:"s3_region,omitempty" json:"s3_region,omitempty" jsonschema:"description=S3 region"`
	S3CredentialID   string `yaml:"s3_credential_id,omitempty" json:"s3_credential_id,omitempty" jsonschema:"description=OpenStack EC2 credential ID"`
	S3ForcePathStyle bool   `yaml:"s3_force_path_style,omitempty" json:"s3_force_path_style,omitempty" jsonschema:"description=Force S3 path style"`
	S3Insecure       bool   `yaml:"s3_insecure,omitempty" json:"s3_insecure,omitempty" jsonschema:"description=Allow insecure S3 connections"`

	// Swift backend
	// Deprecated: Tempo has no Swift storage backend upstream. These fields are
	// retained for backward compatibility but are non-functional; storage_type: swift
	// is rejected at validation. Use the S3 backend instead.
	SwiftAuthURL                 string `yaml:"swift_auth_url,omitempty" json:"swift_auth_url,omitempty" jsonschema:"description=Deprecated (Tempo has no Swift backend): Swift Keystone V3 authentication URL"`
	SwiftRegion                  string `yaml:"swift_region,omitempty" json:"swift_region,omitempty" jsonschema:"description=Deprecated (Tempo has no Swift backend): Swift region name"`
	SwiftAuthVersion             int    `yaml:"swift_auth_version,omitempty" json:"swift_auth_version,omitempty" jsonschema:"description=Deprecated (Tempo has no Swift backend): Swift authentication version,default=3"`
	SwiftApplicationCredentialID string `yaml:"swift_application_credential_id,omitempty" json:"swift_application_credential_id,omitempty" jsonschema:"description=Deprecated (Tempo has no Swift backend): Swift application credential ID (UUID)"`
	SwiftContainerName           string `yaml:"swift_container_name,omitempty" json:"swift_container_name,omitempty" jsonschema:"description=Deprecated (Tempo has no Swift backend): Swift container name for Tempo traces"`
	SwiftUserDomainName          string `yaml:"swift_user_domain_name,omitempty" json:"swift_user_domain_name,omitempty" jsonschema:"description=Deprecated (Tempo has no Swift backend): Swift user domain name"`
	SwiftDomainName              string `yaml:"swift_domain_name,omitempty" json:"swift_domain_name,omitempty" jsonschema:"description=Deprecated (Tempo has no Swift backend): Swift domain name"`
}

func init() {
	registry.RegisterServiceConfig("tempo", TempoConfig{})
}
