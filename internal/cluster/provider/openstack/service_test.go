package openstack

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cloudopenstack "github.com/opencenter-cloud/opencenter-cli/internal/cloud/openstack"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	utilerrors "github.com/opencenter-cloud/opencenter-cli/internal/util/errors"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/fs"
)

func TestPlanFillsOnlyUnambiguousProviderFields(t *testing.T) {
	cfg, err := v2.NewV2Default("prod", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack = &v2.OpenStackCloudConfig{}
	cfg.OpenCenter.Infrastructure.Provider = "openstack"

	snapshot := &cloudopenstack.DiscoverySnapshot{
		AuthURL:           "https://identity.example/v3",
		Region:            "RegionOne",
		ProjectID:         "project-1",
		ProjectName:       "project",
		UserDomainName:    "Default",
		ProjectDomainName: "Default",
		Images:            []cloudopenstack.Resource{{ID: "img-linux", Name: "Ubuntu 24.04"}},
		WindowsImages:     []cloudopenstack.Resource{{ID: "img-windows", Name: "Windows 2022"}},
		Networks:          []cloudopenstack.Resource{{ID: "net-1", Name: "cluster-net"}},
		Subnets:           []cloudopenstack.Subnet{{Resource: cloudopenstack.Resource{ID: "subnet-1", Name: "cluster-subnet"}, NetworkID: "net-1"}},
		ExternalNetworks:  []cloudopenstack.Resource{{ID: "public-1", Name: "public"}},
		AvailabilityZones: []cloudopenstack.Resource{{ID: "az1", Name: "az1"}},
	}

	result, prospective, err := Plan(context.Background(), cfg, snapshot, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusPlanned {
		t.Fatalf("status = %q, want %q", result.Status, StatusPlanned)
	}
	osCfg := prospective.OpenCenter.Infrastructure.Cloud.OpenStack
	if osCfg.AuthURL != snapshot.AuthURL || osCfg.ProjectID != snapshot.ProjectID || osCfg.ImageID != "img-linux" || osCfg.NetworkID != "net-1" || osCfg.SubnetID != "subnet-1" {
		t.Fatalf("provider patch did not fill expected values: %#v", osCfg)
	}
	if osCfg.ApplicationCredentialSecret != "" {
		t.Fatal("provider plan imported a secret without --import-auth")
	}
	if len(result.RemoteActions) != 0 || !result.SensitiveValuesRedacted {
		t.Fatalf("unsafe result metadata: %#v", result)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) == "" || strings.Contains(string(encoded), "super-secret") {
		t.Fatalf("structured result contains unexpected sensitive data: %s", encoded)
	}
}

func TestPlanBlocksAmbiguousSelectionWithoutMutation(t *testing.T) {
	cfg, err := v2.NewV2Default("prod", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack = &v2.OpenStackCloudConfig{}
	before := *cfg.OpenCenter.Infrastructure.Cloud.OpenStack

	snapshot := &cloudopenstack.DiscoverySnapshot{
		Images: []cloudopenstack.Resource{{ID: "img-b", Name: "Ubuntu B"}, {ID: "img-a", Name: "Ubuntu A"}},
	}
	result, prospective, err := Plan(context.Background(), cfg, snapshot, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusBlocked || len(result.Selections) == 0 {
		t.Fatalf("result = %#v, want blocked selection", result)
	}
	after := prospective.OpenCenter.Infrastructure.Cloud.OpenStack
	if after.AuthURL != before.AuthURL || after.ImageID != before.ImageID || after.NetworkID != before.NetworkID || after.SubnetID != before.SubnetID || len(after.AvailabilityZones) != len(before.AvailabilityZones) {
		t.Fatal("blocked plan mutated the provider config")
	}
	if got := result.Selections[0].Candidates; len(got) != 2 || got[0].ID != "img-a" || got[1].ID != "img-b" {
		t.Fatalf("candidates are not stable: %#v", got)
	}
}

func TestPlanRequiresReplaceForPopulatedConflict(t *testing.T) {
	cfg, err := v2.NewV2Default("prod", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack = &v2.OpenStackCloudConfig{ImageID: "old-image"}
	snapshot := &cloudopenstack.DiscoverySnapshot{
		Images:            []cloudopenstack.Resource{{ID: "new-image", Name: "Ubuntu"}},
		Networks:          []cloudopenstack.Resource{{ID: "network", Name: "internal"}},
		Subnets:           []cloudopenstack.Subnet{{Resource: cloudopenstack.Resource{ID: "subnet", Name: "internal"}, NetworkID: "network"}},
		ExternalNetworks:  []cloudopenstack.Resource{{ID: "external", Name: "public"}},
		AvailabilityZones: []cloudopenstack.Resource{{ID: "zone", Name: "zone"}},
	}

	result, _, err := Plan(context.Background(), cfg, snapshot, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusBlocked || len(result.Warnings) == 0 {
		t.Fatalf("result = %#v, want conflict block", result)
	}

	result, prospective, err := Plan(context.Background(), cfg, snapshot, Options{Replace: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusPlanned || prospective.OpenCenter.Infrastructure.Cloud.OpenStack.ImageID != "new-image" {
		t.Fatalf("replace did not apply conflict: result=%#v cfg=%q", result, prospective.OpenCenter.Infrastructure.Cloud.OpenStack.ImageID)
	}
}

func TestPlanImportAuthRedactsResult(t *testing.T) {
	cfg, err := v2.NewV2Default("prod", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack = &v2.OpenStackCloudConfig{}
	snapshot := &cloudopenstack.DiscoverySnapshot{
		AuthURL: "https://identity.example/v3",
	}
	result, prospective, err := Plan(context.Background(), cfg, snapshot, Options{ImportAuth: &AuthImport{ApplicationCredentialID: "app-id", ApplicationCredentialSecret: "super-secret"}})
	if err != nil {
		t.Fatal(err)
	}
	if prospective.OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialID != "app-id" || prospective.OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialSecret != "super-secret" {
		t.Fatal("--import-auth did not write profile credentials")
	}
	for _, change := range result.Changes {
		if change.Path == "opencenter.infrastructure.cloud.openstack.application_credential_secret" && change.New != "<redacted>" {
			t.Fatalf("secret was not redacted: %#v", change)
		}
	}
}

func TestApplyPersistenceWritesBackupOnlyForRealPatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cluster.yaml")
	originalBytes := []byte("original\n")
	if err := os.WriteFile(path, originalBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	fileSystem := fs.NewDefaultFileSystem(utilerrors.NewDefaultErrorHandlerWithoutMasking())
	cfg, err := v2.NewV2Default("prod", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	prospective := cloneProviderConfig(cfg)
	result := Result{Status: StatusPlanned, Changes: []Change{{Path: "provider", Old: "", New: "value"}}}
	result, err = (ApplyPersistence{FileSystem: fileSystem, Validate: func(context.Context, *v2.Config) error { return nil }}).Apply(context.Background(), path, cfg, prospective, originalBytes, result)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusApplied {
		t.Fatalf("status = %q, want applied", result.Status)
	}
	backup, err := os.ReadFile(path + ".backup")
	if err != nil {
		t.Fatal(err)
	}
	if string(backup) != string(originalBytes) {
		t.Fatalf("backup = %q, want original bytes", backup)
	}
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(written) == string(originalBytes) {
		t.Fatal("expected atomic write to replace original")
	}

	noOpPath := filepath.Join(dir, "noop.yaml")
	if err := os.WriteFile(noOpPath, originalBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	noOp := Result{Status: StatusNoOp}
	if _, err := (ApplyPersistence{FileSystem: fileSystem}).Apply(context.Background(), noOpPath, cfg, cfg, originalBytes, noOp); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(noOpPath + ".backup"); !os.IsNotExist(err) {
		t.Fatalf("no-op created backup: %v", err)
	}

}

func TestPlanExcludesExternalNetworksFromInternalSelection(t *testing.T) {
	cfg, err := v2.NewV2Default("prod", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack = &v2.OpenStackCloudConfig{}
	snapshot := &cloudopenstack.DiscoverySnapshot{
		Networks:         []cloudopenstack.Resource{{ID: "public", Name: "PUBLICNET"}},
		ExternalNetworks: []cloudopenstack.Resource{{ID: "public", Name: "PUBLICNET"}},
	}
	result, prospective, err := Plan(context.Background(), cfg, snapshot, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusBlocked || prospective.OpenCenter.Infrastructure.Cloud.OpenStack.NetworkID != "" {
		t.Fatalf("external-only inventory selected an internal network: result=%#v config=%#v", result, prospective.OpenCenter.Infrastructure.Cloud.OpenStack)
	}
}

func TestPlanRejectsCrossNetworkSubnetSelector(t *testing.T) {
	cfg, err := v2.NewV2Default("prod", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack = &v2.OpenStackCloudConfig{}
	snapshot := &cloudopenstack.DiscoverySnapshot{
		Images:           []cloudopenstack.Resource{{ID: "image", Name: "Ubuntu"}},
		Networks:         []cloudopenstack.Resource{{ID: "internal", Name: "internal"}},
		Subnets:          []cloudopenstack.Subnet{{Resource: cloudopenstack.Resource{ID: "other-subnet", Name: "other"}, NetworkID: "other-network"}},
		ExternalNetworks: []cloudopenstack.Resource{{ID: "external", Name: "public"}},
	}
	_, _, err = Plan(context.Background(), cfg, snapshot, Options{NetworkID: "internal", SubnetID: "other-subnet", ImageID: "image", ExternalNetworkID: "external"})
	if err == nil || !strings.Contains(err.Error(), "subnet selector") {
		t.Fatalf("cross-network subnet was accepted: %v", err)
	}
}

func TestPlanWritesExternalNetworkNameToPoolMirrors(t *testing.T) {
	cfg, err := v2.NewV2Default("prod", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack = &v2.OpenStackCloudConfig{}
	snapshot := &cloudopenstack.DiscoverySnapshot{
		Images:           []cloudopenstack.Resource{{ID: "image", Name: "Ubuntu"}},
		Networks:         []cloudopenstack.Resource{{ID: "internal", Name: "internal"}},
		Subnets:          []cloudopenstack.Subnet{{Resource: cloudopenstack.Resource{ID: "subnet", Name: "subnet"}, NetworkID: "internal"}},
		ExternalNetworks: []cloudopenstack.Resource{{ID: "external", Name: "PUBLICNET"}},
	}
	result, prospective, err := Plan(context.Background(), cfg, snapshot, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusPlanned {
		t.Fatalf("status = %q, want planned", result.Status)
	}
	osCfg := prospective.OpenCenter.Infrastructure.Cloud.OpenStack
	if osCfg.FloatingIPPool != "PUBLICNET" || osCfg.Networking == nil || osCfg.Networking.FloatingIPPool != "PUBLICNET" || osCfg.FloatingNetworkID != "external" || osCfg.Networking.FloatingNetworkID != "external" {
		t.Fatalf("external compatibility mirrors incorrect: %#v", osCfg)
	}
}

func TestPlanDoesNotPersistTLSWithoutImportTLS(t *testing.T) {
	cfg, err := v2.NewV2Default("prod", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack = &v2.OpenStackCloudConfig{}
	snapshot := &cloudopenstack.DiscoverySnapshot{CA: "/workstation/ca.pem", Insecure: true}
	_, prospective, err := Plan(context.Background(), cfg, snapshot, Options{})
	if err != nil {
		t.Fatal(err)
	}
	osCfg := prospective.OpenCenter.Infrastructure.Cloud.OpenStack
	if osCfg.CA != "" || osCfg.Insecure {
		t.Fatalf("discovery TLS settings were persisted without --import-tls: %#v", osCfg)
	}
	_, prospective, err = Plan(context.Background(), cfg, snapshot, Options{ImportTLS: true, Replace: true})
	if err != nil {
		t.Fatal(err)
	}
	osCfg = prospective.OpenCenter.Infrastructure.Cloud.OpenStack
	if osCfg.CA != snapshot.CA || !osCfg.Insecure {
		t.Fatalf("explicit TLS import was not persisted: %#v", osCfg)
	}
}

func TestPlanPreservesAvailabilityZoneWhenInventoryUnavailable(t *testing.T) {
	cfg, err := v2.NewV2Default("prod", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack = &v2.OpenStackCloudConfig{AvailabilityZone: "existing-zone"}
	result, prospective, err := Plan(context.Background(), cfg, &cloudopenstack.DiscoverySnapshot{}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if prospective.OpenCenter.Infrastructure.Cloud.OpenStack.AvailabilityZone != "existing-zone" || result.Status == StatusBlocked && strings.Contains(strings.Join(result.Warnings, " "), "availability zone") {
		t.Fatalf("unavailable compute inventory changed or blocked existing AZ: result=%#v config=%#v", result, prospective.OpenCenter.Infrastructure.Cloud.OpenStack)
	}
}

func TestApplyPersistenceRejectsStaleSourceBeforeBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cluster.yaml")
	originalBytes := []byte("original\n")
	if err := os.WriteFile(path, originalBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("edited\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := v2.NewV2Default("prod", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	result := Result{Status: StatusPlanned, Changes: []Change{{Path: "provider", Old: "", New: "value"}}}
	fileSystem := fs.NewDefaultFileSystem(utilerrors.NewDefaultErrorHandlerWithoutMasking())
	_, err = (ApplyPersistence{FileSystem: fileSystem}).Apply(context.Background(), path, cfg, cloneProviderConfig(cfg), originalBytes, result)
	if err == nil || !strings.Contains(err.Error(), "changed since planning") {
		t.Fatalf("stale source was not rejected: %v", err)
	}
	if _, statErr := os.Stat(path + ".backup"); !os.IsNotExist(statErr) {
		t.Fatalf("stale source created backup: %v", statErr)
	}
}

func TestApplyPersistenceValidatesBeforeBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cluster.yaml")
	originalBytes := []byte("original\n")
	if err := os.WriteFile(path, originalBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := v2.NewV2Default("prod", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	result := Result{Status: StatusPlanned, Changes: []Change{{Path: "provider", Old: "", New: "value"}}}
	fileSystem := fs.NewDefaultFileSystem(utilerrors.NewDefaultErrorHandlerWithoutMasking())
	_, err = (ApplyPersistence{FileSystem: fileSystem, Validate: func(context.Context, *v2.Config) error { return errors.New("invalid prospective config") }}).Apply(context.Background(), path, cfg, cloneProviderConfig(cfg), originalBytes, result)
	if err == nil || !strings.Contains(err.Error(), "prospective configuration validation failed") {
		t.Fatalf("validation failure was not returned: %v", err)
	}
	if _, statErr := os.Stat(path + ".backup"); !os.IsNotExist(statErr) {
		t.Fatalf("validation failure created backup: %v", statErr)
	}
}
