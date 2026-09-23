---
id: glossary
title: "Glossary"
sidebar_label: Glossary
description: Definitions of terms and acronyms used throughout openCenter CLI documentation.
doc_type: reference
audience: "all users"
tags: [glossary, terminology, definitions]
---
# Glossary

**Purpose:** Definitions of terms used throughout openCenter CLI documentation.

## A

* ***Active Cluster***\
The cluster a shell session is currently operating against without needing to pass a cluster name on every command. Set by `opencenter cluster use`/`cluster active`, resolved via the `OPENCENTER_CLUSTER` environment variable, a session file, or a persistent marker. See [Environment Variables](reference/environment-variables.md).
* ***Age***\
Modern encryption tool using public-key cryptography. openCenter uses Age for SOPS encryption. Simpler than GPG with no key servers or expiration by default.
* ***Ansible***\
Automation tool used by Kubespray to deploy Kubernetes. openCenter generates Ansible inventory files for cluster provisioning.
* ***Application Credential***\
OpenStack authentication method using credential ID and secret instead of username/password. Recommended for automation.
* ***Audit Signing Key***\
A 32-byte HMAC-SHA256 key (`~/.config/opencenter/audit/audit.key` by default) the CLI uses to sign every entry it writes to its tamper-evident audit log (`~/.local/state/opencenter/audit/audit.log`). See [Audit Signing Key](reference/audit-key.md).

## B

* ***BaseConfig***\
The common embedded struct (`internal/config/services/base.go`) every platform service's typed config includes: `Enabled`, `Namespace`, and status fields. Public service config types are meant to hold only fields like this -- not rendering topology or raw Helm overrides.
* ***Barbican***\
OpenStack's Key Manager service. openCenter has a standalone client (`internal/barbican/`) for it, separate from SOPS/Age secret management.
* ***Blueprint***\
The v2 cluster configuration YAML file itself (`<cluster>-config.yaml`, `ClusterPaths.ConfigPath`), stored under the CLI's "blueprints" zone (`<clusters dir>/blueprints/<org>/<cluster>/`). See [File Locations](reference/file-locations.md).
* ***Bootstrap***\
Process of deploying a cluster from configuration. Includes infrastructure provisioning, Kubernetes deployment, and GitOps setup.
* ***BDD (Behavior-Driven Development)***\
Testing methodology using Gherkin scenarios. openCenter uses Godog for BDD tests in `tests/features/`.

## C

* ***Calico***\
Container Network Interface (CNI) plugin for Kubernetes networking. Default CNI in openCenter.
* ***CNI (Container Network Interface)***\
Standard for configuring network interfaces in Linux containers. Kubernetes uses CNI plugins like Calico, Cilium, or Flannel.
* ***Cobra***\
Go library for building CLI applications. openCenter uses Cobra for command structure and flag parsing.
* ***Control Plane***\
Kubernetes components that manage the cluster (API server, scheduler, controller manager, etcd). openCenter deploys 3 control plane nodes for high availability.

## D

* ***Diátaxis***\
Documentation framework organizing content into four types: Tutorials, How-To Guides, Reference, and Explanation. openCenter documentation follows Diátaxis.
* ***Drift Detection***\
Process of identifying differences between desired configuration (Git) and actual cluster state. openCenter provides drift detection commands (`internal/operations/drift_detector.go`). Implemented today for the providers that register an `internal/cloud.CloudProvider` (OpenStack, VMware); it does not currently inspect FluxCD or service-level Kubernetes resources.
* ***Descriptor (overlay descriptor)***\
A YAML file under `internal/services/descriptors/data/` that is the authority for *what files the renderer produces* for a service -- template roots, conditional files, and aggregate targets. Distinct from `ServicePluginManifest`, which describes service identity/dependencies/validation, not rendering topology. See [Renderer Contract](contributing/rendering-contract.md) and [Descriptor Condition Schema](contributing/descriptor-condition-schema.md).

## F

* ***FluxCD***\
GitOps tool that continuously reconciles cluster state with Git repository. openCenter generates FluxCD manifests for GitOps management.
* ***Flavor***\
OpenStack term for VM size (CPU, RAM, disk). Similar to AWS instance types or VMware VM templates.

## G

* ***Gitea***\
Self-hosted Git service. openCenter's local development workflow (`mise run gitea-up`, `opencenter local gitea ...`) runs a disposable Gitea instance as the GitOps push target for local Kind clusters.
* ***GitOps***\
Operational model where Git is the single source of truth for infrastructure and applications. Changes are made via Git commits, not direct cluster access.
* ***Godog***\
Go implementation of Cucumber for BDD testing. openCenter uses Godog for feature tests.

## H

* ***HelmRelease***\
FluxCD custom resource that deploys Helm charts. openCenter generates HelmRelease manifests for platform services.

## K

* ***Kind (Kubernetes in Docker)***\
Tool for running local Kubernetes clusters using Docker containers. Used for development and testing.
* ***Kubespray***\
Ansible playbooks for deploying production-ready Kubernetes clusters. openCenter uses Kubespray for cluster provisioning.
* ***Kustomize***\
Kubernetes configuration management tool using overlays. openCenter uses Kustomize for cluster-specific customization.
* ***Kustomization***\
FluxCD custom resource that applies Kustomize overlays. openCenter generates Kustomization manifests for services.

## M

* ***Magnum***\
OpenStack's managed-Kubernetes service. openCenter's `magnum` provider (`internal/cloud/magnum/`) creates and manages a Magnum-backed cluster directly through the OpenStack API, rather than provisioning VMs with OpenTofu and installing Kubernetes with Kubespray as the plain `openstack` provider does.
* ***Mise***\
Tool version manager and task runner. openCenter uses Mise for managing Go, kubectl, kind, helm versions and build tasks.

## O

* ***Octavia***\
OpenStack load balancer service. When disabled, openCenter uses VRRP for control plane high availability.
* ***OpenTofu***\
Open-source Terraform fork. openCenter supports both Terraform and OpenTofu for infrastructure provisioning.
* ***Overlay***\
Kustomize pattern for customizing base manifests. openCenter uses overlays for cluster-specific configuration.

## P

* ***Pod Security Admission***\
Kubernetes admission controller enforcing security policies on pods. openCenter configures Pod Security Admission via Kubespray.
* ***Preflight Check***\
Validation performed before deployment to catch issues early. openCenter provides preflight commands for connectivity, quotas, and provider constraints.
* ***Provider***\
Infrastructure platform for cluster deployment. openCenter's provider set includes OpenStack, VMware, Baremetal, Kind, and Magnum (the OpenStack managed-Kubernetes variant); AWS, GCP, and Azure are present in the config schema but rejected as "planned, not yet available" by `cmd/provider_availability.go`.

## R

* ***Readiness (validation)***\
The offline, cross-field business-rule checks `opencenter cluster validate` runs after schema validation (`v2.ValidateReadiness`, `internal/config/v2/readiness.go`) -- provider required fields, GitOps auth consistency, and enabled-service secrets. Never contacts a cloud provider or Git remote. See [Validation Rules](reference/validation-rules.md).
* ***Render Catalog***\
The immutable, in-code lookup of built-in rendering behavior for services with non-standard rendering needs (`internal/gitops/render_catalog.go`, `newBuiltInRenderCatalog`). Holds direct Go function references (renderer functions, dependency lists) keyed by service name -- never a mutable, string-based registry. See [Adding New Platform Services](contributing/adding-services.md).

## S

* ***ServicePluginManifest***\
The type (`internal/services/plugin.go`) describing a service's identity, dependencies, and validation rules -- not its rendering topology (that's the descriptor's job; see *Descriptor* above).
* ***SOPS (Secrets OPerationS)***\
Tool for encrypting files with Age or GPG keys. openCenter uses SOPS for secrets management in Git.
* ***Sprig***\
Template function library for Go templates. openCenter uses Sprig functions in templates for string manipulation, encoding, etc.

## T

* ***Terraform***\
Infrastructure as Code tool for provisioning cloud resources. openCenter generates Terraform configurations for cluster infrastructure.

## V

* ***VRRP (Virtual Router Redundancy Protocol)***\
Protocol for high availability of network gateways. openCenter uses VRRP for control plane HA when Octavia is disabled.
* ***vSphere***\
VMware virtualization platform. openCenter supports deploying clusters on vSphere with pre-provisioned VMs.

## W

* ***Worker Node***\
Kubernetes node that runs application workloads. openCenter deploys 2+ worker nodes by default.

---

## Acronyms

| Acronym | Full Term | Description |
| --- | --- | --- |
| ADR | Architecture Decision Record | Document explaining architectural choices |
| API | Application Programming Interface | Interface for software interaction |
| AWS | Amazon Web Services | Cloud computing platform |
| BDD | Behavior-Driven Development | Testing methodology |
| CIDR | Classless Inter-Domain Routing | IP address notation (e.g., 10.0.0.0/16) |
| CLI | Command-Line Interface | Text-based user interface |
| CNI | Container Network Interface | Kubernetes networking standard |
| CR | Custom Resource | Kubernetes API extension |
| CRD | Custom Resource Definition | Schema for Custom Resources |
| CSI | Container Storage Interface | Kubernetes storage standard |
| DI | Dependency Injection | Design pattern for managing dependencies |
| DNS | Domain Name System | Internet naming system |
| HA | High Availability | System design for minimal downtime |
| IAM | Identity and Access Management | Authentication and authorization |
| IaC | Infrastructure as Code | Managing infrastructure via code |
| JSON | JavaScript Object Notation | Data interchange format |
| K8s | Kubernetes | Container orchestration platform |
| OIDC | OpenID Connect | Authentication protocol |
| RBAC | Role-Based Access Control | Authorization model |
| SSH | Secure Shell | Encrypted network protocol |
| TLS | Transport Layer Security | Encryption protocol |
| VM | Virtual Machine | Virtualized computer |
| VRRP | Virtual Router Redundancy Protocol | HA protocol for routers |
| YAML | YAML Ain’t Markup Language | Human-readable data format |

---

## Open items

* ***OpenStack minimal default service profile*** (not implemented)\
A draft plan proposed narrowing the default enabled-service set for the OpenStack provider (to `calico`, `cert-manager`, `fluxcd`, `gateway`, `gateway-api`, `kyverno`, `openstack-ccm`, `openstack-csi`, `sources` only) and disabling `opencenter.identity.oidc.enabled` by default for OpenStack, while leaving Kind, Baremetal, and VMware defaults unchanged. As of this writing, `defaultServiceMap` in `internal/config/v2/defaults.go` takes no provider argument and applies the same default-enabled service set across providers; identity OIDC defaults are not provider-conditional. Anyone picking this up should re-verify current defaults in `internal/config/v2/defaults.go` before implementing.

---

**Evidence:**

* Technical terms from codebase: `internal/` packages
* Provider terminology: `internal/cloud/` implementations
* GitOps concepts: `internal/gitops/` and ecosystem.md
* Kubernetes concepts: Standard Kubernetes documentation
* Tool names: `.mise.toml`, `go.mod` dependencies