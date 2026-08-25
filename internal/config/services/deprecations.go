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

package services

import (
	"fmt"
	"os"
	"sort"

	"gopkg.in/yaml.v3"
)

// DeprecatedConfigKey describes a service configuration key retained for
// compatibility but scheduled for removal in the next major schema version.
type DeprecatedConfigKey struct {
	Key      string
	Reason   string
	Guidance string
}

// deprecatedConfigKeys is the authoritative specification for legacy service
// render metadata. Consumers must use this registry rather than maintaining a
// second list of keys to warn about or remove.
var deprecatedConfigKeys = []DeprecatedConfigKey{
	{
		Key:      "edition",
		Reason:   "it selects an internal base-repository variant",
		Guidance: "let the service descriptor select the supported base path",
	},
	{
		Key:      "enterprise_registry",
		Reason:   "it selects internal enterprise registry resources",
		Guidance: "use supported service configuration for registry credentials",
	},
	{
		Key:      "custom_resources",
		Reason:   "it only adds filenames to the generated kustomization; it does not create or preserve those files",
		Guidance: "put user-owned manifests in the service's custom/ directory instead",
	},
	{
		Key:      "extra_dependencies",
		Reason:   "it changes the internal dependency graph of generated Flux Kustomizations",
		Guidance: "let the service descriptor select generated dependencies",
	},
	{
		Key:      "conditional_dependencies",
		Reason:   "it changes the internal conditional dependency graph of generated Flux Kustomizations",
		Guidance: "let the service descriptor select generated dependencies",
	},
	{
		Key:      "kustomization_content",
		Reason:   "it replaces the generated overlay kustomization with renderer-specific content",
		Guidance: "use the generated overlay and put user-owned manifests in the service's custom/ directory instead",
	},
	{
		Key:      "overlay_files_renderer",
		Reason:   "it selects an internal renderer implementation for generated overlay files",
		Guidance: "use supported typed service configuration or put user-owned manifests in the service's custom/ directory instead",
	},
	{
		Key:      "override_values",
		Reason:   "it provides internal generated Helm override content",
		Guidance: "set supported service values through the service configuration instead",
	},
	{
		Key:      "override_values_renderer",
		Reason:   "it selects an internal renderer implementation for generated Helm override values",
		Guidance: "set supported service values through the service configuration instead",
	},
	{
		Key:      "single_stage",
		Reason:   "it selects an internal Flux rendering topology",
		Guidance: "let the service descriptor select the rendering topology",
	},
	{
		Key:      "base_only",
		Reason:   "it selects an internal Flux rendering topology",
		Guidance: "let the service descriptor select the rendering topology",
	},
	{
		Key:      "source_name",
		Reason:   "it overrides an internal GitRepository name used by generated descriptors",
		Guidance: "use the service's supported source configuration and descriptor defaults",
	},
	{
		Key:      "kustomization_name",
		Reason:   "it overrides an internal Flux Kustomization name used by generated descriptors",
		Guidance: "let the service descriptor select generated Kustomization names",
	},
	{
		Key:      "override_depends_on",
		Reason:   "it overrides the internal dependency graph of generated Flux Kustomizations",
		Guidance: "use service dependency configuration rather than changing generated renderer output",
	},
	{
		Key:      "has_override_values",
		Reason:   "it controls whether an internal generated Helm override secret is emitted",
		Guidance: "set supported service values through the service configuration instead",
	},
}

// DeprecatedServiceConfigKeys returns the ordered service configuration
// deprecation registry. The returned slice is a copy and may be modified by
// the caller without changing the registry.
func DeprecatedServiceConfigKeys() []DeprecatedConfigKey {
	return append([]DeprecatedConfigKey(nil), deprecatedConfigKeys...)
}

// LookupDeprecatedServiceConfigKey looks up a deprecated service configuration
// key by its YAML name.
func LookupDeprecatedServiceConfigKey(key string) (DeprecatedConfigKey, bool) {
	for _, entry := range deprecatedConfigKeys {
		if entry.Key == key {
			return entry, true
		}
	}
	return DeprecatedConfigKey{}, false
}

// DeprecatedConfigWarning describes one explicitly configured deprecated key.
type DeprecatedConfigWarning struct {
	Path  string
	Entry DeprecatedConfigKey
}

// DeprecatedConfigWarnings scans raw v2 YAML for deprecated keys under the
// user-owned services and managed_services maps. It deliberately scans raw
// input before defaults are applied, so built-in defaults do not warn.
func DeprecatedConfigWarnings(data []byte) []DeprecatedConfigWarning {
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil
	}
	root := unwrapDocumentNode(&document)
	if root == nil || root.Kind != yaml.MappingNode {
		return nil
	}

	opencenter := mappingValue(root, "opencenter")
	if opencenter == nil || opencenter.Kind != yaml.MappingNode {
		return nil
	}

	var warnings []DeprecatedConfigWarning
	for _, section := range []string{"services", "managed_services", "managed-service"} {
		sectionNode := mappingValue(opencenter, section)
		if sectionNode == nil || sectionNode.Kind != yaml.MappingNode {
			continue
		}

		serviceNames := make([]string, 0, len(sectionNode.Content)/2)
		serviceNodes := make(map[string]*yaml.Node)
		for i := 0; i+1 < len(sectionNode.Content); i += 2 {
			name := sectionNode.Content[i].Value
			serviceNames = append(serviceNames, name)
			serviceNodes[name] = sectionNode.Content[i+1]
		}
		sort.Strings(serviceNames)

		for _, serviceName := range serviceNames {
			serviceNode := serviceNodes[serviceName]
			if serviceNode == nil || serviceNode.Kind != yaml.MappingNode {
				continue
			}
			for _, entry := range deprecatedConfigKeys {
				if mappingValue(serviceNode, entry.Key) != nil {
					warnings = append(warnings, DeprecatedConfigWarning{
						Path:  fmt.Sprintf("opencenter.%s.%s.%s", section, serviceName, entry.Key),
						Entry: entry,
					})
				}
			}
		}
	}
	return warnings
}

// WarnDeprecatedConfigWarnings writes warnings for deprecated keys explicitly
// present in raw user configuration. Warning output never affects loading.
func WarnDeprecatedConfigWarnings(warnings []DeprecatedConfigWarning) {
	for _, warning := range warnings {
		fmt.Fprintf(os.Stderr, "warning: %s is deprecated, ignored, and rendering is internally owned\n", warning.Path)
		fmt.Fprintf(os.Stderr, "  reason: %s\n", warning.Entry.Reason)
		fmt.Fprintf(os.Stderr, "  guidance: %s\n", warning.Entry.Guidance)
	}
}

// WarnDeprecatedConfigKeys writes warnings for deprecated keys explicitly
// present in raw user configuration. Warning output never affects loading.
func WarnDeprecatedConfigKeys(data []byte) {
	WarnDeprecatedConfigWarnings(DeprecatedConfigWarnings(data))
}

func unwrapDocumentNode(node *yaml.Node) *yaml.Node {
	for node != nil && node.Kind == yaml.DocumentNode {
		if len(node.Content) == 0 {
			return nil
		}
		node = node.Content[0]
	}
	return node
}

func mappingValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}
