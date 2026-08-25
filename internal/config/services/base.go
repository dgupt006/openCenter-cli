package services

// AdoptionMode defines how Flux interacts with a service that may already exist in the cluster.
type AdoptionMode string

const (
	// AdoptionModeManaged means Flux fully manages the service (default behavior).
	AdoptionModeManaged AdoptionMode = "managed"

	// AdoptionModeExternal means the service exists outside of Flux management.
	AdoptionModeExternal AdoptionMode = "external"

	// AdoptionModeSync means Flux renders manifests but won't force changes.
	AdoptionModeSync AdoptionMode = "sync"

	// AdoptionModeDeferred means Flux renders manifests but suspends the Kustomization.
	AdoptionModeDeferred AdoptionMode = "deferred"

	// AdoptionModeTakeover means Flux will take over management of an existing service.
	AdoptionModeTakeover AdoptionMode = "takeover"
)

// ConditionalDependency is a dependsOn entry included only when a gate service is enabled.
type ConditionalDependency struct {
	// Name is the Kustomization name to depend on (e.g. "kube-prometheus-stack-base").
	Name string `yaml:"name" json:"name"`
	// WhenEnabled is the service name that must be enabled for this dependency to apply.
	WhenEnabled string `yaml:"when_enabled" json:"when_enabled"`
}

// ServiceSource describes where the GitOps manifests for a service come from.
type ServiceSource struct {
	Repo    string `yaml:"repo,omitempty" json:"repo,omitempty" jsonschema:"description=GitOps source repository URL"`
	Branch  string `yaml:"branch,omitempty" json:"branch,omitempty" jsonschema:"description=Git branch to track"`
	Release string `yaml:"release,omitempty" json:"release,omitempty" jsonschema:"description=Pinned release tag (mutually exclusive with branch)"`
}

// ServiceImage describes the container image for a service.
type ServiceImage struct {
	Repository string `yaml:"repository,omitempty" json:"repository,omitempty" jsonschema:"description=Container image repository"`
	Tag        string `yaml:"tag,omitempty" json:"tag,omitempty" jsonschema:"description=Container image tag"`
}

// BaseConfig contains common fields for all services.
// Service-specific configs embed this via yaml:",inline".
//
// Design decisions (clean break from v1 flat layout):
//   - `status` removed: runtime state does not belong in declarative config.
//   - `hostname` / `uri` removed from base: derivable from cluster FQDN + service name;
//     services that genuinely need a custom hostname declare it in their own config section.
//   - `source` groups all GitOps source fields into a nested object.
//   - `image` groups container image fields into a nested object.
//   - `adoption_mode` constrained to a known enum.
//
// Rendering fields (Option D: auto-descriptor generation):
//   - Services without an explicit descriptor YAML file get a computed descriptor
//     from these fields. This eliminates per-service boilerplate for standard services.
//   - Complex services (keycloak, cert-manager) keep explicit descriptors.
type BaseConfig struct {
	Enabled      bool          `yaml:"enabled" json:"enabled" jsonschema:"description=Whether this service is deployed"`
	AdoptionMode AdoptionMode  `yaml:"adoption_mode,omitempty" json:"adoption_mode,omitempty" jsonschema:"description=How Flux interacts with this service,enum=managed,enum=external,enum=sync,enum=deferred,enum=takeover,default=managed"`
	Namespace    string        `yaml:"namespace,omitempty" json:"namespace,omitempty" jsonschema:"description=Kubernetes namespace for the service"`
	Source       ServiceSource `yaml:"source,omitempty" json:"source,omitempty" jsonschema:"description=GitOps source configuration"`
	Image        ServiceImage  `yaml:"image,omitempty" json:"image,omitempty" jsonschema:"description=Container image configuration"`

	// Rendering fields — drive auto-descriptor generation for standard services.

	// Edition selects the base path variant (community or enterprise).
	// Empty means no edition suffix on the base path.
	Edition string `yaml:"edition,omitempty" json:"edition,omitempty" jsonschema:"description=Edition variant for base path selection (community or enterprise),enum=community,enum=enterprise"`

	// SourceName overrides the GitRepository name. Default: opencenter-<service-name>.
	// Use this when multiple services share a single source (e.g. observability sub-services).
	SourceName string `yaml:"source_name,omitempty" json:"source_name,omitempty" jsonschema:"description=Override GitRepository source name (default: opencenter-<service>)"`

	// SingleStage renders only an overlay Kustomization (no base stage).
	// Used for services like gateway that have no base in gitops-base.
	SingleStage bool `yaml:"single_stage,omitempty" json:"single_stage,omitempty" jsonschema:"description=Render only overlay stage (no base Kustomization)"`

	// HasOverrideValues controls whether a secretGenerator for override-values is emitted.
	// Default is true when nil (pointer semantics for explicit false).
	HasOverrideValues *bool `yaml:"has_override_values,omitempty" json:"has_override_values,omitempty" jsonschema:"description=Emit secretGenerator for helm override values (default true)"`

	// EnterpriseRegistry adds enterprise OCI registry credential resources.
	EnterpriseRegistry bool `yaml:"enterprise_registry,omitempty" json:"enterprise_registry,omitempty" jsonschema:"description=Include enterprise registry credential resources"`

	// GeneratedResourceFiles adds entries to the generated overlay kustomization
	// resources list. It neither creates nor preserves the referenced files.
	GeneratedResourceFiles []string `yaml:"custom_resources,omitempty" json:"custom_resources,omitempty" jsonschema:"description=Additional resource files for the overlay kustomization.yaml"`

	// ExtraDependencies lists additional FluxCD Kustomization dependsOn entries for the base stage.
	ExtraDependencies []string `yaml:"extra_dependencies,omitempty" json:"extra_dependencies,omitempty" jsonschema:"description=Additional dependsOn entries for the base Kustomization"`

	// ConditionalDependencies lists dependsOn entries that are only included when another service is enabled.
	ConditionalDependencies []ConditionalDependency `yaml:"conditional_dependencies,omitempty" json:"conditional_dependencies,omitempty" jsonschema:"description=Dependencies gated on another service being enabled"`

	// BaseOnly renders only the base Kustomization (no override stage, no overlay directory).
	// Used for services deployed purely from gitops-base with no cluster-specific config.
	BaseOnly bool `yaml:"base_only,omitempty" json:"base_only,omitempty" jsonschema:"description=Render only base stage (no override Kustomization or overlay directory)"`

	// KustomizationName overrides the FluxCD Kustomization name prefix.
	// Default is <service-name> (producing <service>-base and <service>-override).
	// Use when other services depend on a non-standard name (e.g. "envoy-gateway-api").
	KustomizationName string `yaml:"kustomization_name,omitempty" json:"kustomization_name,omitempty" jsonschema:"description=Override FluxCD Kustomization name prefix (default: service name)"`

	// OverrideDependsOn overrides the default dependsOn for the override stage.
	// Default is [<service>-base]. Use this when the override depends on other services.
	OverrideDependsOn []string `yaml:"override_depends_on,omitempty" json:"override_depends_on,omitempty" jsonschema:"description=Override dependsOn for the override Kustomization (default: [service-base])"`

	// OverrideValues provides inline content for the override-values.yaml file.
	// When empty, a placeholder is generated. Use this for services with static override values.
	OverrideValues string `yaml:"override_values,omitempty" json:"override_values,omitempty" jsonschema:"description=Inline content for helm-values/override-values.yaml"`

	// OverrideValuesRendererKey names a registered renderer function that produces
	// dynamic override-values content from the cluster config at render time.
	// Takes precedence over OverrideValues when set.
	OverrideValuesRendererKey string `yaml:"override_values_renderer,omitempty" json:"override_values_renderer,omitempty" jsonschema:"description=Registered renderer key for dynamic override-values generation"`

	// KustomizationContent provides verbatim content for the overlay kustomization.yaml.
	// When set, replaces the auto-generated secretGenerator-based kustomization.
	// Use for services with non-standard kustomization (relative paths, patchesStrategicMerge, etc.).
	KustomizationContent string `yaml:"kustomization_content,omitempty" json:"kustomization_content,omitempty" jsonschema:"description=Verbatim overlay kustomization.yaml content"`

	// OverlayFilesRendererKey names a registered renderer that produces additional
	// overlay files (map[filename]content) from the cluster config at render time.
	// Use for services with Go-templated overlay files (gateway, longhorn).
	OverlayFilesRendererKey string `yaml:"overlay_files_renderer,omitempty" json:"overlay_files_renderer,omitempty" jsonschema:"description=Registered renderer key for dynamic overlay file generation"`
}

// GetHasOverrideValues returns whether override values should be emitted.
// Returns true if not explicitly set (default behavior).
func (b BaseConfig) GetHasOverrideValues() bool {
	if b.HasOverrideValues == nil {
		return true
	}
	return *b.HasOverrideValues
}

// GetSourceName returns the GitRepository source name for this service.
// Falls back to opencenter-<serviceName> if not set.
func (b BaseConfig) GetSourceName(serviceName string) string {
	if b.SourceName != "" {
		return b.SourceName
	}
	return "opencenter-" + serviceName
}

// GetBasePath returns the base path in gitops-base for this service.
// Accounts for edition suffix and shared source grouping.
func (b BaseConfig) GetBasePath(serviceName string) string {
	base := "applications/base/services/" + serviceName
	// Services with a shared source (e.g. observability) use a grouped path
	if b.SourceName != "" && b.SourceName != "opencenter-"+serviceName {
		// e.g. opencenter-observability → applications/base/services/observability/<service>
		groupName := b.SourceName
		if len(groupName) > len("opencenter-") {
			groupName = groupName[len("opencenter-"):]
		}
		base = "applications/base/services/" + groupName + "/" + serviceName
	}
	if b.Edition != "" {
		return base + "/" + b.Edition
	}
	return base
}

// IsEnabled returns true if the service is enabled.
func (b BaseConfig) IsEnabled() bool {
	return b.Enabled
}

// GetStatus is retained for interface compatibility but always returns empty.
// Status is no longer stored in declarative config.
func (b BaseConfig) GetStatus() string {
	return ""
}

// GetAdoptionMode returns the adoption mode of the service.
// Returns AdoptionModeManaged if not set (default behavior).
func (b BaseConfig) GetAdoptionMode() AdoptionMode {
	if b.AdoptionMode == "" {
		return AdoptionModeManaged
	}
	return b.AdoptionMode
}

// IsExternal returns true if the service is externally managed (not rendered by Flux).
func (b BaseConfig) IsExternal() bool {
	return b.GetAdoptionMode() == AdoptionModeExternal
}
