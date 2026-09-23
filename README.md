# openCenter CLI

**openCenter** is a command-line tool that transforms a single declarative YAML configuration into a production-ready Kubernetes cluster with GitOps management.

It standardizes cluster deployment across OpenStack, VMware, Baremetal, Magnum, and Kind, providing configuration validation, secrets management, and automated GitOps repository generation.

## What openCenter Does

- **Configuration-First Workflow:** Single YAML file defines your entire cluster (infrastructure, Kubernetes, services, secrets)
- **Multi-Provider Support:** Deploy to OpenStack, VMware, Baremetal, Magnum (managed OpenStack Kubernetes), or Kind with the same configuration structure
- **Built-in Validation:** Schema validation, business rules, and provider-specific checks catch errors before deployment
- **GitOps Native:** Generates a complete FluxCD-ready repository with Kustomize overlays for cluster-specific customization
- **Secrets Management:** SOPS Age encryption for safe version control of sensitive data
- **Platform Services:** 32 pre-configured managed services (monitoring, logging, ingress, identity, storage, backup) — see [Platform Services](docs/reference/services/index.md)

Not every provider value in the config schema is implemented yet: `aws`, `gcp`, and `azure` are reserved for a future release and rejected by the CLI today. See [Providers Reference](docs/reference/providers.md) for the exact support boundary.

## Quick Start

```bash
# Install tools
mise install

# Build CLI
mise run build

# Initialize cluster
./bin/opencenter cluster init my-cluster --org my-org

# Edit configuration
$EDITOR ~/.config/opencenter/clusters/blueprints/my-org/.my-cluster-config.yaml

# Validate
./bin/opencenter cluster validate my-cluster

# Generate GitOps repository
./bin/opencenter cluster generate my-cluster

# Deploy
./bin/opencenter cluster deploy my-cluster
```

See [Getting Started](docs/getting-started/getting-started.md) for the full walkthrough, including exact timing expectations per provider.

## Key Capabilities

- **Cluster Lifecycle:** Initialize, configure, validate, generate, deploy, destroy
- **Configuration Management:** Schema-driven with defaults, validation, and override capabilities
- **Secrets Operations:** Generate keys, encrypt/decrypt, rotate, check expiration, sync, validate drift
- **GitOps Repository:** Automated generation with infrastructure (OpenTofu) and applications (FluxCD/Kustomize)
- **Provider Abstraction:** Unified configuration structure across OpenStack, VMware, Baremetal, Magnum, and Kind — see [Provider Comparison](docs/concepts/provider-comparison.md)
- **Service Management:** Enable/disable platform services, customize configurations, view options
- **Operational Tools:** Drift detection, backup/restore, audit logging, cluster doctor, import

## Configuration Example

```yaml
opencenter:
  cluster:
    cluster_name: production
    kubernetes:
      version: "1.35.4"
      network_plugin: calico

  infrastructure:
    provider: openstack
    compute:
      master_count: 3
      worker_count: 3
    cloud:
      openstack:
        auth_url: https://identity.api.rackspacecloud.com/v3
        region: sjc3
        application_credential_id: ${OPENSTACK_APP_CRED_ID}
        application_credential_secret: ${OPENSTACK_APP_CRED_SECRET}

  services:
    keycloak:
      enabled: true
    kube-prometheus-stack:
      enabled: true
    loki:
      enabled: true
    velero:
      enabled: true

secrets:
  sops_age_key_file: ~/.config/opencenter/clusters/secrets/age/keys/production-key.txt
```

See [Configuration Schema Reference](docs/reference/configuration-schema.md) for the complete structure and [Default Values](docs/reference/default-values.md) for every default shown above.

## CLI Commands Quick Reference

```bash
# Cluster Lifecycle
opencenter cluster init <name> --org <org>     # Initialize new cluster
opencenter cluster configure <name> --guided   # Guided provider configuration
opencenter cluster validate <name>             # Validate configuration
opencenter cluster generate <name>             # Generate GitOps repository
opencenter cluster deploy <name>               # Deploy cluster
opencenter cluster destroy <name>              # Destroy cluster

# Cluster Management
opencenter cluster list                        # List all clusters
opencenter cluster use <name>                  # Set active cluster
opencenter cluster active                      # Show active cluster
opencenter cluster status <name>               # Show cluster status
opencenter cluster describe <name>             # Detailed cluster description
opencenter cluster doctor                      # Audit local prerequisite binaries

# Configuration
opencenter cluster set <name> <path=value>     # Update configuration value
opencenter cluster edit <name>                 # Edit in $EDITOR
opencenter cluster normalize <name>            # Add missing defaults
opencenter cluster export <name>               # Export effective config

# Service Management
opencenter cluster service enable <svc>        # Enable a platform service
opencenter cluster service disable <svc>       # Disable a platform service
opencenter cluster service status              # Show all service states
opencenter cluster service options <svc>       # Show service config options

# Worker Pool Management
opencenter cluster pool add <name>             # Add a worker pool
opencenter cluster pool update <name>          # Update pool configuration
opencenter cluster pool scale <name> --count=N # Scale pool node count
opencenter cluster pool remove <name>          # Remove pool (requires count=0)
opencenter cluster pool list                   # List all worker pools

# Secrets Management
opencenter secrets keys generate               # Generate Age key pair
opencenter secrets keys rotate --type sops     # Rotate encryption keys
opencenter secrets keys check                  # Check key expiration
opencenter secrets keys backup                 # Backup Age keys
opencenter secrets sync <name>                  # Sync secrets to manifests
opencenter secrets validate <name>              # Validate secrets for drift
opencenter secrets encrypt                      # Encrypt secrets in YAML
opencenter secrets decrypt                      # Decrypt secrets in YAML
opencenter secrets status                       # Show encryption status
opencenter secrets login                        # Refresh Keystone token
opencenter secrets list                         # List secrets
opencenter secrets get <name>                   # Download and decrypt
opencenter secrets set <name>                   # Create or update

# Operations
opencenter cluster drift detect <name>         # Detect infrastructure drift
opencenter cluster drift reconcile <name>      # Reconcile drift
opencenter cluster backup create <name>        # Create backup
opencenter cluster backup restore <id>         # Restore from backup
opencenter cluster lock <name>                 # Lock cluster
opencenter cluster import scan                 # Scan repo for import
opencenter cluster migrate-layout --org <org>  # Migrate to secure layout

# CLI Settings
opencenter settings view                       # Display current settings
opencenter settings set <key> <value>          # Set a value (dot notation)
opencenter settings get <key>                  # Get a value
opencenter settings path                       # Show settings file path
opencenter settings edit                       # Edit settings in editor
opencenter settings ide                        # Generate schema + editor setup
opencenter settings explain                    # Explain config effects

# Plugins
opencenter plugins list                        # List external plugins

# Utilities
opencenter version                             # Show version information
opencenter shell-init                          # Output shell integration script
opencenter --help                              # Show help
```

See [CLI Commands Reference](docs/reference/opencenter/opencenter.md) for the full generated command tree.

## Documentation

Documentation is written in Markdown with YAML frontmatter, organised by
lifecycle category following the [Diátaxis](https://diataxis.fr/) framework.
See [Documentation Home](docs/index.md) for the complete, current site map —
it is regenerated alongside the docs and is the canonical index. Highlights:

- **Getting Started:** [First cluster walkthrough](docs/getting-started/getting-started.md), plus dedicated guides for [Kind](docs/getting-started/kind-local-development.md), [OpenStack](docs/getting-started/openstack-first-cluster.md), [VMware](docs/getting-started/vmware-deployment.md), and [multi-cluster deployments](docs/getting-started/multi-cluster-setup.md).
- **Operations (how-to):** [`docs/operations/`](docs/operations/) — validating config, managing secrets, customizing services, networking, worker pools, backups, upgrades, migrations, troubleshooting, CI/CD, and CLI plugins.
- **Reference:** [CLI commands](docs/reference/opencenter/opencenter.md) (auto-generated), [configuration schema](docs/reference/configuration-schema.md), [providers](docs/reference/providers.md), [services](docs/reference/services/index.md), [environment variables](docs/reference/environment-variables.md), [exit codes](docs/reference/exit-codes.md), and more under [`docs/reference/`](docs/reference/).
- **Concepts (explanation):** [Architecture](docs/concepts/architecture.md), [GitOps workflow](docs/concepts/gitops-workflow.md), [security model](docs/concepts/security-model.md), [configuration lifecycle](docs/concepts/configuration-lifecycle.md), [provider comparison](docs/concepts/provider-comparison.md), and more under [`docs/concepts/`](docs/concepts/).
- **Contributing:** [`docs/contributing/`](docs/contributing/) — development setup, code structure, testing, adding providers/services, release process.
- **Architecture maps for contributors and AI agents:** [`docs/CODEMAPS/`](docs/CODEMAPS/INDEX.md) — package-boundary maps of the runtime, not part of the published doc site.

**Start here:** [Documentation Home](docs/index.md) · [Glossary](docs/glossary.md) · [Docs Layout](docs/README.md)

## Development Workflow

### Prerequisites

- [Mise](https://mise.jdx.dev/) - Tool version manager
- [Git](https://git-scm.com/) - Version control
- Go, kubectl, kind, helm (managed by Mise)

### Build and Test

```bash
# Install tools
mise install

# Build binary
mise run build

# Run unit tests
mise run test

# Run BDD tests
mise run godog

# Run property-based tests
mise run test-properties

# Lint code
mise run lint

# Format code
mise run fmt
```

### Development Tasks

```bash
# Build for multiple platforms
mise run build-all

# Create release
mise run release v1.0.0

# Generate JSON schema
mise run schema

# Validate templates
mise run validate-templates

# Regenerate CLI reference docs (docs/reference/opencenter/**)
mise run docs-gen

# Run a Kind cluster with openCenter-managed CNI
opencenter cluster init dev-cluster --type kind --kind-disable-default-cni
opencenter cluster validate dev-cluster
opencenter cluster generate dev-cluster
opencenter cluster deploy dev-cluster

# Setup local Gitea for testing
mise run gitea-up
```

See [Mise Tasks Reference](docs/reference/mise-tasks.md) for the complete list.

Tagged releases are published by GitHub Actions. Use `mise run release` for local preflight builds, then push a `v*` tag to create the signed release artifacts.

## Project Structure

```
openCenter-cli/
├── cmd/                    # CLI commands (Cobra)
│   ├── root.go            # Root command and global flags
│   ├── cluster*.go        # Cluster lifecycle commands
│   ├── secrets*.go        # Secrets management commands
│   ├── config*.go         # Settings commands (Cobra Use: "settings")
│   ├── docs/generate.go   # CLI reference doc generator (mise run docs-gen)
│   └── plugins.go         # Plugin management
├── internal/              # Internal packages
│   ├── config/           # CLI settings, thin v1-compat layer over config/v2
│   ├── config/v2/        # Authoritative cluster configuration: loader, defaults, validation
│   ├── cluster/          # Cluster lifecycle services (init, validate, bootstrap, per-provider steps)
│   ├── gitops/           # GitOps repository generation (render catalog, templates, ownership)
│   ├── secrets/          # Multi-cluster secrets management (rotation, registry, reconcile)
│   ├── secretartifacts/  # Logical-secret to physical-artifact planning
│   ├── sops/              # SOPS encryption (Age keys, file encrypt/decrypt)
│   ├── cloud/             # Provider adapters (OpenStack, VMware, Kind, Magnum)
│   ├── credentials/       # Cloud credential extraction
│   ├── barbican/          # OpenStack Key Manager client
│   ├── security/          # Audit logging, input validation, command sanitization
│   ├── di/                # Dependency injection container (typed app graph)
│   ├── services/          # Platform service descriptors, plugin registry
│   ├── operations/        # Drift detection and backup interfaces/implementations
│   ├── resilience/        # Retry, circuit breaker, distributed locks
│   ├── provision/         # Embedded OpenTofu provisioning templates
│   ├── template/          # Template engine with caching and sandboxing
│   ├── plugins/           # External CLI plugin discovery
│   ├── importer/          # Live cluster/GitOps repo import and scan
│   ├── localdev/          # Local dev environment (Kind, Gitea, Flux)
│   ├── logging/           # Structured logging
│   ├── tofu/              # OpenTofu/Terraform execution
│   ├── ui/                # Prompts, error formatting, guided flows
│   ├── core/               # Shared: path resolution, validation engine
│   └── util/               # Files, errors, crypto helpers
├── docs/                  # Documentation (Markdown with YAML frontmatter)
│   ├── README.md          # Layout, build, and editing rules
│   ├── index.md           # Documentation home
│   ├── glossary.md        # Term definitions
│   ├── getting-started/   # Tutorials (doc_type: tutorial)
│   ├── operations/        # How-to guides (doc_type: how-to)
│   ├── reference/         # Reference (doc_type: reference)
│   │   ├── opencenter/    # Auto-generated Cobra command pages
│   │   └── services/      # Per-service reference docs
│   ├── concepts/           # Explanations (doc_type: explanation)
│   ├── providers/           # Per-provider how-to guides
│   ├── contributing/         # Contributor docs
│   ├── release/               # Release notes
│   └── CODEMAPS/               # Architecture maps (not part of the published site)
├── tests/                 # BDD tests (Godog)
│   └── features/         # Gherkin feature files
├── schema/                # JSON schema definitions
├── hack/                  # Development scripts and local Gitea setup
├── .mise.toml            # Mise configuration and tasks
├── go.mod                # Go module definition
└── main.go               # CLI entrypoint
```

See [Code Structure](docs/contributing/code-structure.md) and [Codemaps](docs/CODEMAPS/INDEX.md) for the detailed explanation.

## Configuration File Locations

openCenter resolves each directory the same way: an `OPENCENTER_*_DIR` environment variable, then a value in the CLI settings file, then a computed default.

| Role | Env var | Default |
| --- | --- | --- |
| Config dir | `OPENCENTER_CONFIG_DIR` | `~/.config/opencenter` (platform-specific equivalent) |
| Clusters dir | `OPENCENTER_CLUSTERS_DIR` | `<config-dir>/clusters` |
| GitOps dir | `OPENCENTER_GITOPS_DIR` | `<clusters-dir>/gitops` |
| Blueprints dir (cluster config files) | `OPENCENTER_BLUEPRINTS_DIR` | `<clusters-dir>/blueprints` |
| Cluster state dir | `OPENCENTER_CLUSTER_STATE_DIR` | `<clusters-dir>/state` |
| Secrets dir | `OPENCENTER_SECRETS_DIR` | `<clusters-dir>/secrets` |
| Plugins dir | `OPENCENTER_PLUGINS_DIR` | `<config-dir>/plugins` |

See [File Locations Reference](docs/reference/file-locations.md) for the full per-cluster layout and resolution order.

## Environment Variables

openCenter reads several environment variables beyond the path-resolution ones above:

| Variable | Description | Default |
|----------|-------------|---------|
| `OPENCENTER_LOG_LEVEL` | Default log level (`debug`, `info`, `warn`, `error`); only applies if `--log-level` wasn't passed | `warn` |
| `OPENCENTER_CLUSTER` | Active-cluster override | none |
| `SOPS_AGE_KEY_FILE` | Path to an Age private key file, for SOPS operations outside the per-cluster key layout | none |
| `KUBECONFIG` | Kubernetes config file | `~/.kube/config` |

See [Environment Variables Reference](docs/reference/environment-variables.md) for the complete, source-verified list.

## Contributing

We welcome contributions. See the [Contributing Guide](docs/contributing/contributing.md) to get started.

### Quick Contribution Workflow

1. Fork and clone the repository
2. Create a feature branch
3. Make your changes
4. Run tests: `mise run test && mise run godog`
5. Submit a pull request

### Extension Points

- **Custom Providers:** Add new infrastructure providers in `internal/cloud/<provider>/` — see [Adding Providers](docs/contributing/adding-providers.md) (worked example: the Magnum provider)
- **Custom Services:** Add platform services via a descriptor in `internal/services/descriptors/data/` or a plugin in `internal/services/plugins/` — see [Adding Services](docs/contributing/adding-services.md)
- **Custom Validators:** Add validation rules in `internal/core/validation/`
- **Plugins:** Create external plugins as `opencenter-<plugin>` executables

See the [contributing pages](docs/contributing/) for detailed guides.

## License

This project is licensed under the Apache 2.0 License. See [LICENSE](LICENSE) for details.

## Support

- **Documentation:** [docs/](docs/)
- **Security Policy:** [SECURITY.md](SECURITY.md)
- **Issues:** [GitHub Issues](https://github.com/opencenter-cloud/openCenter-cli/issues)
- **Discussions:** [GitHub Discussions](https://github.com/opencenter-cloud/openCenter-cli/discussions)

## Related Projects

openCenter CLI is part of the openCenter ecosystem:

- **[openCenter-gitops-base](https://github.com/opencenter-cloud/openCenter-gitops-base)** - Platform services library with security-hardened Helm values
- **[openCenter-customer-app-example](https://github.com/opencenter-cloud/openCenter-customer-app-example)** - Reference application deployment patterns
- **[openCenter-AirGap](https://github.com/opencenter-cloud/openCenter-AirGap)** - Air-gapped deployment packaging
- **[opencenter-windows](https://github.com/opencenter-cloud/opencenter-windows)** - Windows worker node support
