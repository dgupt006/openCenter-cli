---
id: adding-services
title: "Adding New Platform Services"
sidebar_label: Adding New Platform Services
description: How to add new platform services to openCenter-cli using auto-descriptors or explicit templates.
doc_type: how-to
audience: "developers, platform engineers"
tags: [services, rendering, gitops, auto-descriptor]
---
# Adding New Platform Services

**Purpose:** For developers, shows how to add new platform services to openCenter-cli.

## Prerequisites

* Development environment set up (see [Development Setup](development-setup.md))
* Service’s Helm chart already added to `openCenter-gitops-base` under `applications/base/services/<service>/`

## Quick Path: Standard Services (Auto-Descriptor)

Most services follow the standard two-stage FluxCD pattern. For these, adding a service requires **only configuration changes** -- no templates, no descriptor files, no Go code beyond a config struct.

### Step 1: Register the Service Config Type

If the service has no custom fields beyond `BaseConfig`, it’s already registered via `DefaultServiceConfig` in `internal/config/services/default_services.go`. Just add the name:

```go
defaults := []string{
    // ... existing services
    "my-service",
}
```

If the service needs custom fields (storage type, credentials, etc.), create a typed config:

```go
// internal/config/services/my_service.go
package services

type MyServiceConfig struct {
    BaseConfig `yaml:",inline"`
    BucketName string `yaml:"bucket_name,omitempty" json:"bucket_name,omitempty"`
}

func init() {
    registry.RegisterServiceConfig("my-service", MyServiceConfig{})
}
```

### Step 2: Set Defaults

Add the service to `internal/config/v2/defaults.go` in `defaultServiceMap()`:

```go
"my-service": &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{
    Enabled:   true,
    Namespace: "my-service",
}},
```

That’s it. The auto-descriptor engine generates:

* `services/sources/opencenter-my-service.yaml` (GitRepository)
* `services/fluxcd/my-service.yaml` (two-stage Kustomization)
* `services/my-service/kustomization.yaml` (overlay with secretGenerator)
* `services/my-service/helm-values/override-values.yaml` (placeholder)
* `services/my-service/custom/` (user-owned, seeded once, never overwritten)
* Entries in aggregate `kustomization.yaml` files
* An entry per generated file in the overlay's `.opencenter-generated.json` manifest

### Step 3: Regenerate Schema

```bash
mise run schema-v2
```

The regeneration test file is created on demand by the mise task, so
`go test -run TestRegenSchema` on its own will not find anything to run.

### BaseConfig Rendering Fields

Control rendering behavior via `BaseConfig` fields. These are set in Go, in
`defaultServiceMap()` -- they are contributor surface, not user surface.

| Field | Default | Use When | YAML key |
| --- | --- | --- | --- |
| `Namespace` | (required) | Always set -- target namespace | supported |
| `Edition` | `""` | Service has community/enterprise variants in gitops-base | supported |
| `ExtraDependencies` | `[]` | Additional dependsOn for the base stage | supported |
| `ConditionalDependencies` | `[]` | Dependencies gated on another service being enabled | supported |
| `OverrideValues` | `""` | Inline override-values content (default: empty placeholder) | supported |
| `EnterpriseRegistry` | `false` | Service needs enterprise OCI registry credentials | supported |
| `GeneratedResourceFiles` | `[]` | Extra **generator-owned** files a renderer emits into the overlay | deprecated |
| `SourceName` | `opencenter-<name>` | Multiple services share one GitRepository (e.g. observability) | deprecated |
| `SingleStage` | `false` | Service has no base in gitops-base (overlay-only) | deprecated |
| `BaseOnly` | `false` | Service needs no cluster-specific overlay | deprecated |
| `HasOverrideValues` | `true` (nil) | Set `false` to skip secretGenerator | deprecated |
| `OverrideDependsOn` | `[]` | Override stage dependsOn (default: `[<service>-base]`) | deprecated |

`GeneratedResourceFiles` was previously named `CustomResources`. The rename is
deliberate: the field only adds `resources:` entries to the generated overlay
kustomization. It does **not** create those files and does **not** protect them
from deletion. Every filename listed here must be emitted by a renderer in the
same pass, otherwise `kustomize build` fails after regeneration. Prefer letting
the renderer report its own emitted files rather than listing them here.

To keep hand-authored manifests in an overlay, put them in the service's
`custom/` directory. That directory is user-owned: the generator never writes to
it, never deletes from it, and includes it in the overlay kustomization.

#### On the "deprecated" column

The `deprecated` marker applies to the **YAML key only**. The Go fields remain
fully supported and are the intended way to configure a service you are adding.
Setting the corresponding key in a cluster config marks it `deprecated: true` in
the JSON schema (an editor hint; validation still passes) and prints a warning to
stderr. The registry lives in `internal/config/services/deprecations.go`.

These keys are deprecated at the YAML layer because they select internal
rendering topology -- which Flux template is used, which GitRepository is
referenced, whether an override secret is emitted -- and the only correct value
is the one that matches what exists in openCenter-gitops-base. Overriding them
per cluster produces a broken render, not a customization.

### Examples

**Observability sub-service (shared source):**

```go
"mimir": &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{
    Enabled:    false,
    Namespace:  "observability",
    SourceName: "opencenter-observability",
}},
```

**Base-only service (no cluster customization):**

```go
"external-snapshotter": &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{
    Enabled:  true,
    Namespace: "external-snapshotter",
    BaseOnly: true,
}},
```

**Single-stage service (overlay-only, no base):**

```go
"gateway": &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{
    Enabled:           true,
    Namespace:         "gateway",
    SingleStage:       true,
    ExtraDependencies: []string{"envoy-gateway-api-base"},
}},
```

**Service with conditional dependency:**

```go
"rbac-manager": &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{
    Enabled:   true,
    Namespace: "rbac-system",
    BaseOnly:  true,
    ConditionalDependencies: []services.ConditionalDependency{
        {Name: "kube-prometheus-stack-base", WhenEnabled: "kube-prometheus-stack"},
    },
}},
```

## Complex Services: Explicit Descriptors

Services that need custom rendering logic (multi-component, conditional files, templated override-values, custom renderers) use explicit descriptors.

### When to Use Explicit Descriptors

* Multi-component services (keycloak: 4 sub-stages)
* Services with conditional file rendering (keycloak backup cronjob, region-specific patches)
* Services with templated override-values (loki, tempo, openstack-ccm)
* Services with custom renderers (cert-manager multi-credential DNS)

### Step 1: Create Descriptor

Create `internal/services/descriptors/data/service-<name>.yaml`:

```yaml
name: service-my-complex-service
layer: services
service: my-complex-service
aggregate_targets:
  - services-fluxcd-aggregate
  - services-sources-aggregate
roots:
  - path: services/my-complex-service
files:
  - template: services/sources/opencenter-my-complex-service.yaml.tpl
  - template: services/fluxcd/my-complex-service.yaml.tpl
  - template: services/my-complex-service/conditional-file.yaml.tpl
    when:
      field: opencenter.services.my-complex-service.some_field
      operator: true
```

### Step 2: Create Templates

Create the `.tpl` files referenced by the descriptor under `internal/gitops/templates/cluster-apps-base/`.

### Step 3: Add to Aggregate Lists

Add the service to the hardcoded lists in:

* `services/sources/kustomization.yaml.tpl`
* `services/fluxcd/kustomization.yaml.tpl`

### Step 4: (Optional) Custom Renderer

For services like cert-manager that generate dynamic files based on config:

```go
// internal/gitops/my_service_renderer.go
func renderMyServiceDynamicFiles(cfg v2.Config, targetDir string, workspace *GitOpsWorkspace) error {
    // Generate files based on typed config
}
```

Hook it into `RenderClusterApps` or the service plugin’s `Renderer` function.

## Verification

After adding a service:

```bash
# Build
go build ./...

# Run tests
go test ./internal/gitops/ ./internal/config/... ./internal/services/...

# Regenerate schema
mise run schema-v2

# Test rendering (dry-run)
./bin/opencenter cluster generate <org>/<cluster> --dry-run
```

## Evidence

* Auto-descriptor engine: `internal/gitops/auto_descriptor.go`
* BaseConfig fields: `internal/config/services/base.go`
* Service defaults: `internal/config/v2/defaults.go` → `defaultServiceMap()`
* Explicit descriptors: `internal/services/descriptors/data/`
* Descriptor renderer: `internal/gitops/descriptor_renderer.go`
* Rendering contract: `docs/dev/rendering-contract.md`