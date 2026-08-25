package v2

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/defaults"
	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	"gopkg.in/yaml.v3"
)

func TestSanitizeLegacyRenderMetadataRemovesAllRecognizedKeys(t *testing.T) {
	input := []byte(`schema_version: "2.0"
opencenter:
  services:
    alpha:
      edition: enterprise
      enterprise_registry: true
      source_name: [one, two]
      single_stage: true
      has_override_values: false
      custom_resources: [resource.yaml]
      extra_dependencies: [network]
      conditional_dependencies: [{name: network, when_enabled: cni}]
      base_only: false
      kustomization_name: custom
      override_depends_on: [sources]
      override_values: |
        arbitrary: value
      override_values_renderer:
        arbitrary:
          payload: true
      kustomization_content: generated
      overlay_files_renderer: 42
  managed_services:
    beta:
      base_only: false
  managed-service:
    gamma:
      overlay_files_renderer: 42
`)

	got, warnings, err := SanitizeLegacyRenderMetadata(input)
	if err != nil {
		t.Fatalf("SanitizeLegacyRenderMetadata() error = %v", err)
	}

	wantPaths := []string{
		"opencenter.services.alpha.base_only",
		"opencenter.services.alpha.conditional_dependencies",
		"opencenter.services.alpha.custom_resources",
		"opencenter.services.alpha.edition",
		"opencenter.services.alpha.enterprise_registry",
		"opencenter.services.alpha.extra_dependencies",
		"opencenter.services.alpha.has_override_values",
		"opencenter.services.alpha.kustomization_content",
		"opencenter.services.alpha.kustomization_name",
		"opencenter.services.alpha.overlay_files_renderer",
		"opencenter.services.alpha.override_depends_on",
		"opencenter.services.alpha.override_values",
		"opencenter.services.alpha.override_values_renderer",
		"opencenter.services.alpha.single_stage",
		"opencenter.services.alpha.source_name",
		"opencenter.managed_services.beta.base_only",
		"opencenter.managed-service.gamma.overlay_files_renderer",
	}
	if len(warnings) != len(wantPaths) {
		t.Fatalf("warning count = %d, want %d: %+v", len(warnings), len(wantPaths), warnings)
	}
	for i, want := range wantPaths {
		if warnings[i].Path != want {
			t.Errorf("warning[%d].Path = %q, want %q", i, warnings[i].Path, want)
		}
	}

	output := string(got)
	for _, key := range []string{"edition:", "enterprise_registry:", "source_name:", "single_stage:", "has_override_values:", "custom_resources:", "extra_dependencies:", "conditional_dependencies:", "base_only:", "kustomization_name:", "override_depends_on:", "override_values:", "override_values_renderer:", "kustomization_content:", "overlay_files_renderer:"} {
		if strings.Contains(output, key) {
			t.Errorf("sanitized YAML still contains %q:\n%s", key, output)
		}
	}
	for _, value := range []string{"arbitrary:", "payload:", "one", "42"} {
		if strings.Contains(output, value) {
			t.Errorf("sanitized YAML still contains renderer payload %q:\n%s", value, output)
		}
	}
}

func TestConfigLoaderLegacyRenderMetadataIsIgnoredBeforeStrictDecode(t *testing.T) {
	cfg := newValidV2TestConfig("openstack")
	cfg.OpenCenter.Services = ServiceMap{
		"olm": &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: true}},
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("yaml.Marshal() error = %v", err)
	}

	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}
	service := mappingValueForPublicTest(mappingValueForPublicTest(mappingValueForPublicTest(&document, "opencenter"), "services"), "olm")
	service.Content = append(service.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "override_values_renderer"},
		&yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "renderer"},
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "arbitrary"},
		}},
	)
	var raw bytes.Buffer
	encoder := yaml.NewEncoder(&raw)
	if err := encoder.Encode(&document); err != nil {
		t.Fatalf("encode YAML: %v", err)
	}
	if err := encoder.Close(); err != nil {
		t.Fatalf("close YAML encoder: %v", err)
	}

	loaded, err := NewConfigLoader(defaultsForPublicTest()).LoadFromBytes(raw.Bytes())
	if err != nil {
		t.Fatalf("LoadFromBytes() rejected recognized legacy metadata: %v", err)
	}
	if loaded.OpenCenter.Services["olm"] == nil {
		t.Fatal("expected service to remain loaded")
	}
}

func TestConfigLoaderUnknownServiceKeyStillFails(t *testing.T) {
	cfg := newValidV2TestConfig("openstack")
	cfg.OpenCenter.Services = ServiceMap{
		"olm": &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: true}},
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("yaml.Marshal() error = %v", err)
	}

	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}
	service := mappingValueForPublicTest(mappingValueForPublicTest(mappingValueForPublicTest(&document, "opencenter"), "services"), "olm")
	service.Content = append(service.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "unknown_service_key"},
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "must-fail"},
	)
	var raw bytes.Buffer
	encoder := yaml.NewEncoder(&raw)
	if err := encoder.Encode(&document); err != nil {
		t.Fatalf("encode YAML: %v", err)
	}
	if err := encoder.Close(); err != nil {
		t.Fatalf("close YAML encoder: %v", err)
	}

	if _, err := NewConfigLoader(defaultsForPublicTest()).LoadFromBytes(raw.Bytes()); err == nil || !strings.Contains(err.Error(), "unknown_service_key") {
		t.Fatalf("LoadFromBytes() error = %v, want unknown key rejection", err)
	}
}

func TestMarshalPublicConfigOmitsLegacyRenderMetadataAndIsIdempotent(t *testing.T) {
	cfg := newValidV2TestConfig("openstack")
	cfg.OpenCenter.Services = ServiceMap{
		"olm": &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{
			Enabled: true,
		}},
	}

	first, err := MarshalPublicConfig(cfg)
	if err != nil {
		t.Fatalf("MarshalPublicConfig() error = %v", err)
	}
	second, err := SanitizePublicYAML(first)
	if err != nil {
		t.Fatalf("SanitizePublicYAML() error = %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("public serialization is not idempotent:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
	for _, key := range []string{"edition:", "enterprise_registry:", "source_name:", "single_stage:", "has_override_values:", "custom_resources:", "extra_dependencies:", "conditional_dependencies:", "base_only:", "kustomization_name:", "override_depends_on:", "override_values:", "override_values_renderer:", "kustomization_content:", "overlay_files_renderer:"} {
		if strings.Contains(string(first), key) {
			t.Errorf("public YAML contains internal metadata %q:\n%s", key, first)
		}
	}
}

func TestSaveAndExportUsePublicSerialization(t *testing.T) {
	loader := NewConfigLoader(defaultsForPublicTest())
	cfg := newValidV2TestConfig("openstack")
	cfg.OpenCenter.Services = ServiceMap{
		"olm": &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: true}},
	}

	path := t.TempDir() + "/config.yaml"
	if err := loader.SaveToFile(cfg, path); err != nil {
		t.Fatalf("SaveToFile() error = %v", err)
	}
	saved, err := loader.fileSystem.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	if strings.Contains(string(saved), "source_name:") {
		t.Fatalf("saved config contains internal metadata:\n%s", saved)
	}

	exported, err := loader.ExportEffectiveConfig(cfg)
	if err != nil {
		t.Fatalf("ExportEffectiveConfig() error = %v", err)
	}
	if strings.Contains(string(exported), "source_name:") {
		t.Fatalf("exported config contains internal metadata:\n%s", exported)
	}
}

func TestPublicSerializationCleansDefaultAndFullTemplates(t *testing.T) {
	defaultConfig, err := NewV2Default("default-output", "openstack")
	if err != nil {
		t.Fatalf("NewV2Default() error = %v", err)
	}
	defaultOutput, err := MarshalPublicConfig(defaultConfig)
	if err != nil {
		t.Fatalf("MarshalPublicConfig(default) error = %v", err)
	}

	fullOutput, err := RenderFullTemplateYAMLFromConfig(defaultConfig)
	if err != nil {
		t.Fatalf("RenderFullTemplateYAMLFromConfig() error = %v", err)
	}
	fullOutput, err = SanitizePublicYAML(fullOutput)
	if err != nil {
		t.Fatalf("SanitizePublicYAML(full) error = %v", err)
	}

	for name, output := range map[string][]byte{
		"default": defaultOutput,
		"full":    fullOutput,
	} {
		for _, key := range []string{
			"edition:",
			"enterprise_registry:",
			"source_name:",
			"single_stage:",
			"has_override_values:",
			"base_only:",
			"kustomization_name:",
			"override_depends_on:",
			"override_values_renderer:",
			"kustomization_content:",
			"overlay_files_renderer:",
			"custom_resources:",
		} {
			if strings.Contains(string(output), key) {
				t.Errorf("%s output contains internal metadata %q", name, key)
			}
		}
	}
}

func defaultsForPublicTest() defaults.Registry {
	return defaults.NewRegistry()
}

func mappingValueForPublicTest(node *yaml.Node, key string) *yaml.Node {
	if node == nil {
		return nil
	}
	if node.Kind == yaml.DocumentNode {
		if len(node.Content) == 0 {
			return nil
		}
		return mappingValueForPublicTest(node.Content[0], key)
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

type legacyMetadataServiceConfig struct {
	services.BaseConfig    `yaml:",inline"`
	OverrideValuesRenderer string `yaml:"override_values_renderer,omitempty" json:"override_values_renderer,omitempty"`
}

func TestDecodePublicConfigWarnsAndStripsLegacyMetadata(t *testing.T) {
	input := []byte(`schema_version: "2.0"
opencenter:
  meta:
    name: public-decode
    organization: acme
    env: dev
    region: dfw3
  services:
    olm:
      enabled: true
      override_values_renderer: legacy
`)

	oldStderr := os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stderr = writer
	cfg, decodeErr := DecodePublicConfig(input)
	_ = writer.Close()
	os.Stderr = oldStderr
	warningOutput, _ := io.ReadAll(reader)
	_ = reader.Close()

	if decodeErr != nil {
		t.Fatalf("DecodePublicConfig() error = %v", decodeErr)
	}
	if cfg.OpenCenter.Meta.Name != "public-decode" {
		t.Fatalf("decoded name = %q, want public-decode", cfg.OpenCenter.Meta.Name)
	}
	if !strings.Contains(string(warningOutput), "opencenter.services.olm.override_values_renderer is deprecated") {
		t.Fatalf("warning output = %q, want legacy metadata warning", warningOutput)
	}
	if service, ok := cfg.OpenCenter.Services["olm"]; !ok || service == nil {
		t.Fatalf("decoded service = %#v, want enabled olm service", service)
	}
}

func TestDecodePublicConfigRejectsUnknownServiceKey(t *testing.T) {
	input := []byte(`schema_version: "2.0"
opencenter:
  services:
    olm:
      enabled: true
      unknown_service_key: must-fail
`)

	if _, err := DecodePublicConfig(input); err == nil || !strings.Contains(err.Error(), "unknown_service_key") {
		t.Fatalf("DecodePublicConfig() error = %v, want unknown service key rejection", err)
	}
}

func TestToJSONUsesPublicSerialization(t *testing.T) {
	cfg := newValidV2TestConfig("openstack")
	cfg.OpenCenter.Services = ServiceMap{
		"olm": &legacyMetadataServiceConfig{BaseConfig: services.BaseConfig{Enabled: true}, OverrideValuesRenderer: "legacy"},
	}
	cfg.OpenCenter.LegacyTalos = map[string]any{"cluster_name": "legacy-only"}

	data, err := cfg.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() error = %v", err)
	}
	if strings.Contains(string(data), "override_values_renderer") {
		t.Fatalf("ToJSON() contains internal metadata: %s", data)
	}
	if strings.Contains(string(data), "\"talos\"") {
		t.Fatalf("ToJSON() contains json-internal LegacyTalos: %s", data)
	}

	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("json.Unmarshal(ToJSON()) error = %v", err)
	}
	if got := document["schema_version"]; got != "2.0" {
		t.Fatalf("schema_version = %v, want 2.0", got)
	}
	opencenter, ok := document["opencenter"].(map[string]any)
	if !ok {
		t.Fatalf("opencenter = %T, want object", document["opencenter"])
	}
	serviceMap, ok := opencenter["services"].(map[string]any)
	if !ok {
		t.Fatalf("services = %T, want object", opencenter["services"])
	}
	olm, ok := serviceMap["olm"].(map[string]any)
	if !ok {
		t.Fatalf("olm = %T, want object", serviceMap["olm"])
	}
	if got := olm["enabled"]; got != true {
		t.Fatalf("olm.enabled = %v, want true", got)
	}
}
