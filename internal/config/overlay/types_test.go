package overlay

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

func sampleUnitsConfig() UnitsConfig {
	return UnitsConfig{
		CustomerManaged: CustomerManagedConfig{
			Enabled:        true,
			RepositoryName: "customer-overlay",
			RepositoryURL:  "https://git.example.com/overlay.git",
			Branch:         "main",
			Interval:       "5m",
			FluxNamePrefix: "cust",
			SecretName:     "customer-git-auth",
			EmitSecret:     true,
			Kustomizations: []CustomerManagedKustomization{
				{Name: "base", Path: "./base"},
				{Name: "extra", Path: "./extra", DependsOn: []string{"base"}},
			},
		},
		SOPS: SOPSGenerationConfig{
			Enabled: true,
			Rules: []SOPSGenerationRule{
				{
					PathRegex:      ".+\\.yaml$",
					AgeRecipients:  []string{"age1abc", "age1def"},
					EncryptedRegex: "^(data|string):",
				},
			},
		},
	}
}

func sampleSecrets() Secrets {
	return Secrets{
		CustomerManaged: CustomerManagedSecrets{
			Identity:    "private-key-material",
			IdentityPub: "public-key-material",
			KnownHosts:  "git.example.com ssh-ed25519 AAAA...",
		},
	}
}

// stableYAML marshals v and returns the canonical bytes used for comparisons.
func stableYAML(t *testing.T, v any) []byte {
	t.Helper()
	data, err := yaml.Marshal(v)
	if err != nil {
		t.Fatalf("yaml.Marshal: %v", err)
	}
	return data
}

func TestUnitsConfig_YAMLRoundTrip(t *testing.T) {
	original := sampleUnitsConfig()

	var decoded UnitsConfig
	if err := yaml.Unmarshal(stableYAML(t, &original), &decoded); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}

	if !bytes.Equal(stableYAML(t, &decoded), stableYAML(t, &original)) {
		t.Fatalf("YAML not stable across round trip:\nfirst: %s\nsecond: %s", stableYAML(t, &original), stableYAML(t, &decoded))
	}

	var diff []string
	if decoded.CustomerManaged.Enabled != original.CustomerManaged.Enabled ||
		decoded.CustomerManaged.RepositoryName != original.CustomerManaged.RepositoryName ||
		decoded.CustomerManaged.RepositoryURL != original.CustomerManaged.RepositoryURL ||
		decoded.CustomerManaged.Branch != original.CustomerManaged.Branch ||
		decoded.CustomerManaged.Interval != original.CustomerManaged.Interval ||
		decoded.CustomerManaged.FluxNamePrefix != original.CustomerManaged.FluxNamePrefix ||
		decoded.CustomerManaged.SecretName != original.CustomerManaged.SecretName ||
		decoded.CustomerManaged.EmitSecret != original.CustomerManaged.EmitSecret {
		diff = append(diff, "CustomerManaged scalar fields differ")
	}
	if !reflect.DeepEqual(decoded.CustomerManaged.Kustomizations, original.CustomerManaged.Kustomizations) {
		diff = append(diff, fmt.Sprintf("Kustomizations: want %+v got %+v", original.CustomerManaged.Kustomizations, decoded.CustomerManaged.Kustomizations))
	}
	if !reflect.DeepEqual(decoded.SOPS.Rules, original.SOPS.Rules) {
		diff = append(diff, fmt.Sprintf("SOPS rules: want %+v got %+v", original.SOPS.Rules, decoded.SOPS.Rules))
	}
	if decoded.SOPS.Enabled != original.SOPS.Enabled {
		diff = append(diff, "SOPS.Enabled differs")
	}
	if len(diff) > 0 {
		t.Fatalf("decoded != original: %s", strings.Join(diff, "; "))
	}
}

func TestSecrets_YAMLRoundTrip(t *testing.T) {
	original := sampleSecrets()

	data, err := yaml.Marshal(&original)
	if err != nil {
		t.Fatalf("yaml.Marshal: %v", err)
	}
	var decoded Secrets
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}
	if decoded != original {
		t.Fatalf("decoded != original:\noriginal: %+v\ndecoded: %+v", original, decoded)
	}
}

func TestUnitsConfig_JSONRoundTrip(t *testing.T) {
	original := sampleUnitsConfig()

	data, err := json.Marshal(&original)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var decoded UnitsConfig
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if want, got := original.CustomerManaged.RepositoryURL, decoded.CustomerManaged.RepositoryURL; want != got {
		t.Fatalf("RepositoryURL: want %q got %q", want, got)
	}
	if len(decoded.CustomerManaged.Kustomizations) != 2 {
		t.Fatalf("expected 2 kustomizations, got %d", len(decoded.CustomerManaged.Kustomizations))
	}
	if decoded.CustomerManaged.Kustomizations[1].DependsOn[0] != "base" {
		t.Fatalf("unexpected DependsOn: %v", decoded.CustomerManaged.Kustomizations[1].DependsOn)
	}
	if len(decoded.SOPS.Rules) != 1 || decoded.SOPS.Rules[0].AgeRecipients[0] != "age1abc" {
		t.Fatalf("unexpected SOPS rules: %+v", decoded.SOPS.Rules)
	}
}

func TestUnitsConfig_ZeroValueOmitsFields(t *testing.T) {
	var empty UnitsConfig

	data, err := yaml.Marshal(&empty)
	if err != nil {
		t.Fatalf("yaml.Marshal: %v", err)
	}
	text := string(data)
	for _, field := range []string{"customer_managed", "sops"} {
		if strings.Contains(text, field) {
			t.Fatalf("expected %q omitted for zero-value UnitsConfig, got:\n%s", field, text)
		}
	}
}

func TestUnitsConfig_DisabledSectionOmitsNestedFields(t *testing.T) {
	cfg := UnitsConfig{
		CustomerManaged: CustomerManagedConfig{Enabled: false},
	}

	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("yaml.Marshal: %v", err)
	}

	var decoded map[string]any
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("yaml.Unmarshal into map: %v", err)
	}
	if cm, present := decoded["customer_managed"]; present {
		mapping, isMap := cm.(map[string]any)
		if !isMap || len(mapping) != 0 {
			t.Fatalf("expected customer_managed to be absent or an empty mapping, got %v (%T)", cm, cm)
		}
	}
}

func TestProperty_UnitsConfig_YAMLRoundTrip(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	type overlaySample struct {
		Enabled   bool
		Name      string
		RepoURL   string
		Branch    string
		Interval  string
		Prefix    string
		Secret    string
		Emit      bool
		Age1      string
		Age2      string
		PathRegex string
	}

	properties := gopter.NewProperties(parameters)
	properties.Property("UnitsConfig survives a YAML round trip", prop.ForAll(
		func(s overlaySample) bool {
			cfg := UnitsConfig{
				CustomerManaged: CustomerManagedConfig{
					Enabled:        s.Enabled,
					RepositoryName: s.Name,
					RepositoryURL:  s.RepoURL,
					Branch:         s.Branch,
					Interval:       s.Interval,
					FluxNamePrefix: s.Prefix,
					SecretName:     s.Secret,
					EmitSecret:     s.Emit,
					Kustomizations: []CustomerManagedKustomization{
						{Name: s.Name, Path: "./" + s.Branch, DependsOn: []string{s.Prefix}},
					},
				},
				SOPS: SOPSGenerationConfig{
					Enabled: s.Enabled,
					Rules: []SOPSGenerationRule{
						{PathRegex: s.PathRegex, AgeRecipients: []string{s.Age1, s.Age2}, EncryptedRegex: "^(data|string):"},
					},
				},
			}

			data, err := yaml.Marshal(&cfg)
			if err != nil {
				return false
			}
			var decoded UnitsConfig
			if err := yaml.Unmarshal(data, &decoded); err != nil {
				return false
			}
			return reflect.DeepEqual(decoded, cfg)
		},
		gen.Struct(reflect.TypeOf(overlaySample{}), map[string]gopter.Gen{
			"Enabled":   gen.Bool(),
			"Name":      gen.AlphaString(),
			"RepoURL":   gen.AlphaString(),
			"Branch":    gen.AlphaString(),
			"Interval":  gen.AlphaString(),
			"Prefix":    gen.AlphaString(),
			"Secret":    gen.AlphaString(),
			"Emit":      gen.Bool(),
			"Age1":      gen.AlphaString(),
			"Age2":      gen.AlphaString(),
			"PathRegex": gen.AlphaString(),
		}),
	))

	properties.TestingRun(t)
}
