---
id: plugin-internal-services
title: "Plugin Internal Services"
sidebar_label: Plugin Internal Services
description: How openCenter CLI internal service plugins work, using cert-manager to explain service behavior and the code required to add a new platform service.
doc_type: explanation
audience: "developers, platform engineers"
tags: [plugins, services, cert-manager, gitops, extensions]
---
# Plugin Internal Services

**Purpose:** For developers and platform engineers, explains the internal service plugin system in openCenter CLI, uses cert-manager as the worked example, and maps the practical code changes required to add a new platform service.

## What This Plugin Mechanism Is

The internal service plugin system is how openCenter models platform services that are:

* configured inside the cluster YAML
* enabled or disabled with `opencenter cluster service ...`
* validated as part of cluster configuration
* rendered into GitOps templates and manifests

Examples include:

* cert-manager
* Loki
* Harbor
* Keycloak
* Velero

This is different from the external CLI plugin mechanism. External CLI plugins add new commands such as `opencenter foo`; they do not participate in service config typing, service validation, or GitOps manifest generation.

For that separate mechanism, see [Plugin External CLI](plugin-external-cli.md).

## The Service Plugin Model

At a high level, a service such as `cert-manager` is not just a Go plugin object. It is the combination of:

1. A typed configuration struct.
2. A config-type registration entry so the CLI can instantiate that struct by service name.
3. A service plugin implementation that provides metadata, validation, status, and an optional render hook.
4. Optional validation-engine extensions.
5. GitOps templates that actually produce the Kubernetes and Flux manifests.
6. CLI wiring for `cluster service enable`, `cluster service options`, and service-specific secrets.

The core contract is the `ServicePlugin` interface with:

* `Name`
* `Type`
* `Validate`
* `Render`
* `Status`

Evidence:

* `internal/services/plugin.go`
* `internal/services/base_plugin.go`
* `internal/services/registry.go`

## Base Plugin Composition

Most built-in services use `BaseServicePlugin` rather than implementing every method from scratch.

`BaseServicePlugin` provides:

* standard metadata storage
* a validator callback
* a renderer callback
* a status callback
* default no-op behavior where needed

The common pattern is:

1. create a base plugin with metadata
2. embed it in a service-specific struct
3. inject service-specific validation/render/status functions

Evidence:

* `internal/services/base_plugin.go`
* `internal/services/plugins/cert_manager.go`

## Cert-Manager Walkthrough

### Configuration Registration

`cert-manager` starts with a typed config struct:

* `CertManagerConfig` embeds `BaseConfig`
* it adds fields such as `letsencrypt_server`, `email`, `region`, `dns_zones`, `issuers`, and `dns_provider`
* its `init()` function registers the config type under the name `cert-manager`

That registration is what lets generic CLI logic look up a service by name and instantiate the correct struct dynamically.

Evidence:

* `internal/config/services/cert_manager.go`
* `internal/config/registry/registry.go`

### Default Behavior

When a new cluster configuration is built, `defaultServiceMap` in `internal/config/v2/defaults.go` (the authoritative v2 defaults, not a top-level `internal/config/defaults.go`, which does not exist) defaults cert-manager into the services map with only:

* `enabled: true`
* `namespace: cert-manager`

`Email`, `LetsEncryptServer`, and `Region` are left empty at this stage; the JSON-schema `default=` annotations on those fields (visible in `internal/config/services/cert_manager.go`) are documentation hints for the generated schema, not values applied to the Go struct. The fallback values operators see in rendered output — ACME server `https://acme-v02.api.letsencrypt.org/directory` and a placeholder support email — are applied later, at render time, by `extractCertManagerConfig` in `internal/gitops/cert_manager_renderer.go`, only when the corresponding field is still empty.

Evidence:

* `internal/config/v2/defaults.go`
* `internal/gitops/cert_manager_renderer.go`

### Enabling the Service from the CLI

When you run:

```bash
opencenter cluster service enable cert-manager --param="email=admin@example.com"
```

the CLI takes a mostly generic path:

1. Look up the service config type in the config registry.
2. Instantiate it with reflection.
3. Set `Enabled = true`.
4. Apply `--param` values by matching CLI keys to JSON tags on struct fields.
5. Apply `--secret` values by routing into the service’s secret struct.
6. Run service-specific checks.
7. Save the updated cluster config.
8. Optionally render just that service with `--render`.

For cert-manager specifically, the CLI also hard-requires `email`.

One important implementation detail: the CLI help for service-specific options and secrets is manually maintained. It is not generated from the service config struct. Adding a new service usually means updating `getServiceOptions`, `getServiceSecrets`, and service-specific validation logic yourself.

Evidence:

* `cmd/cluster_service.go`

## The Cert-Manager Plugin Object

The cert-manager service plugin itself is intentionally small.

`NewCertManagerPlugin()`:

* creates a base plugin with metadata
* injects a cert-manager validator
* injects a cert-manager status function
* injects a render function that is currently just a placeholder

The current cert-manager validator checks only a few service-local rules:

* `letsencrypt_server` must use `https://`
* `email` must contain `@`

The status function reports:

* disabled state when the service is off
* config status when it is enabled
* details such as ACME server and email address

Evidence:

* `internal/services/plugins/cert_manager.go`

## Dependencies and Validation Metadata

Built-in services are also registered with dependency metadata.

For example:

* `cert-manager` has no dependencies
* `keycloak` depends on `cert-manager`
* `harbor` depends on `cert-manager`

That dependency graph can be topologically sorted by the service registry. The registry also hosts service-specific validators such as `service:cert-manager`.

Evidence:

* `internal/services/plugins/registry.go`
* `internal/services/plugins/validators.go`
* `internal/services/registry.go`

## How Rendering Actually Happens Today

This is the most important nuance in the current implementation:

The production `cluster generate` path does not call `ServicePlugin.Render()` for cert-manager. Instead, it renders by walking the embedded GitOps template tree directly.

The current setup flow (see `SetupService.Setup` / `generateGitOpsManifests` in `internal/cluster/setup_service.go`) is:

1. Copy the base GitOps repository structure (`gitops.CopyBase`).
2. Render the cluster application overlays (`gitops.RenderClusterAppsWithEncryption`), which resolves each enabled service either through an explicit descriptor (cert-manager is one) or through the immutable `RenderCatalog` of direct planning/rendering functions (most other built-in services — gateway, kyverno, loki, tempo, velero, metallb, and more — have no `.tpl` directory at all and render entirely from Go code in `internal/gitops/render_catalog.go` and its companion renderer files).
3. Render the infrastructure templates (`gitops.RenderInfrastructureCluster`) and the per-cluster Flux bridge (`gitops.RenderClusterFluxBridge`).
4. Run OpenTofu provisioning when needed (non-Kind providers only).

So although the service plugin has a `Render()` method, cert-manager's actual behavior comes from its explicit descriptor, its `.tpl` files, and the Go code in `internal/gitops/cert_manager_renderer.go` — not from `ServicePlugin.Render()`, which remains an unused placeholder for cert-manager and other built-in services.

Evidence:

* `internal/cluster/setup_service.go`
* `internal/gitops/copy.go`
* `internal/gitops/embed.go`

## Cert-Manager Template Behavior

cert-manager's rendered output comes from two different mechanisms working together, not a single template family:

1. `services/sources/opencenter-cert-manager.yaml.tpl` — the GitRepository source, a static file-walk template rendered through the shared `sourceAuthBlockForService` helper in `internal/gitops/copy.go`. Because `openCenter-gitops-base` is a public repository, this source renders **anonymously** (no `secretRef`); see the correction in [GitOps Workflow](gitops-workflow.md).
2. `services/fluxcd/cert-manager.yaml.tpl` — the two FluxCD `Kustomization` resources (`cert-manager-base`, `cert-manager-override`).
3. `services/cert-manager/rackspace-selfsigned-ca.yaml.tpl`, `rackspace-ca-issuer.yaml`, `rackspace-selfsigned-issuer.yaml`, `helm-values/`, and `README.md` — static/rendered bootstrap files copied by the same file-walk renderer.
4. Everything credential- and issuer-specific — the AWS/Cloudflare/Designate `Secret` and `ClusterIssuer` objects, and the overlay's `kustomization.yaml` itself — is generated by Go code in `internal/gitops/cert_manager_renderer.go` (`planCertManagerDynamicActions`), using Go string templates embedded in that file rather than `.tpl` files on disk. There is no `letsencrypt-issuer.yaml.tpl` or `kustomization.yaml.tpl` under `services/cert-manager/` — those outputs are planned actions, so they participate in the same atomic write/promotion boundary as file-walk output (see [Rendering Ownership and Secret Artifacts](../CODEMAPS/rendering-ownership-and-secret-artifacts.md)).

`planCertManagerDynamicActions` reads `cfg.Secrets.CertManager.AWS` and `cfg.Secrets.CertManager.Cloudflare` (each a *map* of named credentials, not a single credential), plus `DNSProvider: "designate"` on `CertManagerConfig` for OpenStack Designate DNS-01. It emits one `Secret` + one `ClusterIssuer` per named AWS or Cloudflare credential, an optional Designate issuer, and a `kustomization.yaml` that lists exactly the files it just planned.

Evidence:

* `internal/gitops/templates/cluster-apps-base/services/sources/opencenter-cert-manager.yaml.tpl`
* `internal/gitops/templates/cluster-apps-base/services/fluxcd/cert-manager.yaml.tpl`
* `internal/gitops/cert_manager_renderer.go`
* `internal/gitops/render_catalog.go` (`{descriptorName: "service-cert-manager", planner: planCertManagerDynamicActions}`)

### Secrets Fallback Behavior

There is no single AWS-credential fallback chain anymore. `validateCertManagerCredentials` in `internal/gitops/cert_manager_renderer.go` validates every enabled entry in `cfg.Secrets.CertManager.AWS` and `cfg.Secrets.CertManager.Cloudflare` independently (DNS-1123 name, then required credential fields per provider). Each named credential gets its own `Secret` and `ClusterIssuer`; there is no implicit fallback to a "global application" or "global infrastructure" AWS credential for cert-manager.

Evidence:

* `internal/config/v2/config.go` (`CertManagerSecrets`)
* `internal/config/v2/helpers.go` (`EnabledCertManagerAWSCredentials`, `EnabledCertManagerCloudflareCredentials`)
* `internal/gitops/cert_manager_renderer.go`

### Inclusion Versus Presence

The renderer controls service behavior through two different mechanisms:

* file-level skipping for disabled service directories
* kustomization-level conditional inclusion for source and Flux wiring files

So a file may exist in the rendered overlay tree, but it only becomes active when the aggregate `kustomization.yaml` includes it.

Evidence:

* `internal/gitops/copy.go`
* `internal/gitops/templates/cluster-apps-base/services/sources/kustomization.yaml.tpl`
* `internal/gitops/templates/cluster-apps-base/services/fluxcd/kustomization.yaml.tpl`

## The Staged Pipeline API

`internal/gitops` also exposes a `PipelineGenerator` with ordered stages (`internal/gitops/stages/service_stage.go` and siblings) built on the reusable template registry in `internal/template` (`global_registry.go`, `embedded_registry.go`). This staged model is a supporting API for library consumers and tests, not the live command path: `opencenter cluster generate` reaches `SetupService`, which calls `internal/gitops`'s direct render/copy/promotion functions described above, not `PipelineGenerator`. See [GitOps Engine](../CODEMAPS/gitops-engine.md) for the authoritative statement of which path is live.

Evidence:

* `internal/template/global_registry.go`
* `internal/template/embedded_registry.go`
* `internal/gitops/stages/service_stage.go`
* `internal/gitops/pipeline.go`

## How To Add a New Internal Service Plugin

If you want to add a new platform service like cert-manager, the practical path today is to add a built-in service to this repository.

### Step 1: Add the Typed Config

Create `internal/config/services/<service>.go`:

* embed `BaseConfig`
* add service-specific fields
* register the config in `init()` with `registry.RegisterServiceConfig("<service>", MyServiceConfig{})`

This is required so `cluster service enable <service>` can instantiate the correct type.

### Step 2: Add Defaults

Add the service to `defaultServiceMap` in `internal/config/v2/defaults.go` (there is no top-level `internal/config/defaults.go`).

Without this, new cluster configs will not consistently include or initialize the service.

### Step 3: Add the Service Plugin

Create `internal/services/plugins/<service>.go` and model it after cert-manager, Loki, Harbor, or Velero:

```go
type MyServiceConfig struct {
    BaseConfig `yaml:",inline"`
    Endpoint   string `yaml:"endpoint" json:"endpoint,omitempty"`
}

func init() {
    registry.RegisterServiceConfig("my-service", MyServiceConfig{})
}

type MyServicePlugin struct {
    *svc.BaseServicePlugin
}

func NewMyServicePlugin() svc.ServicePlugin {
    base := svc.NewBasePlugin(svc.PluginMetadata{
        Name:        "my-service",
        Version:     "1.0.0",
        Description: "My custom platform service",
        Type:        svc.ServiceTypeCore,
        Author:      "opencenter",
        License:     "Apache-2.0",
    })

    p := &MyServicePlugin{BaseServicePlugin: base}
    base.SetValidator(p.validate)
    base.SetStatusFunc(p.status)
    return p
}
```

### Step 4: Register the Service and Dependencies

Update `internal/services/plugins/registry.go`:

* add `NewMyServicePlugin()` to the built-in service list
* declare dependencies such as `cert-manager` if your service requires TLS or issuers

If you want validation-engine integration, also add a validator in `internal/services/plugins/validators.go`.

### Step 5: Wire the CLI Management Path

Update `cmd/cluster_service.go`:

* `getServiceOptions`
* `getServiceSecrets`
* `validateService`
* `processSecrets` service-to-field mapping if the service has secrets

This is necessary because the current CLI service-management path is only partially generic.

### Step 6: Add Secret Types and Fallback Helpers

If your templates need secrets:

* add the service secret struct in the main config types
* add helper methods on `Config` if templates need fallback logic

Cert-manager is the best example because its templates call config helper methods directly.

### Step 7: Add GitOps Templates

At minimum, add:

* `internal/gitops/templates/cluster-apps-base/services/<service>/...`
* `internal/gitops/templates/cluster-apps-base/services/sources/opencenter-<service>.yaml.tpl`
* `internal/gitops/templates/cluster-apps-base/services/fluxcd/<service>.yaml.tpl`

Those exact file names matter. `RenderSingleService()` looks for:

* a service directory named after the service
* a source file named `opencenter-<service>.yaml` or `.yaml.tpl`
* a Flux file named `<service>.yaml` or `.yaml.tpl`

If the naming does not match, `opencenter cluster service enable <service> --render` will miss the files.

### Step 8: Update Aggregate Kustomizations

If the new service should be selectable through the normal overlay structure, update the aggregate kustomization templates:

* `services/sources/kustomization.yaml.tpl`
* `services/fluxcd/kustomization.yaml.tpl`

Those files decide whether the rendered source and Flux files are actually included.

### Step 9: Update Template Registry Inference If Needed

If you want the stage-based template registry path to recognize the new service automatically, add the service name to the hard-coded list in `internal/template/embedded_registry.go`.

The direct render path does not need this, but the template-registry path does.

### Step 10: Add Tests

At minimum, add:

* plugin unit tests
* integration tests for dependencies and status
* config and service-enable tests
* template rendering tests

## Related Reading

* [Plugin External CLI](plugin-external-cli.md)
* [Service Templates](services-templates.md)
* [Adding Services](../contributing/adding-services.md)
* [Code Structure](../contributing/code-structure.md)
* [GitOps Workflow](gitops-workflow.md)

## Evidence

Primary code paths referenced in this explanation:

* `cmd/cluster_service.go`
* `internal/config/services/cert_manager.go`
* `internal/config/registry/registry.go`
* `internal/config/v2/defaults.go`
* `internal/config/v2/config.go`
* `internal/config/v2/helpers.go`
* `internal/services/plugin.go`
* `internal/services/base_plugin.go`
* `internal/services/registry.go`
* `internal/services/plugins/cert_manager.go`
* `internal/services/plugins/registry.go`
* `internal/services/plugins/validators.go`
* `internal/cluster/setup_service.go`
* `internal/gitops/copy.go`
* `internal/gitops/embed.go`
* `internal/gitops/render_catalog.go`
* `internal/gitops/cert_manager_renderer.go`
* `internal/gitops/stages/service_stage.go`
* `internal/gitops/pipeline.go`
* `internal/template/global_registry.go`
* `internal/template/embedded_registry.go`