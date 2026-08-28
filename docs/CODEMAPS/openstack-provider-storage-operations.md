---
id: openstack-provider-storage-operations
title: "Explain OpenStack Provider and Storage Operations"
sidebar_label: OpenStack Provider and Storage
description: Explains the typed OpenStack provider plan/apply flow and the explicit one-service storage plan/apply flow, including persistence, credentials, and recovery boundaries.
doc_type: explanation
audience: "contributors, maintainers, operators"
tags: [openstack, provider, storage, plan, apply, credentials]
---
# OpenStack provider and storage operations

OpenStack configuration is split into two explicit command families:

```text
cluster provider openstack plan <cluster>
cluster provider openstack apply <cluster>
cluster service storage plan <service> --cluster <cluster> --backend swift|s3
cluster service storage apply <service> --cluster <cluster> --backend swift|s3
```

The provider family updates provider metadata and resource selections. The storage family provisions storage for exactly one supported service. These commands replace the former combined synchronization workflow; there is no compatibility command or implicit multi-service operation.

## Provider plan/apply

`cmd/cluster_provider_openstack.go` resolves the organization/cluster identifier, reads the native v2 configuration, requires the configured provider to be `openstack`, loads the selected `clouds.yaml` profile, and performs read-only OpenStack discovery through `internal/cloud/openstack`.

`internal/cluster/provider/openstack.Plan` receives the typed config and a discovery snapshot. It returns a prospective typed config plus a structured result. Empty or placeholder fields can be filled from unambiguous discovery; populated fields require `--replace` before they can change. Ambiguous images, networks, subnets, or availability zones are returned as selections for an explicit selector. `--import-auth` and `--import-tls` opt into importing those profile values, with sensitive result fields redacted. With `--create-internal-network`, the planner bypasses internal network/subnet discovery and selection, atomically clears the top-level and nested internal network/subnet mirrors, and sets `internal_network_mode` to `tofu-managed`; populated mirrors require `--replace`. The mode rejects internal selectors and a configured `networking.vlan.id`.

The provider path has no remote mutation capability. `plan` validates and reports the prospective configuration. `apply` repeats the plan, validates the candidate, checks that the source file has not changed, writes a backup, and atomically persists the typed configuration. `--dry-run` stops before persistence. No OpenStack resource, container, or credential is created by either provider operation.

## Storage plan/apply

`cmd/cluster_service_storage.go` requires one service, one cluster, one backend, and an OpenStack cloud profile. Supported mappings are `loki` and `tempo` with Swift or S3, and `etcd-backup` and `velero` with S3. Container/bucket names and explicit S3 endpoints are validated before remote work.

`internal/cluster/storage/openstack.Plan` performs storage preflight, derives the endpoint and region, and calculates typed service and secret changes before any confirmation or apply mutation. When credential creation, rotation, or revocation is required, preflight resolves the credential owner from `auth.user_id` when explicitly configured; otherwise it extracts `token.user.id` from the already-authenticated Keystone v3 result without an identity lookup. Existing complete credentials are reused unless `--rotate-credentials` is supplied. A partial credential pair blocks the plan until rotation is explicitly requested, and that already-blocked path does not require owner resolution. All output redacts generated or sensitive values; the resolved owner ID is internal preflight state and is omitted from JSON/YAML serialization.

`apply` replans against the current file and completes owner resolution before confirmation and before `EnsureContainer`. It ensures the object-store container, creates a Swift application credential or project-scoped EC2 credential when required, validates the prospective typed configuration, writes a backup, persists the configuration atomically, and then revokes a replaced credential. The resolved owner is passed explicitly to credential create/delete adapters. A recovery record tracks creation, persistence, and revocation. Owner/preflight failures occur before any container, credential, recovery, backup, or config mutation and are ordinary errors; only failures after credential creation or rotation revoke/persistence boundaries return partial status and retain recovery information. The command maps that state to exit code 4.

Global `--dry-run` returns the storage plan without ensuring containers, creating credentials, persisting configuration, or revoking credentials. `--yes` is required for non-text structured apply output; text apply can use the interactive confirmation.

## Boundaries

- Provider plan/apply is local typed configuration reconciliation backed by read-only discovery.
- Storage plan/apply is an explicit one-service provisioning workflow with remote actions, typed persistence, credential reuse/rotation, and recovery handling.
- `secrets sync` remains the encrypted Kubernetes-manifest synchronization path; it does not provision OpenStack storage credentials.
- `cluster status --sync` remains the live service-status path; it is unrelated to provider or storage provisioning.
- Lifecycle deployment, GitOps generation, drift reconciliation, and external plugin execution remain separate boundaries.

## Package ownership

| Package | Responsibility |
|---|---|
| `cmd/cluster_provider.go` and `cmd/cluster_provider_openstack.go` | Register and execute provider plan/apply commands, output, confirmation, and exit classification |
| `cmd/cluster_service.go` and `cmd/cluster_service_storage.go` | Register and execute explicit one-service storage plan/apply commands |
| `internal/cluster/provider/openstack` | Pure typed provider planning and local atomic persistence |
| `internal/cluster/storage/openstack` | Storage mapping, credential planning, remote-action sequencing, recovery, and atomic persistence |
| `internal/cloud/openstack/profile.go` | `clouds.yaml` profile loading and default path resolution |
| `internal/cloud/openstack/read_only_discovery.go` | Authenticated read-only inventory for provider selections |
| `internal/cloud/openstack/storage.go` | Storage preflight, container, application-credential, and EC2-credential adapters |
| `internal/config/v2` and `internal/config/services` | Typed configuration and service/secret fields persisted by the workflows |

## Verification surfaces

Command-tree tests assert that the legacy command path is absent and all four replacement operations are present. Provider and storage package tests cover deterministic plans, ambiguity and replacement guards, redaction, stale-file protection, credential reuse/rotation, atomic persistence, and partial/recovery outcomes. Built-in reference pages are regenerated from `cmd.NewBuiltinRootCmd()` so stale command pages are removed as part of generation.
