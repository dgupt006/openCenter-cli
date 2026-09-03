package v2

import (
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
)

type enabledNonCanonicalEtcdBackup struct{}

func (enabledNonCanonicalEtcdBackup) IsEnabled() bool { return true }

func TestValidateReadinessEtcdBackupRejectsNonCanonicalConfig(t *testing.T) {
	cfg := validReadinessConfig(t, "openstack")
	cfg.OpenCenter.Services["etcd-backup"] = enabledNonCanonicalEtcdBackup{}

	report := ValidateReadiness(cfg)
	assertIssue(t, report, SeverityError, CategoryServices, "opencenter.services.etcd-backup")
}

func TestValidateReadinessEtcdBackupDisabledSkipsIncompleteConfig(t *testing.T) {
	cfg := validReadinessConfig(t, "openstack")
	cfg.OpenCenter.Services["etcd-backup"] = &services.EtcdBackupConfig{}

	report := ValidateReadiness(cfg)
	assertNoIssue(t, report, "opencenter.services.etcd-backup.s3_endpoint")
	assertNoIssue(t, report, "secrets.etcd_backup.access_key_id")
	assertNoIssue(t, report, "secrets.etcd_backup.secret_access_key")
}

func TestValidateReadinessEtcdBackupIncompleteConfig(t *testing.T) {
	cfg := validReadinessConfig(t, "openstack")
	cfg.OpenCenter.Services["etcd-backup"] = &services.EtcdBackupConfig{
		BaseConfig: services.BaseConfig{Enabled: true},
	}

	report := ValidateReadiness(cfg)
	assertIssue(t, report, SeverityError, CategoryServices, "opencenter.services.etcd-backup.s3_endpoint")
	assertIssue(t, report, SeverityError, CategoryServices, "opencenter.services.etcd-backup.s3_bucket_name")
	assertIssue(t, report, SeverityError, CategoryServices, "opencenter.services.etcd-backup.s3_region")
	assertIssue(t, report, SeverityError, CategoryServices, "secrets.etcd_backup.access_key_id")
	assertIssue(t, report, SeverityError, CategoryServices, "secrets.etcd_backup.secret_access_key")
}

func TestValidateReadinessEtcdBackupCompleteConfig(t *testing.T) {
	cfg := validReadinessConfig(t, "openstack")
	cfg.OpenCenter.Services["etcd-backup"] = &services.EtcdBackupConfig{
		BaseConfig:   services.BaseConfig{Enabled: true},
		S3Endpoint:   "https://s3.example/v1",
		S3BucketName: "etcd-backups",
		S3Region:     "RegionOne",
	}
	cfg.Secrets.EtcdBackup = EtcdBackupSecrets{AccessKeyID: "access-key", SecretAccessKey: "secret-key"}

	report := ValidateReadiness(cfg)
	if !report.Valid {
		t.Fatalf("expected complete etcd-backup configuration to be ready, got:\n%s", renderIssues(report.Issues))
	}
	assertNoIssue(t, report, "opencenter.services.etcd-backup.s3_endpoint")
	assertNoIssue(t, report, "secrets.etcd_backup.access_key_id")
}

func TestNewV2DefaultDisablesEtcdBackup(t *testing.T) {
	cfg, err := NewV2Default("etcd-default-disabled", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	service, ok := cfg.OpenCenter.Services["etcd-backup"].(*services.EtcdBackupConfig)
	if !ok {
		t.Fatalf("etcd-backup default has type %T", cfg.OpenCenter.Services["etcd-backup"])
	}
	if service.Enabled {
		t.Fatal("etcd-backup must be disabled by default without a portable S3 destination")
	}
}
