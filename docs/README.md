# Documentation

This folder is the source for the openCenter CLI documentation. Pages are
written in Markdown with YAML frontmatter and follow the
[Diátaxis](https://diataxis.fr/) framework: every page is one of
`tutorial`, `how-to`, `reference`, or `explanation`.

## Layout

Pages are organised by **lifecycle category**, not by Diátaxis type:

| Directory | Default `doc_type` | Purpose |
|---|---|---|
| `getting-started/` | `tutorial` | First-cluster walkthroughs |
| `operations/` | `how-to` | Day-2 task guides |
| `reference/` | `reference` | CLI, schema, services, flags |
| `concepts/` | `explanation` | Architecture and rationale |
| `providers/` | `reference` | Per-provider guides |
| `contributing/` | mixed (`how-to`/`reference`/`explanation`) | Developer docs |
| `release/` | `reference` | Release notes |
| `CODEMAPS/` | `explanation` | Durable code-map deep dives (not part of the published site) |

A few pages live at the `docs/` root rather than in a lifecycle directory
(`architecture.md`, `llm-code-map.md`, `opencenter-cluster-via-cli.md`,
`index.md`, `glossary.md`, `README.md`) -- see "Other" below.

## Complete Site Map

### Getting Started (Tutorials)

| Page | Description |
|------|-------------|
| [getting-started](getting-started/getting-started.md) | End-to-end first cluster walkthrough |
| [kind-local-development](getting-started/kind-local-development.md) | Local cluster with Kind |
| [openstack-first-cluster](getting-started/openstack-first-cluster.md) | First cluster on OpenStack |
| [vmware-deployment](getting-started/vmware-deployment.md) | Deploy on VMware |
| [multi-cluster-setup](getting-started/multi-cluster-setup.md) | Multiple clusters in one org |

### Operations (How-To Guides)

| Page | Description |
|------|-------------|
| [validate-configuration](operations/validate-configuration.md) | Validate cluster config |
| [manage-secrets](operations/manage-secrets.md) | SOPS encryption lifecycle |
| [customize-services](operations/customize-services.md) | Enable/disable/configure services |
| [normalize-legacy-renderer-metadata](operations/normalize-legacy-renderer-metadata.md) | Remove legacy v2 renderer metadata |
| [configure-networking](operations/configure-networking.md) | Network and DNS setup |
| [add-worker-pools](operations/add-worker-pools.md) | Add worker node groups |
| [manage-worker-pools](operations/manage-worker-pools.md) | Scale, update, and remove worker pools |
| [backup-and-restore](operations/backup-and-restore.md) | Velero backup/restore |
| [upgrade-kubernetes](operations/upgrade-kubernetes.md) | Kubernetes version upgrades |
| [migrate-clusters](operations/migrate-clusters.md) | Cluster migration |
| [troubleshoot-deployment](operations/troubleshoot-deployment.md) | Deployment debugging |
| [integrate-ci-cd](operations/integrate-ci-cd.md) | CI/CD pipeline integration |
| [create-install-cli-plugin](operations/create-install-cli-plugin.md) | CLI plugin authoring |
| [flux-bootstrap-methods](operations/flux-bootstrap-methods.md) | Flux bootstrap options |
| [create-kind-cluster](operations/create-kind-cluster.md) | Kind cluster creation |
| [create-openstack-cluster](operations/create-openstack-cluster.md) | OpenStack cluster creation |
| [deploy-openstack-cluster](operations/deploy-openstack-cluster.md) | OpenStack cluster deployment |
| [deployment-profiles](operations/deployment-profiles.md) | Deployment method profiles |

### Reference

#### Configuration & CLI

| Page | Description |
|------|-------------|
| [cli-commands](reference/cli-commands.md) | Full CLI command tree |
| [configuration-schema](reference/configuration-schema.md) | Cluster config YAML schema |
| [gitops-configuration](reference/gitops-configuration.md) | GitOps section field reference |
| [configuration-precedence](reference/configuration-precedence.md) | Config override order |
| [default-values](reference/default-values.md) | Default config values |
| [environment-variables](reference/environment-variables.md) | Env var reference |
| [exit-codes](reference/exit-codes.md) | CLI exit codes |
| [file-locations](reference/file-locations.md) | Config/key file paths |
| [validation-rules](reference/validation-rules.md) | Validation rule catalog |
| [mise-tasks](reference/mise-tasks.md) | Development task runner |
| [github-actions-workflows](reference/github-actions-workflows.md) | Repository CI/CD workflows and runner contract |
| [audit-key](reference/audit-key.md) | Audit signing key |
| [providers](reference/providers.md) | Infrastructure providers |
| [platform-services](reference/platform-services.md) | Platform service catalog |
| [opencenter/](reference/opencenter/) | Auto-generated per-command reference |

#### Platform Services (per-service docs)

| Page | Category | Description |
|------|----------|-------------|
| [services/index](reference/services/index.md) | -- | Service matrix and overview |
| [services/calico](reference/services/calico.md) | Networking | Calico CNI |
| [services/cilium](reference/services/cilium.md) | Networking | Cilium CNI |
| [services/kube-ovn](reference/services/kube-ovn.md) | Networking | Kube-OVN CNI |
| [services/gateway-api](reference/services/gateway-api.md) | Networking | Gateway API CRDs |
| [services/gateway](reference/services/gateway.md) | Networking | Envoy gateway |
| [services/metallb](reference/services/metallb.md) | Networking | Bare-metal LB |
| [services/cert-manager](reference/services/cert-manager.md) | Security | TLS certificates |
| [services/keycloak](reference/services/keycloak.md) | Security | Identity/OIDC |
| [services/kyverno](reference/services/kyverno.md) | Security | Policy engine |
| [services/rbac-manager](reference/services/rbac-manager.md) | Security | RBAC management |
| [services/sealed-secrets](reference/services/sealed-secrets.md) | Security | Encrypted secrets |
| [services/openstack-ccm](reference/services/openstack-ccm.md) | Cloud | OpenStack CCM |
| [services/openstack-csi](reference/services/openstack-csi.md) | Storage | Cinder CSI |
| [services/vsphere-csi](reference/services/vsphere-csi.md) | Storage | vSphere CSI |
| [services/longhorn](reference/services/longhorn.md) | Storage | Distributed storage |
| [services/external-snapshotter](reference/services/external-snapshotter.md) | Storage | Volume snapshots |
| [services/kube-prometheus-stack](reference/services/kube-prometheus-stack.md) | Observability | Prometheus + Grafana |
| [services/loki](reference/services/loki.md) | Observability | Log aggregation |
| [services/tempo](reference/services/tempo.md) | Observability | Distributed tracing |
| [services/mimir](reference/services/mimir.md) | Observability | Long-term metrics |
| [services/opentelemetry-kube-stack](reference/services/opentelemetry-kube-stack.md) | Observability | OTel collectors |
| [services/alert-proxy](reference/services/alert-proxy.md) | Observability | Alert forwarding |
| [services/fluxcd](reference/services/fluxcd.md) | GitOps | Continuous delivery |
| [services/sources](reference/services/sources.md) | GitOps | Shared Flux GitRepository sources |
| [services/weave-gitops](reference/services/weave-gitops.md) | GitOps | GitOps dashboard |
| [services/velero](reference/services/velero.md) | Backup | Disaster recovery |
| [services/etcd-backup](reference/services/etcd-backup.md) | Backup | etcd snapshots |
| [services/headlamp](reference/services/headlamp.md) | Management | K8s dashboard |
| [services/olm](reference/services/olm.md) | Management | Operator Lifecycle |
| [services/postgres-operator](reference/services/postgres-operator.md) | Management | PostgreSQL operator |
| [services/harbor](reference/services/harbor.md) | Management | Container registry |
| [services/kafka-cluster](reference/services/kafka-cluster.md) | Management | Apache Kafka |

### Concepts (Explanations)

| Page | Description |
|------|-------------|
| [architecture](concepts/architecture.md) | System architecture overview |
| [reference-architecture](concepts/reference-architecture.md) | Target cluster architecture |
| [gitops-workflow](concepts/gitops-workflow.md) | GitOps model and FluxCD |
| [configuration-lifecycle](concepts/configuration-lifecycle.md) | Config from init to deploy |
| [security-model](concepts/security-model.md) | Security design and SOPS |
| [security-update-design](concepts/security-update-design.md) | Security update/patch design |
| [services-templates](concepts/services-templates.md) | Template rendering system |
| [drift-detection](concepts/drift-detection.md) | Infrastructure drift |
| [plugin-internal-services](concepts/plugin-internal-services.md) | Internal plugin system |
| [plugin-external-cli](concepts/plugin-external-cli.md) | External CLI plugins |
| [provider-comparison](concepts/provider-comparison.md) | Provider feature matrix |

### Providers

| Page | Description |
|------|-------------|
| [vmware](providers/vmware.md) | VMware provider guide |
| [vmware-quick-start](providers/vmware-quick-start.md) | VMware quick start |
| [vmware-terraform-template](providers/vmware-terraform-template.md) | VMware Terraform |

### Contributing

| Page | Description |
|------|-------------|
| [contributing](contributing/contributing.md) | Contribution guide |
| [development-setup](contributing/development-setup.md) | Dev environment setup |
| [code-structure](contributing/code-structure.md) | Package layout |
| [testing-guide](contributing/testing-guide.md) | Testing approach |
| [adding-providers](contributing/adding-providers.md) | New provider guide (worked example: Magnum) |
| [adding-services](contributing/adding-services.md) | New service guide |
| [build-system](contributing/build-system.md) | Mise build system |
| [release-process](contributing/release-process.md) | Release workflow |
| [validation](contributing/validation.md) | `cluster validate` execution flow |
| [services](contributing/services.md) | Service enable/disable lifecycle |
| [rendering-contract](contributing/rendering-contract.md) | Renderer-owned vs. bootstrap-owned paths, lifecycle states |
| [descriptor-condition-schema](contributing/descriptor-condition-schema.md) | Overlay descriptor condition operators |
| [overlay-security-policy](contributing/overlay-security-policy.md) | Overlay rendering security policy |
| [cluster-init-details](contributing/cluster-init-details.md) | `cluster init` internals |
| [cluster-deploy-openstack](contributing/cluster-deploy-openstack.md) | `cluster deploy` internals (OpenStack) |
| [kind-cluster-verification](contributing/kind-cluster-verification.md) | Kind cluster service verification |

### Release Notes

| Page | Description |
|------|-------------|
| [1.0.0-rc01](release/1.0.0-rc01.md) | Release candidate 1 |
| [1.0.0-rc02](release/1.0.0-rc02.md) | Release candidate 2 |
| [1.0.0-rc03](release/1.0.0-rc03.md) | Release candidate 3 |
| [1.0.0-rc04](release/1.0.0-rc04.md) | Release candidate 4 |
| [1.0.0-rc05](release/1.0.0-rc05.md) | Release candidate 5 |
| [1.0.0-rc06](release/1.0.0-rc06.md) | Release candidate 6 |

### Other

| Page | Description |
|------|-------------|
| [index](index.md) | Documentation home |
| [glossary](glossary.md) | Term definitions |
| [architecture](architecture.md) | Terse, contributor-facing package/entry-point map (distinct in scope from `concepts/architecture.md`) |
| [llm-code-map](llm-code-map.md) | Entry points, package responsibilities, and safe-change boundaries for code-oriented agents |
| [opencenter-cluster-via-cli](opencenter-cluster-via-cli.md) | Full OpenStack cluster walkthrough via the CLI (provider discovery, per-service storage) |

## Non-Published Content

- [`CODEMAPS/`](CODEMAPS/) -- Architecture maps for the development workflow.
  Not part of the published site. See [CODEMAPS/INDEX.md](CODEMAPS/INDEX.md)
  for the full set: CLI commands, cluster lifecycle, config system, DI
  container, GitOps engine, import/operations/resilience, OpenStack provider
  storage operations, providers, rendering ownership and secret artifacts,
  runtime extensions and local development, and secrets management.

## Editing Rules

- Every page must start with YAML frontmatter: `id`, `title`,
  `sidebar_label`, `description`, `doc_type`, `audience`, `tags`.
- Pick exactly one `doc_type` per file. Split mixed content and cross-link.
- Start the body with a `**Purpose:**` line naming the audience and scope.
- Place pages in the lifecycle directory matching the reader's task.
- Refresh the per-command reference under `reference/opencenter/`
  with `go run -tags tools ./cmd/docs` when the Cobra tree changes.
- Every technical claim must be verifiable against the current source --
  when rewriting a page, re-derive facts from the code/schema/CI config
  rather than carrying forward unverified prose from an earlier revision.

## Tooling

- `hack/scripts/audit_doc_frontmatter.py` -- verify frontmatter rules (CI-safe; run via `mise run test-docs-frontmatter`).
- `hack/tag_wip_failures.py` -- tag failing BDD scenarios `@wip` (via `mise run tag-wip-failures`).
