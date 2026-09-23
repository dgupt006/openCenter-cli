---
id: index
title: "openCenter CLI Documentation"
sidebar_label: openCenter CLI Documentation
description: Landing page for the openCenter CLI documentation, organised by lifecycle category.
doc_type: reference
audience: "all users"
tags: [opencenter, cli, documentation, home]
---
# openCenter CLI Documentation

**Purpose:** For all users, points to the openCenter CLI documentation organised by lifecycle category (getting started, operations, reference, concepts, providers, contributing).

openCenter is a command-line tool that turns a single declarative YAML file into a production-ready Kubernetes cluster with GitOps management. It standardises cluster deployment across OpenStack, VMware, Baremetal, Kind, and OpenStack Magnum, and ships configuration validation, secrets management, and FluxCD-ready repository generation.

## Quick start

* [Getting Started](getting-started/getting-started.md) -- create your first cluster in 10 minutes.
* [Create and Deploy an OpenStack Cluster with the CLI](opencenter-cluster-via-cli.md) -- a full walkthrough including provider discovery and per-service storage.
* [CLI Commands Reference](reference/cli-commands.md) -- complete command tree.
* [Configuration Schema](reference/configuration-schema.md) -- file structure and field reference.
* [Glossary](glossary.md) -- terminology used throughout these docs.

## Getting started

Tutorial-style walkthroughs for first-time setup.

* [Getting Started](getting-started/getting-started.md) -- end-to-end first cluster.
* [OpenStack First Cluster](getting-started/openstack-first-cluster.md) -- deploy on OpenStack.
* [Kind Local Development](getting-started/kind-local-development.md) -- local development cluster.
* [VMware Deployment](getting-started/vmware-deployment.md) -- deploy on pre-provisioned vSphere VMs.
* [Multi-Cluster Management](getting-started/multi-cluster-setup.md) -- manage several clusters.
* [Create and Deploy an OpenStack Cluster with the CLI](opencenter-cluster-via-cli.md) -- deeper OpenStack walkthrough.

## Operations

Task-oriented how-to guides for day-2 work.

|     |     |
| --- | --- |
| Cluster creation | [Create a Kind cluster](operations/create-kind-cluster.md)<br> [Create an OpenStack cluster](operations/create-openstack-cluster.md)<br> [Deploy an OpenStack cluster](operations/deploy-openstack-cluster.md)<br> [Deployment profiles](operations/deployment-profiles.md) |
| Configuration | [Validate configuration](operations/validate-configuration.md)<br> [Customize services](operations/customize-services.md)<br> [Configure networking](operations/configure-networking.md)<br> [Normalize legacy renderer metadata](operations/normalize-legacy-renderer-metadata.md) |
| Secrets and bootstrap | [Manage secrets](operations/manage-secrets.md)<br> [Configure Flux bootstrap auth](operations/flux-bootstrap-methods.md) |
| Day-2 operations | [Add worker pools](operations/add-worker-pools.md)<br> [Manage worker pools](operations/manage-worker-pools.md)<br> [Backup and restore](operations/backup-and-restore.md)<br> [Upgrade Kubernetes](operations/upgrade-kubernetes.md)<br> [Migrate clusters](operations/migrate-clusters.md) |
| Integration and plugins | [Integrate CI/CD](operations/integrate-ci-cd.md)<br> [Create and install a CLI plugin](operations/create-install-cli-plugin.md) |
| Troubleshooting | [Troubleshoot deployment](operations/troubleshoot-deployment.md) |

## Providers

Provider-specific guides.

* [VMware Provider Guide](providers/vmware.md)
* [VMware Quick Start](providers/vmware-quick-start.md)
* [VMware Terraform Template](providers/vmware-terraform-template.md)

See also [Providers](reference/providers.md) in the reference section for the full provider comparison and support matrix.

## Reference

Lookup material -- structured, complete, scan-friendly.

|     |     |
| --- | --- |
| CLI and configuration | [CLI Commands](reference/cli-commands.md)<br> [Configuration Schema](reference/configuration-schema.md)<br> [GitOps Configuration](reference/gitops-configuration.md)<br> [Configuration Precedence](reference/configuration-precedence.md)<br> [Default Values](reference/default-values.md)<br> [Environment Variables](reference/environment-variables.md)<br> [Exit Codes](reference/exit-codes.md)<br> [File Locations](reference/file-locations.md) |
| Platform and providers | [Platform Services](reference/platform-services.md)<br> [Per-Service Reference](reference/services/index.md)<br> [Providers](reference/providers.md)<br> [Validation Rules](reference/validation-rules.md) |
| Security and tooling | [Audit Signing Key](reference/audit-key.md)<br> [Mise Tasks](reference/mise-tasks.md)<br> [GitHub Actions Workflows](reference/github-actions-workflows.md) |

## Concepts

Background and rationale -- the "why" behind the design.

* [Architecture](concepts/architecture.md) -- the user/operator-facing architecture explanation.
* [Reference Architecture](concepts/reference-architecture.md)
* [GitOps Workflow](concepts/gitops-workflow.md)
* [Configuration Lifecycle](concepts/configuration-lifecycle.md)
* [Security Model](concepts/security-model.md)
* [Security Update Design](concepts/security-update-design.md)
* [Services and Templates](concepts/services-templates.md)
* [Drift Detection](concepts/drift-detection.md)
* [Plugin Internal Services](concepts/plugin-internal-services.md)
* [Plugin External CLI](concepts/plugin-external-cli.md)
* [Provider Comparison](concepts/provider-comparison.md)

For the code-level (contributor-facing) architecture map, rather than the concept-level explanation above, see [Understand the openCenter CLI Architecture](architecture.md) and the [Architecture Maps](CODEMAPS/INDEX.md) under Contributing below.

## Contributing

Developer documentation for contributors and maintainers.

* [Contributing Guide](contributing/contributing.md)
* [Development Environment Setup](contributing/development-setup.md)
* [Codebase Organization](contributing/code-structure.md)
* [Understand the openCenter CLI Architecture](architecture.md) -- terse, contributor-facing package/entry-point map (complements [Codebase Organization](contributing/code-structure.md) and the concept-level [Architecture](concepts/architecture.md) above).
* [Navigate the openCenter CLI Code Map](llm-code-map.md) -- entry points, package responsibilities, and safe-change boundaries.
* [Testing Guide](contributing/testing-guide.md)
* [Adding New Infrastructure Providers](contributing/adding-providers.md)
* [Adding New Platform Services](contributing/adding-services.md)
* [Build System (Mise)](contributing/build-system.md)
* [Release Process](contributing/release-process.md)
* [Cluster Validate Execution Flow](contributing/validation.md)
* [Service Enable/Disable Lifecycle](contributing/services.md)
* [Renderer Contract](contributing/rendering-contract.md)
* [Descriptor Condition Schema](contributing/descriptor-condition-schema.md)
* [Overlay Rendering Security Policy](contributing/overlay-security-policy.md)
* [Cluster Init Details](contributing/cluster-init-details.md)
* [cluster deploy -- OpenStack Provider](contributing/cluster-deploy-openstack.md)
* [Kind Cluster Verification Guide](contributing/kind-cluster-verification.md)

### Architecture maps (code-map deep dives)

Durable architecture maps for contributors and code-oriented agents, not part of the published reader-facing site -- see [Architecture Maps Index](CODEMAPS/INDEX.md) for the full set (CLI commands, cluster lifecycle, config system, DI container, GitOps engine, import/operations/resilience, OpenStack provider storage operations, providers, rendering ownership and secret artifacts, runtime extensions and local development, secrets management).

## Release notes

* [1.0.0-rc01](release/1.0.0-rc01.md)
* [1.0.0-rc02](release/1.0.0-rc02.md)
* [1.0.0-rc03](release/1.0.0-rc03.md)
* [1.0.0-rc04](release/1.0.0-rc04.md)
* [1.0.0-rc05](release/1.0.0-rc05.md)
* [1.0.0-rc06](release/1.0.0-rc06.md)

## Documentation framework

These docs follow the [Diátaxis framework](https://diataxis.fr/) but are organised by lifecycle category (getting-started, operations, reference, concepts, providers, contributing) rather than by Diátaxis type. See [docs/README.md](README.md) for the complete site map and editing rules.

## Getting help

* [GitHub Issues](https://github.com/opencenter-cloud/openCenter-cli/issues) -- report bugs or request features.
* [Open a docs issue](https://github.com/opencenter-cloud/openCenter-cli/issues/new) for documentation problems.
