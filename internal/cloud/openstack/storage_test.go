package openstack

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestStoragePreflightIsRedactedByShape(t *testing.T) {
	profile := Profile{AuthURL: "https://identity.example/v3", UserID: "user-1", Password: "password-secret", AppCredSecret: "app-secret"}
	preflight := StoragePreflight{AuthURL: profile.AuthURL, Endpoint: "https://swift.example", Region: "RegionOne", ProjectID: "project-1", CredentialOwnerID: "user-1"}
	data, err := json.Marshal(preflight)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "user-1") || strings.Contains(string(data), "password-secret") || strings.Contains(string(data), "app-secret") {
		t.Fatal("storage preflight exposed private data in JSON")
	}
	yamlData, err := yaml.Marshal(preflight)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(yamlData), "user-1") || strings.Contains(string(yamlData), "password-secret") || strings.Contains(string(yamlData), "app-secret") {
		t.Fatal("storage preflight exposed private data in YAML")
	}
}

func TestStorageAdapterHTTPLifecycleAndContext(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v3/auth/tokens":
			w.Header().Set("X-Subject-Token", "token")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"token":{"user":{"id":"token-user"},"project":{"id":"project-1","name":"project"},"catalog":[{"type":"identity","name":"keystone","endpoints":[{"region":"RegionOne","interface":"public","url":"` + serverURLPlaceholder + `/v3"}]},{"type":"object-store","name":"swift","endpoints":[{"region":"RegionOne","interface":"public","url":"` + serverURLPlaceholder + `/object"}]},{"type":"s3","name":"s3","endpoints":[{"region":"RegionOne","interface":"public","url":"` + serverURLPlaceholder + `/s3"}]}]}}`))
		case r.Method == http.MethodPut && r.URL.Path == "/object/container":
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/application_credentials"):
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"application_credential":{"id":"app-1","secret":"app-secret"}}`))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/credentials/OS-EC2"):
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"credential":{"access":"access-1","secret":"ec2-secret","tenant_id":"project-1"}}`))
		case r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	// The handler is installed after server creation so the response can use its URL.
	serverURLPlaceholder = server.URL
	profile := Profile{AuthURL: server.URL + "/v3", Username: "login-user", Password: "password-secret", DomainName: "Default", ProjectID: "project-1", Region: "RegionOne"}
	adapter := NewStorageAdapter(profile)
	preflight, err := adapter.Preflight(context.Background(), "s3", "", true)
	if err != nil {
		t.Fatal(err)
	}
	if preflight.CredentialOwnerID != "token-user" {
		t.Fatalf("credential owner = %q", preflight.CredentialOwnerID)
	}
	if preflight.S3Endpoint != server.URL+"/s3" {
		t.Fatalf("S3 endpoint = %q", preflight.S3Endpoint)
	}
	if err := adapter.EnsureContainer(context.Background(), ContainerRequest{Name: "container", Region: "RegionOne"}); err != nil {
		t.Fatal(err)
	}
	app, err := adapter.CreateAppCredential(context.Background(), AppCredentialRequest{UserID: preflight.CredentialOwnerID, Name: "storage"})
	if err != nil || app.ID != "app-1" || app.Secret != "app-secret" {
		t.Fatalf("app=%+v err=%v", app, err)
	}
	if err := adapter.DeleteAppCredential(context.Background(), app.ID, preflight.CredentialOwnerID); err != nil {
		t.Fatal(err)
	}
	ec2, err := adapter.CreateEC2Credentials(context.Background(), EC2CredentialRequest{UserID: preflight.CredentialOwnerID, ProjectID: "project-1"})
	if err != nil || ec2.AccessKeyID != "access-1" || ec2.Secret != "ec2-secret" || ec2.Endpoint != server.URL+"/s3" {
		t.Fatalf("ec2=%+v err=%v", ec2, err)
	}
	if err := adapter.DeleteEC2Credentials(context.Background(), ec2.ID, preflight.CredentialOwnerID); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := adapter.Preflight(ctx, "swift", "", false); err == nil {
		t.Fatal("expected canceled preflight")
	}
	joined := strings.Join(requests, "\n")
	for _, want := range []string{"PUT /object/container", "POST /v3/users/token-user/application_credentials", "DELETE /v3/users/token-user/application_credentials/app-1", "POST /v3/users/token-user/credentials/OS-EC2", "DELETE /v3/users/token-user/credentials/OS-EC2/access-1"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing request %q in %s", want, joined)
		}
	}
}

var serverURLPlaceholder = ""

func TestStorageAdapterMasksProfileSecretsInHTTPErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v3/auth/tokens" {
			w.Header().Set("X-Subject-Token", "token")
			_, _ = w.Write([]byte(`{"token":{"project":{"id":"project-1"},"catalog":[]}}`))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("password-secret app-secret"))
	}))
	defer server.Close()
	adapter := NewStorageAdapter(Profile{AuthURL: server.URL + "/v3", Password: "password-secret", UserID: "user-1", AppCredSecret: "app-secret"})
	_, err := adapter.CreateAppCredential(context.Background(), AppCredentialRequest{UserID: "user-1", Name: "storage"})
	if err == nil {
		t.Fatal("expected create error")
	}
	if strings.Contains(err.Error(), "password-secret") || strings.Contains(err.Error(), "app-secret") {
		t.Fatalf("secret leaked: %v", err)
	}
}

func TestStoragePreflightProfileUserIDOverridesTokenUser(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v3/auth/tokens" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("X-Subject-Token", "token")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"token":{"user":{"id":"token-user"},"project":{"id":"project-1"},"catalog":[{"type":"object-store","name":"swift","endpoints":[{"region":"RegionOne","interface":"public","url":"` + serverURLPlaceholder + `/object"}]}]}}`))
	}))
	defer server.Close()
	serverURLPlaceholder = server.URL

	adapter := NewStorageAdapter(Profile{AuthURL: server.URL + "/v3", UserID: "override-user", Password: "password-secret", ProjectID: "project-1", Region: "RegionOne"})
	preflight, err := adapter.Preflight(context.Background(), "swift", "", true)
	if err != nil {
		t.Fatal(err)
	}
	if preflight.CredentialOwnerID != "override-user" {
		t.Fatalf("credential owner = %q, want override-user", preflight.CredentialOwnerID)
	}
}

func TestStoragePreflightMissingTokenUserDoesNotReachCredentialEndpoint(t *testing.T) {
	credentialEndpoint := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/v3/auth/tokens" {
			w.Header().Set("X-Subject-Token", "token")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"token":{"project":{"id":"project-1"},"catalog":[{"type":"object-store","name":"swift","endpoints":[{"region":"RegionOne","interface":"public","url":"` + serverURLPlaceholder + `/object"}]}]}}`))
			return
		}
		if strings.Contains(r.URL.Path, "/application_credentials") || strings.Contains(r.URL.Path, "/credentials/OS-EC2") {
			credentialEndpoint = true
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	serverURLPlaceholder = server.URL

	adapter := NewStorageAdapter(Profile{AuthURL: server.URL + "/v3", Username: "login-user", Password: "password-secret", DomainName: "Default", ProjectID: "project-1", Region: "RegionOne"})
	_, err := adapter.Preflight(context.Background(), "swift", "", true)
	if err == nil || !strings.Contains(err.Error(), "token user") {
		t.Fatalf("error=%v, want missing token user error", err)
	}
	if credentialEndpoint {
		t.Fatal("credential endpoint was reached after owner preflight failure")
	}
}
