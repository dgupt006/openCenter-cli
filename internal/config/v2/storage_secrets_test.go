package v2

import (
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
)

func TestStorageSecretContractsRoundTrip(t *testing.T) {
	cfg, err := NewV2Default("storage-contracts", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	cfg.OpenCenter.Services["tempo"] = &services.TempoConfig{StorageType: "s3", BucketName: "tempo"}
	cfg.OpenCenter.Services["etcd-backup"] = &services.EtcdBackupConfig{S3BucketName: "etcd"}
	cfg.OpenCenter.Services["velero"] = &services.VeleroConfig{StorageType: "s3", BackupBucket: "velero"}
	cfg.Secrets.Tempo = TempoSecrets{AccessKey: "tempo-access", SecretKey: "tempo-secret"}
	cfg.Secrets.EtcdBackup = EtcdBackupSecrets{AccessKeyID: "etcd-access", SecretAccessKey: "etcd-secret"}
	cfg.Secrets.Velero = VeleroSecrets{AccessKeyID: "velero-access", SecretAccessKey: "velero-secret"}
	data, err := MarshalPublicConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodePublicConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Secrets.Tempo.AccessKey != "tempo-access" || decoded.Secrets.Tempo.SecretKey != "tempo-secret" {
		t.Fatalf("Tempo secrets not canonical: %+v", decoded.Secrets.Tempo)
	}
	if decoded.Secrets.EtcdBackup.AccessKeyID != "etcd-access" || decoded.Secrets.EtcdBackup.SecretAccessKey != "etcd-secret" {
		t.Fatalf("etcd-backup secrets not round-tripped: %+v", decoded.Secrets.EtcdBackup)
	}
	if decoded.Secrets.Velero.AccessKeyID != "velero-access" || decoded.Secrets.Velero.SecretAccessKey != "velero-secret" {
		t.Fatalf("Velero secrets not round-tripped: %+v", decoded.Secrets.Velero)
	}
}
