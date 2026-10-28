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
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/opencenter-cloud/opencenter-cli/internal/security"
)

// CredentialCheckOptions describes the credentials and expectations that
// ValidateOpenStackCredentials authenticates and asserts.
type CredentialCheckOptions struct {
	AuthURL                     string
	Region                      string
	ApplicationCredentialID     string
	ApplicationCredentialSecret string
	Domain                      string
	TenantName                  string
	ProjectName                 string
	ProjectDomainName           string
	UserDomainName              string
	Insecure                    bool

	Username string
	Password string
}

// CredentialHealth is the outcome of a live Keystone authentication attempt.
type CredentialHealth struct {
	// Authenticated reports that Keystone accepted the credentials.
	Authenticated bool
	// TokenRegion is the region_name reported by the token, when the cloud
	// provides one.
	TokenRegion string
	// CatalogRegions are the distinct regions advertised by the service
	// catalog. Many clouds omit token.region_name, so the catalog is the
	// authoritative fallback for the region assertion.
	CatalogRegions []string
}

// keystoneTokenResponse is the subset of the v3 token body needed to assert
// region placement. gophercloud's Authenticate helper discards the body, so
// the request is issued directly to keep region_name and the catalog.
type keystoneTokenResponse struct {
	Token struct {
		RegionName string `json:"region_name"`
		Catalog    []struct {
			Type      string `json:"type"`
			Endpoints []struct {
				Region   string `json:"region"`
				RegionID string `json:"region_id"`
			} `json:"endpoints"`
		} `json:"catalog"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Title   string `json:"title"`
		} `json:"error"`
	} `json:"token"`
}

// credentialCheckTimeout bounds the preflight network call so an unreachable
// keystone host fails deploy quickly instead of hanging the pipeline.
const credentialCheckTimeout = 20 * time.Second

// ValidateOpenStackCredentials authenticates against Keystone for real and
// asserts that the configured region is served by the configured auth_url.
//
// It exists because auth_url is consumed verbatim by everything that runs
// inside the cluster: the rendered OpenStack cloud-controller-manager
// cloudConfig and the cinder CSI cloud.conf both embed it. A credential that
// was issued in a different region than auth_url points at is rejected by
// Keystone with 404 "Could not find Application Credential". That is invisible
// at provision time when OpenTofu authenticates through OS_* environment
// variables which happen to resolve correctly, so deploy reports success while
// every in-cluster consumer is permanently broken: the CCM never initializes
// nodes, the cloud-provider-uninitialized taint is never removed, and the
// cluster is left unschedulable. Catching it here fails deploy at preflight.
//
// The returned error never contains the credential secret.
func ValidateOpenStackCredentials(ctx context.Context, opts CredentialCheckOptions) (*CredentialHealth, error) {
	authURL := strings.TrimSpace(opts.AuthURL)
	if authURL == "" {
		return nil, errors.New("cloud.openstack.auth_url is empty; authentication may fail")
	}
	payload, err := keystoneAuthPayload(opts)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, credentialCheckTimeout)
	defer cancel()

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode keystone auth request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, tokensURL(authURL), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("invalid cloud.openstack.auth_url %q: %w", authURL, err)
	}
	request.Header.Set("Content-Type", "application/json")

	client := credentialHTTPClient(opts.Insecure)
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("keystone at auth_url %q is unreachable: %w", authURL, err)
	}
	defer func() { _ = response.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read keystone response from auth_url %q: %w", authURL, err)
	}
	secrets := []string{opts.ApplicationCredentialSecret, opts.Password}
	mask := func(message string) string { return security.MaskSecrets(message, secrets...) }

	if response.StatusCode < 200 || response.StatusCode > 299 {
		return nil, classifyAuthFailure(response.StatusCode, raw, authURL, opts, mask)
	}

	var parsed keystoneTokenResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("parse keystone token response from auth_url %q: %s", authURL, mask(err.Error()))
	}
	health := &CredentialHealth{Authenticated: true, TokenRegion: strings.TrimSpace(parsed.Token.RegionName)}
	health.CatalogRegions = tokenCatalogRegions(parsed)
	if err := assertRegion(health, opts.Region, authURL); err != nil {
		return health, err
	}
	return health, nil
}

// keystoneAuthPayload builds the v3 token create body, preferring application
// credentials and falling back to username/password with a project scope.
func keystoneAuthPayload(opts CredentialCheckOptions) (map[string]any, error) {
	identity := map[string]any{}
	switch {
	case strings.TrimSpace(opts.ApplicationCredentialID) != "" && strings.TrimSpace(opts.ApplicationCredentialSecret) != "":
		identity["methods"] = []string{"application_credential"}
		identity["application_credential"] = map[string]any{
			"id":     strings.TrimSpace(opts.ApplicationCredentialID),
			"secret": opts.ApplicationCredentialSecret,
		}
	case strings.TrimSpace(opts.Username) != "" && opts.Password != "":
		identity["methods"] = []string{"password"}
		user := map[string]any{
			"name":     strings.TrimSpace(opts.Username),
			"password": opts.Password,
		}
		if domain := firstNonEmptyStr(opts.UserDomainName, opts.Domain); domain != "" {
			user["domain"] = map[string]any{"name": domain}
		}
		password := map[string]any{"user": user}
		tenant := firstNonEmptyStr(opts.ProjectName, opts.TenantName)
		if tenant != "" {
			project := map[string]any{"name": tenant}
			if projectDomain := firstNonEmptyStr(opts.ProjectDomainName, opts.Domain); projectDomain != "" {
				project["domain"] = map[string]any{"name": projectDomain}
			}
			password["scope"] = map[string]any{"project": project}
		} else if domain := firstNonEmptyStr(opts.UserDomainName, opts.Domain); domain != "" {
			password["scope"] = map[string]any{"domain": map[string]any{"name": domain}}
		}
		identity["password"] = password
	default:
		return nil, errors.New("cloud.openstack credentials are incomplete: set application_credential_id and application_credential_secret, or username and password")
	}
	return map[string]any{"auth": map[string]any{"identity": identity}}, nil
}

// tokensURL appends the identity v3 token path to the configured auth_url,
// tolerating both bare hosts and URLs that already carry /v3.
func tokensURL(authURL string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(authURL), "/")
	if strings.HasSuffix(trimmed, "/v3") {
		return trimmed + "/auth/tokens"
	}
	return trimmed + "/v3/auth/tokens"
}

// tokenCatalogRegions returns the distinct regions advertised by the token
// catalog, sorted for deterministic messages.
func tokenCatalogRegions(parsed keystoneTokenResponse) []string {
	seen := map[string]struct{}{}
	var regions []string
	for _, entry := range parsed.Token.Catalog {
		for _, endpoint := range entry.Endpoints {
			region := firstNonEmptyStr(endpoint.Region, endpoint.RegionID)
			if region == "" {
				continue
			}
			if _, ok := seen[region]; ok {
				continue
			}
			seen[region] = struct{}{}
			regions = append(regions, region)
		}
	}
	sort.Strings(regions)
	return regions
}

// assertRegion fails when the configured region is not served by the
// authenticated cloud. A token region or catalog region match is sufficient;
// clouds that report neither cannot be asserted, so the check stays silent
// rather than blocking a working configuration.
func assertRegion(health *CredentialHealth, region, authURL string) error {
	region = strings.TrimSpace(region)
	if region == "" {
		return nil
	}
	if health.TokenRegion == "" && len(health.CatalogRegions) == 0 {
		return nil
	}
	if strings.EqualFold(health.TokenRegion, region) {
		return nil
	}
	for _, available := range health.CatalogRegions {
		if strings.EqualFold(available, region) {
			return nil
		}
	}
	return fmt.Errorf(
		"cloud.openstack.region %q is not offered by cloud.openstack.auth_url %q; that endpoint serves region(s) %s — point auth_url at the keystone that serves %q, or set region to one of %s",
		region, authURL, quoteList(health.CatalogRegions), region, quoteList(health.CatalogRegions),
	)
}

// classifyAuthFailure turns a Keystone rejection into an actionable message.
// A 404 naming an application credential is the signature of a credential
// issued in a different region than auth_url points at, so it is called out
// with the sibling endpoint derived from the configured URL.
func classifyAuthFailure(status int, raw []byte, authURL string, opts CredentialCheckOptions, mask func(string) string) error {
	var parsed keystoneTokenResponse
	_ = json.Unmarshal(raw, &parsed)
	message := ""
	if parsed.Token.Error != nil {
		message = strings.TrimSpace(parsed.Token.Error.Message)
	}
	if message == "" {
		message = strings.TrimSpace(snip(strings.TrimSpace(string(raw))))
	}
	message = mask(message)

	// Keystone also answers with 200 + token.error for some deployments.
	if status == http.StatusNotFound || status == http.StatusUnauthorized || parsed.Token.Error != nil {
		credentialID := strings.TrimSpace(opts.ApplicationCredentialID)
		if credentialID != "" && mentionsCredential(message) {
			return fmt.Errorf(
				"application credential %q is not known to the keystone endpoint at auth_url %q (HTTP %d%s). "+
					"Application credentials are scoped to the region that issued them: if region %q is correct, "+
					"auth_url points at the wrong regional keystone. %s%s",
				credentialID, authURL, status, errorTitle(parsed), opts.Region, siblingHint(authURL, opts.Region), keystoneSaid(message))
		}
		if status == http.StatusUnauthorized {
			return fmt.Errorf("keystone rejected the credentials presented to auth_url %q (HTTP 401):%s", authURL, keystoneSaid(message))
		}
		return fmt.Errorf("keystone authentication failed for auth_url %q (HTTP %d):%s", authURL, status, keystoneSaid(message))
	}
	return fmt.Errorf("keystone authentication failed for auth_url %q (HTTP %d):%s", authURL, status, keystoneSaid(message))
}

// errorTitle renders the keystone error title as a status hint.
func errorTitle(parsed keystoneTokenResponse) string {
	if parsed.Token.Error == nil {
		return ""
	}
	if title := strings.TrimSpace(parsed.Token.Error.Title); title != "" {
		return ", " + title
	}
	return ""
}

// keystoneSaid renders the upstream message as a trailing clause.
func keystoneSaid(message string) string {
	if message == "" {
		return " (no message from keystone)"
	}
	return " " + message
}

// mentionsCredential reports whether the message is the classic wrong-region
// application credential rejection.
func mentionsCredential(message string) bool {
	lowered := strings.ToLower(message)
	return strings.Contains(lowered, "application credential") || strings.Contains(lowered, "application_credential")
}

// siblingHint suggests the keystone host that would match the configured
// region when the configured host embeds a different region code, as in
// https://keystone.api.dfw3.example.com/v3/ with region IAD3.
func siblingHint(authURL, region string) string {
	region = strings.TrimSpace(region)
	if region == "" {
		return ""
	}
	host := authURL
	if i := strings.Index(host, "://"); i >= 0 {
		host = host[i+3:]
	}
	if i := strings.Index(host, "/"); i >= 0 {
		host = host[:i]
	}
	parts := strings.Split(host, ".")
	replaced := false
	for index, part := range parts {
		if looksLikeRegionToken(part) {
			parts[index] = strings.ToLower(region)
			replaced = true
			break
		}
	}
	if !replaced {
		return ""
	}
	scheme := "https://"
	if i := strings.Index(authURL, "://"); i >= 0 {
		scheme = authURL[:i+3]
	}
	return fmt.Sprintf("A credential issued in %q is usually served by %s%s%s",
		region, scheme, strings.Join(parts, "."), versionSuffix(authURL))
}

// versionSuffix returns the trailing path of the auth URL (for example "/v3/")
// so a suggested sibling endpoint keeps the original API version.
func versionSuffix(authURL string) string {
	if i := strings.Index(authURL, "://"); i >= 0 {
		if rest := authURL[i+3:]; strings.Contains(rest, "/") {
			return rest[strings.Index(rest, "/"):]
		}
	}
	return ""
}

// looksLikeRegionToken reports whether a host label is a region code rather
// than a generic service word: short, alphanumeric, carries a digit, and not a
// known keyword.
func looksLikeRegionToken(label string) bool {
	label = strings.ToLower(strings.TrimSpace(label))
	if len(label) < 3 || len(label) > 6 {
		return false
	}
	switch label {
	case "api", "keystone", "identity", "auth", "cloud", "internal", "openstack":
		return false
	}
	hasDigit := false
	for _, r := range label {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
			hasDigit = true
		default:
			return false
		}
	}
	// Region codes in this deployment style pair letters with digits (IAD3,
	// DFW3, SJC3); a purely alphabetic label is more likely a service name.
	return hasDigit
}

// credentialHTTPClient skips TLS verification only when the cluster config
// opts in, matching how the rendered in-cluster services consume insecure.
func credentialHTTPClient(insecure bool) *http.Client {
	transport := &http.Transport{}
	if insecure {
		//nolint:gosec // explicitly configured by cloud.openstack.insecure
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	return &http.Client{Transport: transport, Timeout: credentialCheckTimeout}
}

func firstNonEmptyStr(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func quoteList(values []string) string {
	if len(values) == 0 {
		return "(none)"
	}
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, fmt.Sprintf("%q", value))
	}
	return strings.Join(quoted, ", ")
}

// snip truncates a response body so a large HTML error page cannot swamp the
// deploy log.
func snip(body string) string {
	const limit = 400
	if len(body) <= limit {
		return body
	}
	return body[:limit] + "…"
}
