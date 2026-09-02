package services

import "github.com/opencenter-cloud/opencenter-cli/internal/config/registry"

// LokiConfig extends BaseConfig with Loki-specific configuration.
type LokiConfig struct {
	BaseConfig `yaml:",inline"`

	// Storage
	StorageType  string `yaml:"storage_type,omitempty" json:"storage_type,omitempty" jsonschema:"description=Loki storage backend type,enum=s3,enum=swift,default=swift"`
	BucketName   string `yaml:"bucket_name,omitempty" json:"bucket_name,omitempty" jsonschema:"description=Storage bucket/container name"`
	VolumeSize   int    `yaml:"volume_size,omitempty" json:"volume_size,omitempty" jsonschema:"description=Persistent volume size in GB"`
	StorageClass string `yaml:"storage_class,omitempty" json:"storage_class,omitempty" jsonschema:"description=Storage class for PVCs"`

	// Swift backend
	// Loki's chart Swift driver (openstack.SwiftConfig) authenticates with a Keystone
	// service-user username/password, not application credentials. Use SwiftUsername +
	// the swift_password secret. SwiftApplicationCredentialID is retained for backward
	// compatibility but is not honored by Loki's Swift driver.
	SwiftAuthURL             string `yaml:"swift_auth_url,omitempty" json:"swift_auth_url,omitempty" jsonschema:"description=Swift Keystone V3 authentication URL (must end in /v3)"`
	SwiftRegion              string `yaml:"swift_region,omitempty" json:"swift_region,omitempty" jsonschema:"description=Swift region name"`
	SwiftAuthVersion         int    `yaml:"swift_auth_version,omitempty" json:"swift_auth_version,omitempty" jsonschema:"description=Swift authentication version,default=3"`
	SwiftUsername            string `yaml:"swift_username,omitempty" json:"swift_username,omitempty" jsonschema:"description=Swift Keystone username (service user) for username/password auth"`
	SwiftProjectName         string `yaml:"swift_project_name,omitempty" json:"swift_project_name,omitempty" jsonschema:"description=Swift Keystone project name"`
	SwiftProjectDomainName   string `yaml:"swift_project_domain_name,omitempty" json:"swift_project_domain_name,omitempty" jsonschema:"description=Swift Keystone project domain name (defaults to swift_domain_name)"`
	SwiftContainerName       string `yaml:"swift_container_name,omitempty" json:"swift_container_name,omitempty" jsonschema:"description=Swift container name for Loki logs"`
	SwiftUserDomainName      string `yaml:"swift_user_domain_name,omitempty" json:"swift_user_domain_name,omitempty" jsonschema:"description=Swift user domain name"`
	SwiftDomainName          string `yaml:"swift_domain_name,omitempty" json:"swift_domain_name,omitempty" jsonschema:"description=Swift domain name"`
	SwiftApplicationCredentialID string `yaml:"swift_application_credential_id,omitempty" json:"swift_application_credential_id,omitempty" jsonschema:"description=Deprecated: not honored by Loki's Swift driver (use swift_username/swift_password). Retained for backward compatibility."`

	// S3 backend
	S3Endpoint       string `yaml:"s3_endpoint,omitempty" json:"s3_endpoint,omitempty" jsonschema:"description=S3 endpoint URL"`
	S3Region         string `yaml:"s3_region,omitempty" json:"s3_region,omitempty" jsonschema:"description=S3 region"`
	S3CredentialID   string `yaml:"s3_credential_id,omitempty" json:"s3_credential_id,omitempty" jsonschema:"description=OpenStack EC2 credential ID"`
	S3ForcePathStyle bool   `yaml:"s3_force_path_style,omitempty" json:"s3_force_path_style,omitempty" jsonschema:"description=Force S3 path style"`
	S3Insecure       bool   `yaml:"s3_insecure,omitempty" json:"s3_insecure,omitempty" jsonschema:"description=Allow insecure S3 connections"`
}

func init() {
	registry.RegisterServiceConfig("loki", LokiConfig{})
}
