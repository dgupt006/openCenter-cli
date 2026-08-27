package openstack

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadProfileRetainsAuthRegionAndTLSInputs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "clouds.yaml")
	contents := `clouds:
  prod:
    region_name: RegionOne
    interface: internal
    cacert: /etc/openstack/ca.pem
    cert: /etc/openstack/client.pem
    key: /etc/openstack/client.key
    insecure: true
    verify: false
    auth:
      auth_url: https://identity.example/v3
      username: operator
      password: secret
      project_id: project-1
      project_name: project
      user_domain_name: Users
      project_domain_name: Projects
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	profile, err := LoadProfile(path, "prod")
	if err != nil {
		t.Fatal(err)
	}
	if profile.AuthURL != "https://identity.example/v3" || profile.Region != "RegionOne" || profile.Interface != "internal" || !profile.Insecure {
		t.Fatalf("profile metadata not retained: %#v", profile)
	}
	if profile.CACert != "/etc/openstack/ca.pem" || profile.Cert != "/etc/openstack/client.pem" || profile.Key != "/etc/openstack/client.key" || profile.Verify == nil || *profile.Verify {
		t.Fatalf("TLS settings not retained: %#v", profile)
	}
	if profile.AuthOptions().TenantID != "project-1" || profile.AuthOptions().DomainName != "Users" {
		t.Fatalf("auth options not mapped: %#v", profile.AuthOptions())
	}
}

func TestProfileProviderHonorsCanceledContextBeforeAuthentication(t *testing.T) {
	profile := Profile{AuthURL: "https://identity.example/v3"}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := profile.provider(ctx); err == nil {
		t.Fatal("expected canceled context error")
	}
}

func TestLoadProfileRequiresSelectedProfile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "clouds.yaml")
	if err := os.WriteFile(path, []byte("clouds: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadProfile(path, "missing"); err == nil {
		t.Fatal("expected missing profile error")
	}
}

func TestDiscoverySnapshotHasNoCredentialSecretFields(t *testing.T) {
	typ := reflect.TypeOf(DiscoverySnapshot{})
	if _, ok := typ.FieldByName("ApplicationCredentialSecret"); ok {
		t.Fatal("DiscoverySnapshot exposes application credential secret")
	}
	if _, ok := typ.FieldByName("ApplicationCredentialID"); ok {
		t.Fatal("DiscoverySnapshot exposes application credential ID import material")
	}
}

func TestDefaultCloudsYAMLPathHonorsEnvironment(t *testing.T) {
	t.Setenv("OS_CLIENT_CONFIG_FILE", "/tmp/test-clouds.yaml")
	if got := DefaultCloudsYAMLPath(); got != "/tmp/test-clouds.yaml" {
		t.Fatalf("default clouds path = %q, want environment override", got)
	}
}
