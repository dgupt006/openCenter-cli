---
id: adding-providers
title: "Adding New Infrastructure Providers"
sidebar_label: Adding New Infrastructure
description: Add support for a new infrastructure provider, using the Magnum provider as the current worked example.
doc_type: how-to
audience: "developers"
tags: [contributing, providers]
---
# Adding New Infrastructure Providers

**Purpose:** For developers, shows how to add support for a new infrastructure provider (cloud platform, bare metal, virtualization), using `internal/cloud/magnum/` -- the most recently added provider -- as the worked example.

## Prerequisites

* Development environment set up (see [Development Environment Setup](development-setup.md)).
* Understanding of the target provider's API/SDK.
* Test credentials for the target provider (or a way to fully mock its client -- see the Magnum test suite below for the pattern).

## The extension points, precisely

A provider touches several independent extension points. Not every provider needs all of them -- Kind and Baremetal, for example, do not implement a standalone `internal/cloud/<provider>` API client at all. Work through this list and implement only what your provider actually needs:

1. **Config schema and validation** -- `internal/config/v2/provider.go` (or `internal/config/v2/config.go` for the struct itself).
2. **Guided configuration (`cluster configure --guided`)** -- an `orchestration.ProviderOrchestrator` implementation in `internal/cluster/<provider>_configure_orchestrator.go`, registered in `internal/cluster/configure_service.go`.
3. **Deploy/bootstrap lifecycle (`cluster deploy`)** -- a `lifecycleBootstrapProvider` implementation in `internal/cluster/<provider>_bootstrap_provider.go`, wired by a switch in `internal/cluster/bootstrap_service.go`.
4. **Destroy lifecycle (`cluster destroy`)** -- a `lifecycleDestroyProvider` implementation in `internal/cluster/<provider>_destroy_provider.go`, wired by a switch in `internal/cluster/destroy_service.go`.
5. **Standalone API client** (if the provider has a lifecycle API worth isolating from the rest of the codebase) -- `internal/cloud/<provider>/provider.go`, imported only by the bootstrap/destroy providers above.
6. **Provider availability gate** -- `cmd/provider_availability.go`'s `checkProviderAvailability`.
7. *(Separate and optional)* **Drift detection** -- `internal/cloud.CloudProvider` (`GetCurrentState`/`DetectDrift`/`ReconcileDrift`), registered via `CloudProviderFactory.RegisterProvider`. This is a distinct interface from the lifecycle client in step 5. Today only OpenStack and VMware implement it; Kind, Baremetal, and Magnum do not -- do not assume every provider needs this.

## Worked example: Magnum

Magnum (`internal/cloud/magnum/provider.go`, ~1150 lines) manages an OpenStack Magnum-managed Kubernetes cluster directly through Gophercloud -- no Terraform/Kubespray, unlike the OpenStack provider. It is a good template because it exercises every extension point except drift detection.

### 1. Config schema (`internal/config/v2/provider.go`, `internal/config/v2/config.go`)

* `InfrastructureConfig.Cloud.Magnum *MagnumCloudConfig` holds the provider-specific block.
* Every *other* provider's `ValidateConfig` explicitly rejects a populated `cfg.Cloud.Magnum` block (mutual exclusion is enforced both ways -- see the `"infrastructure.cloud.magnum must be empty when provider is <x>"` checks for openstack/aws/gcp/azure/vmware).
* `MagnumProvider.ValidateConfig` requires `auth_url`, `region`, `project_id`, `cluster_template`, and requires `application_credential_id`/`application_credential_secret` to be supplied together (both or neither).

### 2. Guided configuration (`internal/cluster/magnum_configure_orchestrator.go`)

Implements `orchestration.ProviderOrchestrator`:

* `Name()` / `Supports(provider string)` -- identifies itself as `"magnum"`.
* `Discover(ctx, cfg)` -- intentionally a no-op for Magnum (cluster templates are deployment-specific and must be entered manually, not discovered from the cloud). A provider with a discoverable catalog (images, flavors, networks -- see OpenStack) implements real discovery here.
* `Prompts(cfg, discovery)` -- returns `[]orchestration.PromptSpec` for every field the guided flow should ask about (`magnum.auth_url`, `magnum.region`, `magnum.project_id`, `magnum.application_credential_id`, `magnum.application_credential_secret` (as a `PromptKindSecret`), `magnum.cluster_template`, `magnum.keypair`, `magnum.master_flavor_id`, `magnum.node_flavor_id`, `magnum.master_count`, ...).
* `ApplyAnswers` patches the typed config from the collected answers.
* `CapabilityRequests` -- declares any additional capability checks needed for the flow.

Register the new orchestrator in `internal/cluster/configure_service.go` next to `newMagnumConfigureOrchestrator()`.

### 3. Deploy lifecycle (`internal/cluster/magnum_bootstrap_provider.go`)

Implements `lifecycleBootstrapProvider.BuildSteps(cfg *v2.Config, clusterPaths *paths.ClusterPaths, opts *BootstrapOptions) ([]bootstrapStep, error)`. For Magnum this validates that `opencenter.infrastructure.cloud.magnum` and a kubeconfig path are set, builds a `magnumprovider.Config` via `magnumConfigFromV2`, and returns a small step list (`magnum-create`, ...) that creates or recovers the cluster through the Magnum API, polls for readiness, and exports the kubeconfig -- rather than the many discrete OpenTofu/Kubespray steps used by [the OpenStack provider](cluster-deploy-openstack.md). It persists a durable identity/UUID state file (`magnumIdentityStatePath`) so a lost or ambiguous create response can be recovered instead of creating a duplicate cluster.

Wire the new provider into `internal/cluster/bootstrap_service.go`'s provider switch (`newMagnumBootstrapProvider(s.runner)`).

### 4. Destroy lifecycle (`internal/cluster/magnum_destroy_provider.go`)

Implements `lifecycleDestroyProvider.BuildSteps`, deletes the remote cluster through the Magnum API, waits for deletion, and clears the persisted identity/bootstrap state afterward (`clearDestroyedMagnumState`). Wire it into `internal/cluster/destroy_service.go`'s provider switch (`newMagnumDestroyProvider(cfg)`).

### 5. Standalone API client (`internal/cloud/magnum/provider.go`)

A self-contained Gophercloud client, independent of the rest of this repository's config types (`Config` struct uses its own field names; `magnumConfigFromV2` in `internal/cluster/magnum_bootstrap_provider.go` is the only place that translates `*v2.MagnumCloudConfig` into it). Public surface: `NewProvider`, `NewProviderWithService`, `CreateCluster`, `GetCluster`, `WaitVisible`, `WaitReady`, `ExportKubeconfig`, `DeleteCluster`, `WaitDeleted`. `doc.go` documents a real Gophercloud constraint worth knowing before you write a similar client: **v1 lifecycle operations don't accept contexts**, so the provider checks cancellation before/after each operation and re-authenticates a fresh `ProviderClient` per request group with the context already attached, so Gophercloud's reauthentication callback refreshes the right client.

### 6. Provider availability gate (`cmd/provider_availability.go`)

`checkProviderAvailability` only rejects providers in a hardcoded `planned` map (`aws`, `gcp`, `azure` today) with a "not yet available" error. A new provider works as soon as it is **not** in that map -- you do not need to add anything here for the provider to function. You should, however, update the error message's "Supported providers: ..." list if you want it to stay accurate; it currently reads `openstack, vmware, kind, baremetal` and does not mention `magnum`, which is a real, if harmless, drift in this repository today.

## Checklist for a new provider

* [ ] Config struct + schema entry in `internal/config/v2/config.go`, regenerate with `mise run schema-v2`.
* [ ] Mutual-exclusion checks added to every *other* provider's `ValidateConfig` in `internal/config/v2/provider.go`, and a `ValidateConfig` for the new provider itself.
* [ ] `orchestration.ProviderOrchestrator` implementation, registered in `internal/cluster/configure_service.go`.
* [ ] `lifecycleBootstrapProvider` implementation, registered in `internal/cluster/bootstrap_service.go`.
* [ ] `lifecycleDestroyProvider` implementation, registered in `internal/cluster/destroy_service.go`.
* [ ] Standalone API client under `internal/cloud/<provider>/`, if warranted.
* [ ] Decide whether the provider also needs `internal/cloud.CloudProvider` drift detection -- most new providers do not need this on day one.
* [ ] `cmd/provider_availability.go` -- do *not* add the new provider to the `planned` map; update the "Supported providers" message text.
* [ ] Tests (see below).
* [ ] `docs/reference/providers.md` entry (owned by another doc set -- open a cross-reference, don't duplicate its content here).

## Tests to write

Magnum's test suite is the bar to match. `internal/cloud/magnum/provider_test.go` (~600 lines, ~18 test functions) covers: config/credential validation, auth-scope handling, CA + finite HTTP timeout handling, `CreateCluster` option building, `WaitReady`/`WaitDeleted` state-machine transitions and cancellation, kubeconfig export (secure file permissions, certificate-flow embedding, empty-certificate rejection), name-vs-ID lookup fallback, opt-in insecure transport, and Gophercloud reauthentication/token-refresh under cancellation. `internal/cluster/magnum_lifecycle_test.go` and `internal/cluster/magnum_configure_orchestrator_test.go` cover `BuildSteps` behavior (including the nil-options error case and dry-run not executing lifecycle steps), durable UUID reuse across a lost create response, destroy-time state handling, guided-flow prompt generation, answer application, and capability requests.

At minimum, write tests for:

* Config/credential validation (missing required fields, mutually-exclusive fields).
* The create/wait/destroy state machine, including cancellation mid-operation.
* Recovery from an ambiguous/lost API response (don't silently create a duplicate resource).
* Kubeconfig or credential export (file permissions, no partial/empty writes).
* The guided-configure prompt set and `ApplyAnswers` patching.

## Verification

```bash
mise run build
mise run schema-v2   # if you changed the config schema
go test ./internal/cloud/<provider>/... ./internal/cluster/...
mise run test
mise run godog
```

Then exercise the real CLI flow end to end against a test account:

```bash
./bin/opencenter cluster init test --org test-org --type <provider>
./bin/opencenter cluster configure test --guided
./bin/opencenter cluster validate test
./bin/opencenter cluster generate test
./bin/opencenter cluster deploy test
./bin/opencenter cluster destroy test --force
```
