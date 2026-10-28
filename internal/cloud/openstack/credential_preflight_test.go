// Copyright 2025 Victor Palma <victor.palma@rackspace.com>
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package openstack

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// keystoneStub serves a fixed response to the v3 token create request.
func keystoneStub(t *testing.T, status int, body any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v3/auth/tokens" || r.Method != http.MethodPost {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	}))
}

func tokenBody(region string, catalogRegions ...string) map[string]any {
	endpoints := make([]map[string]any, 0, len(catalogRegions))
	for _, region := range catalogRegions {
		endpoints = append(endpoints, map[string]any{"region": region, "interface": "public", "url": "https://example.invalid"})
	}
	return map[string]any{"token": map[string]any{
		"region_name": region,
		"catalog":     []map[string]any{{"type": "compute", "name": "nova", "endpoints": endpoints}},
	}}
}

func keystoneError(status int, message string) map[string]any {
	return map[string]any{"error": map[string]any{"code": status, "message": message, "title": "Not Found"}}
}

// A credential issued in one region is not resolvable through another
// region's keystone. This is the exact kit-kat failure: auth_url served DFW3
// while the credential and region were IAD3.
func TestValidateOpenStackCredentials_WrongRegionKeystone(t *testing.T) {
	const secret = "super-secret-app-cred-value"
	server := keystoneStub(t, http.StatusNotFound, keystoneError(http.StatusNotFound,
		"Could not find Application Credential: 3165b08519794450ab6994c61693f63e."))
	defer server.Close()

	_, err := ValidateOpenStackCredentials(context.Background(), CredentialCheckOptions{
		AuthURL:                     server.URL + "/v3/",
		Region:                      "IAD3",
		ApplicationCredentialID:     "3165b08519794450ab6994c61693f63e",
		ApplicationCredentialSecret: secret,
	})
	if err == nil {
		t.Fatal("expected an error for a credential the endpoint does not know")
	}
	message := err.Error()
	for _, want := range []string{
		"3165b08519794450ab6994c61693f63e",
		"not known to the keystone endpoint",
		"scoped to the region that issued them",
	} {
		if !strings.Contains(message, want) {
			t.Errorf("error %q missing %q", message, want)
		}
	}
	if strings.Contains(message, secret) {
		t.Errorf("error leaked the application credential secret: %s", message)
	}
}

// The wrong-region message must name the endpoint the operator should use.
// The stub host is 127.0.0.1, which carries no region label, so no suggestion
// is emitted; the labelled-host case is covered separately.
func TestSiblingHint(t *testing.T) {
	got := siblingHint("https://keystone.api.dfw3.rackspacecloud.com/v3/", "IAD3")
	if !strings.Contains(got, "https://keystone.api.iad3.rackspacecloud.com/v3/") {
		t.Errorf("siblingHint = %q, want the iad3 host with the /v3/ suffix preserved", got)
	}
	if siblingHint("https://keystone.example.com/v3/", "IAD3") != "" {
		t.Error("siblingHint should stay silent when the host carries no region label")
	}
	if siblingHint("https://keystone.api.dfw3.example.com/v3/", "") != "" {
		t.Error("siblingHint should stay silent without a configured region")
	}
}

func TestLooksLikeRegionToken(t *testing.T) {
	for _, label := range []string{"iad3", "dfw3", "sjc3", "dfw1"} {
		if !looksLikeRegionToken(label) {
			t.Errorf("%q should look like a region token", label)
		}
	}
	for _, label := range []string{"api", "keystone", "identity", "internal", "com", "rackspacecloud", "keystone-api"} {
		if looksLikeRegionToken(label) {
			t.Errorf("%q should not look like a region token", label)
		}
	}
}

func TestValidateOpenStackCredentials_RegionMismatch(t *testing.T) {
	server := keystoneStub(t, http.StatusOK, tokenBody("", "DFW3"))
	defer server.Close()

	_, err := ValidateOpenStackCredentials(context.Background(), CredentialCheckOptions{
		AuthURL:                     server.URL + "/v3",
		Region:                      "IAD3",
		ApplicationCredentialID:     "id",
		ApplicationCredentialSecret: "secret",
	})
	if err == nil {
		t.Fatal("expected a region assertion error")
	}
	for _, want := range []string{"IAD3", "DFW3", "not offered by"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q missing %q", err.Error(), want)
		}
	}
}

func TestValidateOpenStackCredentials_Healthy(t *testing.T) {
	server := keystoneStub(t, http.StatusOK, tokenBody("IAD3", "IAD3", "DFW3"))
	defer server.Close()

	health, err := ValidateOpenStackCredentials(context.Background(), CredentialCheckOptions{
		AuthURL:                     server.URL + "/v3",
		Region:                      "iad3", // case-insensitive match
		ApplicationCredentialID:     "id",
		ApplicationCredentialSecret: "secret",
	})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if !health.Authenticated {
		t.Error("expected Authenticated")
	}
	if strings.Join(health.CatalogRegions, ",") != "DFW3,IAD3" {
		t.Errorf("CatalogRegions = %v, want sorted DFW3,IAD3", health.CatalogRegions)
	}
}

// Some clouds omit token.region_name; the catalog alone must satisfy the check.
func TestValidateOpenStackCredentials_CatalogOnlyRegion(t *testing.T) {
	server := keystoneStub(t, http.StatusOK, tokenBody("", "IAD3"))
	defer server.Close()

	if _, err := ValidateOpenStackCredentials(context.Background(), CredentialCheckOptions{
		AuthURL:                     server.URL + "/v3",
		Region:                      "IAD3",
		ApplicationCredentialID:     "id",
		ApplicationCredentialSecret: "secret",
	}); err != nil {
		t.Fatalf("catalog region should satisfy the assertion, got %v", err)
	}
}

// A cloud that reports no region anywhere cannot be asserted; the check must
// not block a working configuration.
func TestValidateOpenStackCredentials_NoRegionInfo(t *testing.T) {
	server := keystoneStub(t, http.StatusOK, tokenBody(""))
	defer server.Close()

	if _, err := ValidateOpenStackCredentials(context.Background(), CredentialCheckOptions{
		AuthURL:                     server.URL + "/v3",
		Region:                      "IAD3",
		ApplicationCredentialID:     "id",
		ApplicationCredentialSecret: "secret",
	}); err != nil {
		t.Fatalf("expected no region assertion without region data, got %v", err)
	}
}

func TestValidateOpenStackCredentials_RejectsBadSecret(t *testing.T) {
	const secret = "hunter2-correct-horse"
	server := keystoneStub(t, http.StatusUnauthorized, keystoneError(http.StatusUnauthorized, "The request you have made requires authentication."))
	defer server.Close()

	_, err := ValidateOpenStackCredentials(context.Background(), CredentialCheckOptions{
		AuthURL:                     server.URL + "/v3",
		Region:                      "IAD3",
		ApplicationCredentialID:     "id",
		ApplicationCredentialSecret: secret,
	})
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("expected a 401 error, got %v", err)
	}
	if strings.Contains(err.Error(), secret) {
		t.Errorf("error leaked the secret: %s", err.Error())
	}
}

func TestValidateOpenStackCredentials_EmptyAuthURL(t *testing.T) {
	if _, err := ValidateOpenStackCredentials(context.Background(), CredentialCheckOptions{}); err == nil {
		t.Fatal("expected an error for an empty auth_url")
	}
}

func TestValidateOpenStackCredentials_NoCredentialsConfigured(t *testing.T) {
	_, err := ValidateOpenStackCredentials(context.Background(), CredentialCheckOptions{AuthURL: "https://keystone.invalid/v3"})
	if err == nil || !strings.Contains(err.Error(), "credentials are incomplete") {
		t.Fatalf("expected an incomplete-credentials error, got %v", err)
	}
}

func TestValidateOpenStackCredentials_UnreachableEndpoint(t *testing.T) {
	// Port 1 is reserved and never listening, so the dial fails fast.
	_, err := ValidateOpenStackCredentials(context.Background(), CredentialCheckOptions{
		AuthURL:                     "http://127.0.0.1:1/v3",
		Region:                      "IAD3",
		ApplicationCredentialID:     "id",
		ApplicationCredentialSecret: "secret",
	})
	if err == nil || !strings.Contains(err.Error(), "unreachable") {
		t.Fatalf("expected an unreachable error, got %v", err)
	}
}

// A keystone host that accepts nothing and answers nothing must not be able to
// hang the deploy pipeline: the preflight bounds the call and reports it.
func TestValidateOpenStackCredentials_BoundedByTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("waits out the credential check timeout")
	}
	// 240.0.0.1 is reserved by RFC 1112, so the connection hangs rather than
	// being refused, and only the deadline can end the call.
	start := time.Now()
	_, err := ValidateOpenStackCredentials(context.Background(), CredentialCheckOptions{
		AuthURL:                     "https://240.0.0.1/v3",
		Region:                      "IAD3",
		ApplicationCredentialID:     "id",
		ApplicationCredentialSecret: "secret",
	})
	elapsed := time.Since(start)
	if err == nil || !strings.Contains(err.Error(), "unreachable") {
		t.Fatalf("expected an unreachable error, got %v", err)
	}
	if elapsed > credentialCheckTimeout+5*time.Second {
		t.Errorf("call took %s; the preflight must stay bounded by %s", elapsed, credentialCheckTimeout)
	}
}

func TestKeystoneAuthPayload(t *testing.T) {
	appCred, err := keystoneAuthPayload(CredentialCheckOptions{
		ApplicationCredentialID: "id", ApplicationCredentialSecret: "secret",
	})
	if err != nil {
		t.Fatalf("application credential payload: %v", err)
	}
	identity := appCred["auth"].(map[string]any)["identity"].(map[string]any)
	if fmt.Sprint(identity["methods"]) != "[application_credential]" {
		t.Errorf("methods = %v", identity["methods"])
	}
	if _, ok := identity["application_credential"]; !ok {
		t.Error("expected an application_credential block")
	}

	// An incomplete application credential must not silently fall through.
	if _, err := keystoneAuthPayload(CredentialCheckOptions{ApplicationCredentialID: "id"}); err == nil {
		t.Error("expected an error when only the credential id is set")
	}

	password, err := keystoneAuthPayload(CredentialCheckOptions{
		Username: "user", Password: "pass", UserDomainName: "dom", ProjectName: "proj", ProjectDomainName: "projdom",
	})
	if err != nil {
		t.Fatalf("password payload: %v", err)
	}
	passwordIdentity := password["auth"].(map[string]any)["identity"].(map[string]any)
	block, ok := passwordIdentity["password"].(map[string]any)
	if !ok {
		t.Fatal("expected a password block")
	}
	scope, ok := block["scope"].(map[string]any)
	if !ok {
		t.Fatal("password auth must be scoped to a project")
	}
	project := scope["project"].(map[string]any)
	if project["name"] != "proj" || project["domain"].(map[string]any)["name"] != "projdom" {
		t.Errorf("unexpected project scope: %v", project)
	}
}

func TestTokensURL(t *testing.T) {
	for input, want := range map[string]string{
		"https://keystone.example.com/v3/": "https://keystone.example.com/v3/auth/tokens",
		"https://keystone.example.com/v3":  "https://keystone.example.com/v3/auth/tokens",
		"https://keystone.example.com":     "https://keystone.example.com/v3/auth/tokens",
	} {
		if got := tokensURL(input); got != want {
			t.Errorf("tokensURL(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestTokenCatalogRegionsDeduplicatesAndSorts(t *testing.T) {
	var parsed keystoneTokenResponse
	raw := `{"token":{"catalog":[
	  {"type":"compute","endpoints":[{"region":"IAD3"},{"region":"DFW3"}]},
	  {"type":"network","endpoints":[{"region_id":"IAD3"},{"region":"AAA1"},{"region":""}]}
	]}}`
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatal(err)
	}
	got := tokenCatalogRegions(parsed)
	if strings.Join(got, ",") != "AAA1,DFW3,IAD3" {
		t.Errorf("tokenCatalogRegions = %v, want AAA1,DFW3,IAD3", got)
	}
}
