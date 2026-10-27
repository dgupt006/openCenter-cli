package openstack

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type stubProvisioner struct {
	created AppCredential
	err     error
	calls   int
	gotName string
}

func (s *stubProvisioner) CreateApplicationCredential(_ context.Context, _ Profile, name, _ string) (AppCredential, error) {
	s.calls++
	s.gotName = name
	return s.created, s.err
}

func writeClouds(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "clouds.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestEnsureApplicationCredentialKeepsExistingPair proves a profile that
// already carries an ID/secret pair is never re-minted and clouds.yaml is
// never rewritten: binding a second cluster to the same profile must not
// rotate the credential the first cluster uses.
func TestEnsureApplicationCredentialKeepsExistingPair(t *testing.T) {
	path := writeClouds(t, `clouds:
  prod:
    auth:
      auth_url: https://identity.example/v3
      application_credential_id: existing-id
      application_credential_secret: existing-secret
`)
	profile, err := LoadProfile(path, "prod")
	if err != nil {
		t.Fatal(err)
	}
	provisioner := &stubProvisioner{created: AppCredential{ID: "new", Secret: "new-secret"}}
	credential, updated, err := EnsureApplicationCredential(context.Background(), profile, provisioner, "")
	if err != nil {
		t.Fatal(err)
	}
	if provisioner.calls != 0 {
		t.Fatalf("minted %d credentials for a profile that already has a pair", provisioner.calls)
	}
	if credential.ID != "existing-id" || credential.Secret != "existing-secret" {
		t.Fatalf("returned credential = %#v, want the existing pair", credential)
	}
	if updated.AppCredID != "existing-id" {
		t.Fatalf("updated profile credential = %q, want unchanged", updated.AppCredID)
	}
}

// TestEnsureApplicationCredentialMintsAndPersists pins the core fix: a
// username/password profile gets a fresh credential, and the pair is written
// into clouds.yaml (preserving the rest of the profile) before being
// returned, so the secret is never lost and every later run reuses it.
func TestEnsureApplicationCredentialMintsAndPersists(t *testing.T) {
	path := writeClouds(t, `clouds:
  prod:
    region_name: DFW3
    auth:
      auth_url: https://identity.example/v3
      username: operator
      password: hunter2
      project_name: proj
      project_domain_name: Default
`)
	profile, err := LoadProfile(path, "prod")
	if err != nil {
		t.Fatal(err)
	}
	if profile.SourcePath != path {
		t.Fatalf("profile SourcePath = %q, want %q", profile.SourcePath, path)
	}
	provisioner := &stubProvisioner{created: AppCredential{ID: "minted-id", Secret: "minted-secret"}}
	credential, updated, err := EnsureApplicationCredential(context.Background(), profile, provisioner, "")
	if err != nil {
		t.Fatal(err)
	}
	if provisioner.calls != 1 || provisioner.gotName != DefaultApplicationCredentialName {
		t.Fatalf("provisioner calls=%d name=%q", provisioner.calls, provisioner.gotName)
	}
	if credential.ID != "minted-id" || credential.Secret != "minted-secret" {
		t.Fatalf("returned credential = %#v", credential)
	}
	if updated.AppCredID != "minted-id" || updated.AppCredSecret != "minted-secret" {
		t.Fatalf("reloaded profile = %#v, want the minted pair", updated)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var file providerCloudsFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}
	saved := file.Clouds["prod"]
	if saved.Auth.ApplicationCredentialID != "minted-id" || saved.Auth.ApplicationCredentialSecret != "minted-secret" {
		t.Fatalf("credential not persisted to clouds.yaml:\n%s", data)
	}
	// Everything the operator had must survive the rewrite.
	if saved.RegionName != "DFW3" || saved.Auth.Username != "operator" || saved.Auth.Password != "hunter2" || saved.Auth.ProjectName != "proj" {
		t.Fatalf("clouds.yaml rewrite lost profile data:\n%s", data)
	}
	// The rewritten file must stay 0600: it now holds a minted secret.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("clouds.yaml mode = %v, want 0600", info.Mode().Perm())
	}
	// The atomic write must leave no temp files behind.
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "clouds.yaml" {
		t.Fatalf("atomic write left extra files: %v", entries)
	}
}

// TestEnsureApplicationCredentialReplacesNameOnly proves a name-only
// application credential (no ID) is replaced by a minted ID/secret pair:
// the cluster config consumes the ID, and the name alone cannot be
// persisted into a usable credential.
func TestEnsureApplicationCredentialReplacesNameOnly(t *testing.T) {
	path := writeClouds(t, `clouds:
  prod:
    auth:
      auth_url: https://identity.example/v3
      application_credential_name: only-name
      application_credential_secret: s
`)
	profile, err := LoadProfile(path, "prod")
	if err != nil {
		t.Fatal(err)
	}
	provisioner := &stubProvisioner{created: AppCredential{ID: "minted", Secret: "sec"}}
	if _, updated, err := EnsureApplicationCredential(context.Background(), profile, provisioner, "mycred"); err != nil {
		t.Fatal(err)
	} else if updated.AppCredID != "minted" {
		t.Fatalf("name-only profile not replaced: %#v", updated)
	}
	if provisioner.gotName != "mycred" {
		t.Fatalf("custom credential name ignored: %q", provisioner.gotName)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var file providerCloudsFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}
	if file.Clouds["prod"].Auth.ApplicationCredentialID != "minted" || file.Clouds["prod"].Auth.ApplicationCredentialName != "mycred" {
		t.Fatalf("name-only credential not replaced on disk:\n%s", data)
	}
}

// TestEnsureApplicationCredentialSurfacesProvisionerError proves a Keystone
// failure aborts before any clouds.yaml write: a half-updated profile with a
// name but no secret is worse than an untouched one.
func TestEnsureApplicationCredentialSurfacesProvisionerError(t *testing.T) {
	path := writeClouds(t, `clouds:
  prod:
    auth:
      auth_url: https://identity.example/v3
      username: operator
      password: hunter2
`)
	profile, err := LoadProfile(path, "prod")
	if err != nil {
		t.Fatal(err)
	}
	provisioner := &stubProvisioner{err: errors.New("keystone rejected: 403 forbidden")}
	if _, _, err := EnsureApplicationCredential(context.Background(), profile, provisioner, ""); err == nil || !strings.Contains(err.Error(), "403 forbidden") {
		t.Fatalf("error = %v, want the Keystone failure", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "application_credential_id") {
		t.Fatalf("clouds.yaml modified after a failed mint:\n%s", data)
	}
}

// TestEnsureApplicationCredentialRequiresSourcePath proves a profile loaded
// without a file path cannot receive a minted credential — there is nowhere
// safe to persist the secret.
func TestEnsureApplicationCredentialRequiresSourcePath(t *testing.T) {
	provisioner := &stubProvisioner{created: AppCredential{ID: "x", Secret: "y"}}
	_, _, err := EnsureApplicationCredential(context.Background(), Profile{Name: "prod"}, provisioner, "")
	if err == nil || !strings.Contains(err.Error(), "clouds.yaml") {
		t.Fatalf("error = %v, want the missing-path failure", err)
	}
	if provisioner.calls != 0 {
		t.Fatal("credential minted although it could not be persisted")
	}
}
