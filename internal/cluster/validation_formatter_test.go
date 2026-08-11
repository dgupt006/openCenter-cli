package cluster

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseYAMLFieldError(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantParsed  bool
		wantSection string
		wantField   string
	}{
		{
			name:        "field not found in MetaConfig",
			input:       "line 12: field stage not found in type v2.MetaConfig",
			wantParsed:  true,
			wantSection: "Meta",
			wantField:   "stage",
		},
		{
			name:        "field not found in KubernetesConfig",
			input:       "line 94: field kubespray_version not found in type v2.KubernetesConfig",
			wantParsed:  true,
			wantSection: "Cluster > Kubernetes",
			wantField:   "kubespray_version",
		},
		{
			name:        "field not found in SecretsConfig",
			input:       "line 610: field cert_manager not found in type v2.SecretsConfig",
			wantParsed:  true,
			wantSection: "Secrets",
			wantField:   "cert_manager",
		},
		{
			name:        "field not found in AWSCloudConfig",
			input:       "line 34: field profile not found in type v2.AWSCloudConfig",
			wantParsed:  true,
			wantSection: "Infrastructure > Cloud",
			wantField:   "profile",
		},
		{
			name:        "field not found in unknown type falls back to General",
			input:       "line 5: field foo not found in type v2.SomeNewConfig",
			wantParsed:  true,
			wantSection: "General",
			wantField:   "foo",
		},
		{
			name:       "non-matching error returns false",
			input:      "some other error message",
			wantParsed: false,
		},
		{
			name:       "schema validation error is not a YAML field error",
			input:      "schema validation failed: Key: 'Config.OpenCenter.Meta.Env' Error:Field validation for 'Env' failed on the 'oneof' tag",
			wantParsed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseYAMLFieldError(tt.input)
			if ok != tt.wantParsed {
				t.Fatalf("parseYAMLFieldError() parsed = %v, want %v", ok, tt.wantParsed)
			}
			if !ok {
				return
			}
			if got.Section != tt.wantSection {
				t.Errorf("Section = %q, want %q", got.Section, tt.wantSection)
			}
			if got.Field != tt.wantField {
				t.Errorf("Field = %q, want %q", got.Field, tt.wantField)
			}
			if got.Tag != "unknown_field" {
				t.Errorf("Tag = %q, want %q", got.Tag, "unknown_field")
			}
			if !strings.Contains(got.Message, tt.wantField) {
				t.Errorf("Message %q should contain field name %q", got.Message, tt.wantField)
			}
		})
	}
}

func TestParseRawErrors_YAMLFieldErrors(t *testing.T) {
	rawErrors := []string{
		"[validation] line 12: field stage not found in type v2.MetaConfig",
		"[validation] line 94: field kubespray_version not found in type v2.KubernetesConfig",
		"[validation] line 610: field cert_manager not found in type v2.SecretsConfig",
	}

	parsed := parseRawErrors(rawErrors, "openstack")

	if len(parsed) != 3 {
		t.Fatalf("expected 3 errors, got %d", len(parsed))
	}

	// Each error should be individually parsed, not lumped into one General error
	for _, e := range parsed {
		if e.Tag != "unknown_field" {
			t.Errorf("expected tag 'unknown_field', got %q for field %q", e.Tag, e.Field)
		}
		if e.Section == "General" && e.Field == "" {
			t.Errorf("error was not parsed into structured form: %q", e.Message)
		}
	}

	if parsed[0].Section != "Meta" {
		t.Errorf("first error section = %q, want 'Meta'", parsed[0].Section)
	}
	if parsed[1].Section != "Cluster > Kubernetes" {
		t.Errorf("second error section = %q, want 'Cluster > Kubernetes'", parsed[1].Section)
	}
	if parsed[2].Section != "Secrets" {
		t.Errorf("third error section = %q, want 'Secrets'", parsed[2].Section)
	}
}

func TestFormatResultGrouped_YAMLFieldErrors(t *testing.T) {
	service := &ValidateService{}
	result := &ValidationResult{
		Valid:       false,
		ConfigValid: false,
		Errors: []string{
			"[validation] line 12: field stage not found in type v2.MetaConfig",
			"[validation] line 34: field profile not found in type v2.AWSCloudConfig",
			"[validation] line 610: field cert_manager not found in type v2.SecretsConfig",
		},
	}

	output := service.FormatResultGrouped(result, "openstack")

	// Should contain section headers
	if !strings.Contains(output, "Meta:") {
		t.Error("output missing 'Meta:' section")
	}
	if !strings.Contains(output, "Infrastructure > Cloud:") {
		t.Error("output missing 'Infrastructure > Cloud:' section")
	}
	if !strings.Contains(output, "Secrets:") {
		t.Error("output missing 'Secrets:' section")
	}

	// Each error should be on its own line with the field name
	if !strings.Contains(output, "stage") {
		t.Error("output missing field 'stage'")
	}
	if !strings.Contains(output, "profile") {
		t.Error("output missing field 'profile'")
	}
	if !strings.Contains(output, "cert_manager") {
		t.Error("output missing field 'cert_manager'")
	}

	// Should NOT contain the old single-line dump format
	if strings.Contains(output, "[line 12:") {
		t.Error("output still contains old bracket-delimited format")
	}
}

func TestFormatResultGrouped_OperatorReportShape(t *testing.T) {
	service := &ValidateService{}
	result := &ValidationResult{
		Valid:             false,
		ConfigValid:       false,
		ConnectivityValid: true,
		ProviderValid:     true,
		Provider:          "openstack",
		Target: ValidationTarget{
			Cluster:      "prod",
			Organization: "acme",
			Provider:     "openstack",
			ConfigPath:   "/tmp/prod.yaml",
		},
		ValidationMode: "offline",
		CheckSummary: ValidationCheckSummary{
			Passed:   2,
			Failed:   2,
			Warnings: 1,
			Skipped:  1,
		},
		ServiceReports: []ValidationServiceReport{
			{Name: "cert-manager", Status: CheckStatusPass},
			{Name: "keycloak", Status: CheckStatusFail, Message: "missing admin password", Missing: []string{"secrets.keycloak.admin_password"}},
		},
		GitOpsReport: ValidationGitOpsReport{
			RepositoryURL: "ssh://git@github.com/acme/prod.git",
			LocalPath:     "/tmp/gitops",
			Checks: []ValidationCheck{
				{Name: "Repository URL", Status: CheckStatusPass, Detail: "ssh://git@github.com/acme/prod.git"},
				{Name: "Local git", Status: CheckStatusWarn, Message: "dirty, 1 modified"},
				{Name: "Remote checks", Status: CheckStatusSkip, Message: "skipped in offline mode"},
			},
		},
		Missing: []ValidationMissing{
			{Path: "secrets.keycloak.admin_password"},
		},
		ActionItems: []string{
			"Set secrets.keycloak.admin_password.",
			"Commit or stash local GitOps repository changes before deploy.",
		},
	}

	output := service.FormatResultGrouped(result, result.Provider)

	for _, want := range []string{
		"Cluster: acme/prod",
		"Organization: acme",
		"Provider: openstack",
		"Validation mode: offline",
		"Summary: failed",
		"Checks: 2 passed, 2 failed, 1 warning, 1 skipped",
		"Services:",
		"✓ cert-manager",
		"✗ keycloak",
		"GitOps:",
		"⚠ Local git",
		"- Remote checks",
		"Missing:",
		"secrets.keycloak.admin_password",
		"Action Items:",
		"1. Set secrets.keycloak.admin_password.",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%s", want, output)
		}
	}

	if strings.Index(output, "Missing:") < strings.Index(output, "GitOps:") {
		t.Fatalf("Missing section should appear after GitOps:\n%s", output)
	}
	if strings.Index(output, "Action Items:") < strings.Index(output, "Missing:") {
		t.Fatalf("Action Items should appear after Missing:\n%s", output)
	}
}

func TestFormatResultJSON_IncludesOperatorReportFields(t *testing.T) {
	service := &ValidateService{}
	result := &ValidationResult{
		Valid:             true,
		ConfigValid:       true,
		ConnectivityValid: true,
		ProviderValid:     true,
		Target: ValidationTarget{
			Cluster:      "prod",
			Organization: "acme",
			Provider:     "kind",
			ConfigPath:   "/tmp/prod.yaml",
		},
		ValidationMode: "online",
		CheckSummary: ValidationCheckSummary{
			Passed: 3,
		},
		ServiceReports: []ValidationServiceReport{
			{Name: "fluxcd", Status: CheckStatusPass},
		},
		GitOpsReport: ValidationGitOpsReport{
			Checks: []ValidationCheck{
				{Name: "Repository URL", Status: CheckStatusPass},
			},
		},
		ActionItems: []string{"No action required."},
	}

	jsonStr, err := service.FormatResultJSON(result, result.Provider)
	if err != nil {
		t.Fatalf("FormatResultJSON() error = %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &payload); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, jsonStr)
	}

	for _, key := range []string{"target", "mode", "checks", "services", "gitops", "missing", "action_items"} {
		if _, ok := payload[key]; !ok {
			t.Fatalf("expected JSON key %q in %s", key, jsonStr)
		}
	}

	if payload["mode"] != "online" {
		t.Fatalf("mode = %v, want online", payload["mode"])
	}
}


func TestParseYAMLFieldError_ProviderAsSectionSuggestion(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantSuggestion string
	}{
		{
			name:           "baremetal used as section in InfrastructureConfig",
			input:          "line 47: field baremetal not found in type v2.InfrastructureConfig",
			wantSuggestion: "'baremetal' is a provider name",
		},
		{
			name:           "openstack used as section in InfrastructureConfig",
			input:          "line 20: field openstack not found in type v2.InfrastructureConfig",
			wantSuggestion: "'openstack' is a provider name",
		},
		{
			name:           "vmware used as section in InfrastructureConfig",
			input:          "line 30: field vmware not found in type v2.InfrastructureConfig",
			wantSuggestion: "'vmware' is a provider name",
		},
		{
			name:           "aws used as section in InfrastructureConfig",
			input:          "line 15: field aws not found in type v2.InfrastructureConfig",
			wantSuggestion: "'aws' is a provider name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseYAMLFieldError(tt.input)
			if !ok {
				t.Fatal("expected error to be parsed")
			}
			if !strings.Contains(got.Suggestion, tt.wantSuggestion) {
				t.Errorf("Suggestion = %q, want it to contain %q", got.Suggestion, tt.wantSuggestion)
			}
			// Should also suggest setting provider: <name>
			if !strings.Contains(got.Suggestion, "Set 'provider:") {
				t.Errorf("Suggestion = %q, want it to contain 'Set provider:' guidance", got.Suggestion)
			}
		})
	}
}

func TestParseYAMLFieldError_DidYouMeanSuggestion(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantContains   string
		description    string
	}{
		{
			name:         "typo in NetworkingConfig: vip_interfce -> vip_interface",
			input:        "line 50: field vip_interfce not found in type v2.NetworkingConfig",
			wantContains: "vip_interface",
			description:  "should suggest vip_interface for close typo",
		},
		{
			name:         "typo in ComputeConfig: master_node -> master_nodes",
			input:        "line 60: field master_node not found in type v2.ComputeConfig",
			wantContains: "master_nodes",
			description:  "should suggest master_nodes for singular/plural mistake",
		},
		{
			name:         "typo in InfrastructureConfig: neworking -> networking",
			input:        "line 10: field neworking not found in type v2.InfrastructureConfig",
			wantContains: "networking",
			description:  "should suggest networking for typo",
		},
		{
			name:         "typo in KubernetesConfig: kube_vip_enable -> kube_vip_enabled",
			input:        "line 25: field kube_vip_enable not found in type v2.KubernetesConfig",
			wantContains: "kube_vip_enabled",
			description:  "should suggest kube_vip_enabled for missing trailing 'd'",
		},
		{
			name:         "typo in SSHConfig: authorized_key -> authorized_keys",
			input:        "line 30: field authorized_key not found in type v2.SSHConfig",
			wantContains: "authorized_keys",
			description:  "should suggest authorized_keys for plural",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseYAMLFieldError(tt.input)
			if !ok {
				t.Fatal("expected error to be parsed")
			}
			if !strings.Contains(got.Suggestion, tt.wantContains) {
				t.Errorf("Suggestion = %q, want it to contain %q", got.Suggestion, tt.wantContains)
			}
		})
	}
}

func TestParseYAMLFieldError_UnknownFieldShowsValidOptions(t *testing.T) {
	// A field that's completely unrelated should show available fields
	got, ok := parseYAMLFieldError("line 5: field xyzzy_foobar not found in type v2.SSHConfig")
	if !ok {
		t.Fatal("expected error to be parsed")
	}
	// Should contain "Valid fields include:" since no close match exists
	if !strings.Contains(got.Suggestion, "Valid fields include:") {
		t.Errorf("Suggestion = %q, want it to contain 'Valid fields include:'", got.Suggestion)
	}
	// Should mention at least one valid field
	if !strings.Contains(got.Suggestion, "authorized_keys") {
		t.Errorf("Suggestion = %q, want it to mention 'authorized_keys'", got.Suggestion)
	}
}

func TestLevenshteinDistanceFormatter(t *testing.T) {
	tests := []struct {
		s1, s2   string
		expected int
	}{
		{"", "", 0},
		{"abc", "", 3},
		{"", "abc", 3},
		{"abc", "abc", 0},
		{"abc", "abd", 1},
		{"baremetal", "basemetal", 1},
		{"networking", "neworking", 1},
		{"vip_interface", "vip_interfce", 1},
		{"master_nodes", "master_node", 1},
		{"completely_different", "xyz", 19},
	}

	for _, tt := range tests {
		t.Run(tt.s1+"_vs_"+tt.s2, func(t *testing.T) {
			got := levenshteinDistanceFormatter(tt.s1, tt.s2)
			if got != tt.expected {
				t.Errorf("levenshteinDistanceFormatter(%q, %q) = %d, want %d", tt.s1, tt.s2, got, tt.expected)
			}
		})
	}
}

func TestFormatResultGrouped_InlineSuggestions(t *testing.T) {
	service := &ValidateService{}
	result := &ValidationResult{
		Valid:       false,
		ConfigValid: false,
		Errors: []string{
			"[validation] line 47: field baremetal not found in type v2.InfrastructureConfig",
		},
	}

	output := service.FormatResultGrouped(result, "openstack")

	// Should contain the inline suggestion with → prefix
	if !strings.Contains(output, "→") {
		t.Errorf("output missing inline suggestion (→ prefix):\n%s", output)
	}
	// Should mention it's a provider name
	if !strings.Contains(output, "provider name") {
		t.Errorf("output should explain 'baremetal' is a provider name:\n%s", output)
	}
}
