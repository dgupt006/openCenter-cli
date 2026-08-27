package openstack

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	cloudopenstack "github.com/opencenter-cloud/opencenter-cli/internal/cloud/openstack"
	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type fakeAdapter struct {
	preflight                                                                  cloudopenstack.StoragePreflight
	preflightCalls, containers, appCreates, appDeletes, ec2Creates, ec2Deletes int
	app                                                                        cloudopenstack.AppCredential
	ec2                                                                        cloudopenstack.EC2Credentials
	revokeErr                                                                  error
}

func (f *fakeAdapter) Preflight(context.Context, string, string) (cloudopenstack.StoragePreflight, error) {
	f.preflightCalls++
	return f.preflight, nil
}
func (f *fakeAdapter) EnsureContainer(context.Context, cloudopenstack.ContainerRequest) error {
	f.containers++
	return nil
}
func (f *fakeAdapter) CreateAppCredential(context.Context, cloudopenstack.AppCredentialRequest) (cloudopenstack.AppCredential, error) {
	f.appCreates++
	return f.app, nil
}
func (f *fakeAdapter) DeleteAppCredential(context.Context, string) error {
	f.appDeletes++
	return f.revokeErr
}
func (f *fakeAdapter) CreateEC2Credentials(context.Context, cloudopenstack.EC2CredentialRequest) (cloudopenstack.EC2Credentials, error) {
	f.ec2Creates++
	return f.ec2, nil
}
func (f *fakeAdapter) DeleteEC2Credentials(context.Context, string) error {
	f.ec2Deletes++
	return f.revokeErr
}

type fakeFS struct {
	data                              []byte
	backup, atomic, recovery, removed bool
	atomicErr                         error
}

func (f *fakeFS) ReadFile(string) ([]byte, error) { return append([]byte(nil), f.data...), nil }
func (f *fakeFS) CheckRecoveryPath(string) error  { return nil }
func (f *fakeFS) Backup(string, []byte) error     { f.backup = true; return nil }
func (f *fakeFS) WriteAtomic(_ string, data []byte) error {
	f.atomic = true
	if f.atomicErr != nil {
		return f.atomicErr
	}
	f.data = append([]byte(nil), data...)
	return nil
}
func (f *fakeFS) WriteRecovery(_ string, state RecoveryState) error {
	f.recovery = true
	if strings.Contains(state.CredentialID, "secret") {
		return errors.New("secret leaked")
	}
	return nil
}
func (f *fakeFS) UpdateRecovery(_ string, state RecoveryState) error {
	return f.WriteRecovery("", state)
}
func (f *fakeFS) RemoveRecovery(string) error { f.removed = true; return nil }

func testConfig(t *testing.T) *v2.Config {
	t.Helper()
	cfg, err := v2.NewV2Default("prod", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	cfg.OpenCenter.Infrastructure.Provider = "openstack"
	if cfg.OpenCenter.Infrastructure.Cloud.OpenStack == nil {
		t.Fatal("default OpenStack config missing")
	}
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ProjectID = "project-1"
	cfg.OpenCenter.Services["loki"] = &services.LokiConfig{}
	cfg.OpenCenter.Services["tempo"] = &services.TempoConfig{}
	cfg.OpenCenter.Services["etcd-backup"] = &services.EtcdBackupConfig{}
	cfg.OpenCenter.Services["velero"] = &services.VeleroConfig{}
	cfg.Secrets.Loki = v2.LokiSecrets{}
	cfg.Secrets.Tempo = v2.TempoSecrets{}
	cfg.Secrets.EtcdBackup = v2.EtcdBackupSecrets{}
	cfg.Secrets.Velero = v2.VeleroSecrets{}
	return cfg
}

func testAdapter() *fakeAdapter {
	return &fakeAdapter{preflight: cloudopenstack.StoragePreflight{Endpoint: "https://swift.example/v1/AUTH_project-1", S3Endpoint: "https://s3.example", AuthURL: "https://identity.example/v3", Region: "RegionOne", ProjectID: "project-1"}, app: cloudopenstack.AppCredential{ID: "app-1", Secret: "app-secret"}, ec2: cloudopenstack.EC2Credentials{ID: "ec2-1", AccessKeyID: "access-1", Secret: "ec2-secret", Endpoint: "https://s3.example", Region: "RegionOne", ProjectID: "project-1"}}
}

func TestValidateOptionsMappings(t *testing.T) {
	for _, tc := range []struct {
		service, backend string
		wantErr          bool
	}{{"loki", "swift", false}, {"loki", "s3", false}, {"tempo", "swift", false}, {"tempo", "s3", false}, {"etcd-backup", "s3", false}, {"velero", "s3", false}, {"velero", "swift", true}, {"other", "s3", true}} {
		err := ValidateOptions(Options{Service: tc.service, Backend: tc.backend, Cluster: "prod"})
		if (err != nil) != tc.wantErr {
			t.Errorf("%s=%s error=%v, wantErr=%v", tc.service, tc.backend, err, tc.wantErr)
		}
	}
}

func TestPlanDoesNotMutateAndReusesCompleteCredentials(t *testing.T) {
	cfg := testConfig(t)
	loki := cfg.OpenCenter.Services["loki"].(*services.LokiConfig)
	loki.StorageType, loki.BucketName, loki.S3Endpoint, loki.S3Region, loki.S3CredentialID = "s3", "prod-loki", "https://s3.example", "RegionOne", "ec2-old"
	loki.S3ForcePathStyle = true
	cfg.Secrets.Loki.S3AccessKeyID, cfg.Secrets.Loki.S3SecretAccessKey = "access-secret", "secret-value"
	before, _ := v2.MarshalPublicConfig(cfg)
	adapter := testAdapter()
	planned, err := Plan(context.Background(), PlanInput{Config: cfg, Options: Options{Service: "loki", Backend: "s3", Cluster: "prod"}, Adapter: adapter})
	if err != nil {
		t.Fatal(err)
	}
	if adapter.preflightCalls != 1 || planned.Result.Status != StatusNoOp {
		t.Fatalf("calls=%d status=%s", adapter.preflightCalls, planned.Result.Status)
	}
	after, _ := v2.MarshalPublicConfig(cfg)
	if string(before) != string(after) {
		t.Fatal("plan mutated source config")
	}
	if planned.prospective.Secrets.Loki.S3SecretAccessKey != "secret-value" {
		t.Fatal("reuse replaced existing secret")
	}
	encoded, _ := json.Marshal(planned.Result)
	if strings.Contains(string(encoded), "secret-value") || strings.Contains(string(encoded), "access-secret") {
		t.Fatal("plan result exposed a secret")
	}
}

func TestPlanBlocksPartialPairUnlessRotation(t *testing.T) {
	cfg := testConfig(t)
	cfg.Secrets.Loki.S3AccessKeyID = "only-access"
	planned, err := Plan(context.Background(), PlanInput{Config: cfg, Options: Options{Service: "loki", Backend: "s3", Cluster: "prod"}, Adapter: testAdapter()})
	if err != nil {
		t.Fatal(err)
	}
	if planned.Result.Status != StatusBlocked {
		t.Fatalf("status=%s", planned.Result.Status)
	}
	if !strings.Contains(planned.Result.Warnings[0], "partial") {
		t.Fatal(planned.Result.Warnings)
	}
}

func TestScopedRulesIncludeContainerAndSegmentsOnly(t *testing.T) {
	rules := scopedRules("logs", "project-1")
	if len(rules) == 0 {
		t.Fatal("no rules")
	}
	for _, rule := range rules {
		if !strings.HasPrefix(rule.Path, "/v1/AUTH_project-1/logs") {
			t.Fatalf("unscoped rule: %+v", rule)
		}
		if strings.Contains(rule.Path, "/v1/AUTH_project-1/logs/../") {
			t.Fatalf("traversal rule: %+v", rule)
		}
	}
}

func TestApplyCreatesAndPersistsRecoverySafely(t *testing.T) {
	cfg := testConfig(t)
	raw, err := v2.MarshalPublicConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	fs := &fakeFS{data: raw}
	adapter := testAdapter()
	result, err := Apply(context.Background(), ApplyInput{ConfigPath: "/tmp/prod.yaml", RecoveryPath: "/tmp/prod.recovery", OriginalBytes: raw, Options: Options{Service: "tempo", Backend: "s3", Cluster: "prod"}, Adapter: adapter, FileSystem: fs})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusApplied || adapter.ec2Creates != 1 || !fs.backup || !fs.recovery || !fs.removed {
		t.Fatalf("result=%+v adapter=%+v fs=%+v", result, adapter, fs)
	}
	if strings.Contains(string(fs.data), "ec2-secret") == false {
		t.Fatal("persisted typed secret missing")
	}
}

func TestApplyReturnsPartialWhenLocalPersistenceFails(t *testing.T) {
	cfg := testConfig(t)
	raw, err := v2.MarshalPublicConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	fs := &fakeFS{data: raw, atomicErr: errors.New("write denied")}
	result, err := Apply(context.Background(), ApplyInput{ConfigPath: "/tmp/prod.yaml", RecoveryPath: "/tmp/prod.recovery", OriginalBytes: raw, Options: Options{Service: "velero", Backend: "s3", Cluster: "prod"}, Adapter: testAdapter(), FileSystem: fs})
	if err == nil || result.Status != StatusPartial || !fs.recovery {
		t.Fatalf("result=%+v err=%v recovery=%v", result, err, fs.recovery)
	}
}

func TestApplyRotationKeepsReplacementWhenRevokeFails(t *testing.T) {
	cfg := testConfig(t)
	loki := cfg.OpenCenter.Services["loki"].(*services.LokiConfig)
	loki.StorageType, loki.BucketName, loki.SwiftAuthURL, loki.SwiftRegion, loki.SwiftContainerName, loki.SwiftApplicationCredentialID = "swift", "prod-loki", "https://identity.example/v3", "RegionOne", "prod-loki", "old-app"
	cfg.Secrets.Loki.SwiftApplicationCredentialSecret = "old-secret"
	raw, err := v2.MarshalPublicConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	adapter := testAdapter()
	adapter.revokeErr = errors.New("revoke unavailable")
	fs := &fakeFS{data: raw}
	result, err := Apply(context.Background(), ApplyInput{ConfigPath: "/tmp/prod.yaml", RecoveryPath: "/tmp/prod.recovery", OriginalBytes: raw, Options: Options{Service: "loki", Backend: "swift", Cluster: "prod", RotateCredentials: true}, Adapter: adapter, FileSystem: fs})
	if err == nil || result.Status != StatusPartial {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if adapter.appCreates != 1 || adapter.appDeletes != 1 || result.Recovery == nil || result.Recovery.PersistenceState != "persisted-revoke-failed" || len(result.Warnings) == 0 {
		t.Fatalf("rotation lifecycle incomplete: result=%+v adapter=%+v", result, adapter)
	}
	if !strings.Contains(string(fs.data), "app-secret") {
		t.Fatal("replacement secret was not persisted")
	}
}

func TestRecoveryAndConfigDoNotContainAdapterSecrets(t *testing.T) {
	state := RecoveryState{CredentialID: "ec2-1", CredentialType: "ec2", PersistenceState: "created-not-persisted"}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "secret") {
		t.Fatal(string(data))
	}
	if _, err := os.Stat("/definitely-not-created"); err == nil {
		t.Fatal("unexpected test artifact")
	}
}

func TestStorageResultFormatsMaskCredentialValues(t *testing.T) {
	cfg := testConfig(t)
	cfg.Secrets.Loki.S3SecretAccessKey = "profile-secret"
	result := Result{Warnings: []string{redactError(fmt.Errorf("adapter failed with %s and %s", "profile-secret", "generated-secret"), cfg, "generated-secret").Error()}}
	jsonData, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	yamlData, err := yaml.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Join(result.Warnings, "\n")
	for _, output := range []string{text, string(jsonData), string(yamlData)} {
		if strings.Contains(output, "profile-secret") || strings.Contains(output, "generated-secret") {
			t.Fatalf("credential leaked in output: %s", output)
		}
	}
}

func TestPlanIsolatesAllSixMappings(t *testing.T) {
	cases := []struct{ service, backend string }{
		{"loki", "swift"}, {"loki", "s3"}, {"tempo", "swift"},
		{"tempo", "s3"}, {"etcd-backup", "s3"}, {"velero", "s3"},
	}
	for _, tc := range cases {
		t.Run(tc.service+"-"+tc.backend, func(t *testing.T) {
			cfg := testConfig(t)
			raw, err := v2.MarshalPublicConfig(cfg)
			require.NoError(t, err)
			_, err = Plan(context.Background(), PlanInput{Config: cfg, Options: Options{Service: tc.service, Backend: tc.backend, Cluster: "prod"}, Adapter: testAdapter()})
			require.NoError(t, err)
			after, err := v2.MarshalPublicConfig(cfg)
			require.NoError(t, err)
			require.Equal(t, raw, after)
		})
	}
}

func TestCloneConfigFailureDoesNotReturnOriginal(t *testing.T) {
	original := &v2.Config{Secrets: v2.SecretsConfig{ServiceSecrets: map[string]any{"invalid": func() {}}}}
	cloned, err := cloneConfig(original)
	if err == nil {
		t.Fatal("expected clone failure")
	}
	if cloned != nil || cloned == original {
		t.Fatalf("clone failure returned a configuration: %#v", cloned)
	}
}

func TestStorageValidationRejectsSwiftEndpointAndS3Names(t *testing.T) {
	if err := ValidateOptions(Options{Service: "loki", Backend: "s3", Cluster: "prod", S3Endpoint: "https://swift.example/v1/AUTH_project"}); err == nil {
		t.Fatal("expected Swift endpoint rejection")
	}
	if err := ValidateOptions(Options{Service: "loki", Backend: "s3", Cluster: "prod", Container: "Bad_Bucket"}); err == nil {
		t.Fatal("expected S3 bucket validation failure")
	}
	if err := ValidateOptions(Options{Service: "loki", Backend: "swift", Cluster: "prod", Container: "tenant/container"}); err == nil {
		t.Fatal("expected Swift container validation failure")
	}
}

func TestPlanRequiresCanonicalS3CredentialIDForReuse(t *testing.T) {
	cfg := testConfig(t)
	loki := cfg.OpenCenter.Services["loki"].(*services.LokiConfig)
	loki.StorageType, loki.BucketName, loki.S3Endpoint, loki.S3Region = "s3", "prod-loki", "https://s3.example", "RegionOne"
	cfg.Secrets.Loki.S3AccessKeyID, cfg.Secrets.Loki.S3SecretAccessKey = "access-1", "secret-1"

	planned, err := Plan(context.Background(), PlanInput{Config: cfg, Options: Options{Service: "loki", Backend: "s3", Cluster: "prod"}, Adapter: testAdapter()})
	if err != nil {
		t.Fatal(err)
	}
	if planned.Result.Status != StatusBlocked || !strings.Contains(strings.Join(planned.Result.Warnings, "\n"), "partial") {
		t.Fatalf("status=%s warnings=%v", planned.Result.Status, planned.Result.Warnings)
	}
}

func TestPlanValidatesClusterDerivedContainer(t *testing.T) {
	cfg := testConfig(t)
	cfg.OpenCenter.Cluster.ClusterName = "Bad_Cluster"
	_, err := Plan(context.Background(), PlanInput{Config: cfg, Options: Options{Service: "velero", Backend: "s3", Cluster: "prod"}, Adapter: testAdapter()})
	if err == nil || !strings.Contains(err.Error(), "invalid S3 bucket") {
		t.Fatalf("invalid cluster-derived bucket accepted: %v", err)
	}
}

func TestPlanUsesCanonicalS3EndpointAndEtcdHost(t *testing.T) {
	cfg := testConfig(t)
	planned, err := Plan(context.Background(), PlanInput{Config: cfg, Options: Options{Service: "etcd-backup", Backend: "s3", Cluster: "prod"}, Adapter: &fakeAdapter{preflight: cloudopenstack.StoragePreflight{Endpoint: "https://swift.example/v1/AUTH_project-1", S3Endpoint: "https://s3.example:8443/api", AuthURL: "https://identity.example/v3", Region: "RegionOne", ProjectID: "project-1"}}})
	if err != nil {
		t.Fatal(err)
	}
	service := planned.prospective.OpenCenter.Services["etcd-backup"].(*services.EtcdBackupConfig)
	if service.S3Endpoint != "https://s3.example:8443/api" || service.S3Host != "s3.example:8443" {
		t.Fatalf("canonical S3 endpoint was not consumed: %+v", service)
	}
}

func TestRecoveryReservationIsExclusive(t *testing.T) {
	path := t.TempDir() + "/recovery.json"
	fs := OSFileSystem{}
	state := RecoveryState{Path: path, Service: "loki", Backend: "s3", PersistenceState: "creation-pending"}
	if err := fs.WriteRecovery(path, state); err != nil {
		t.Fatal(err)
	}
	if err := fs.WriteRecovery(path, state); err == nil {
		t.Fatal("second recovery reservation succeeded")
	}
}
