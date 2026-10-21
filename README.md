<!-- last_updated: 2026-09-25 -->

# openCenter CLI

openCenter CLI manages declarative Kubernetes cluster configurations and
generates GitOps assets for provider-specific cluster workflows. The built-in
provider paths are OpenStack, VMware, Baremetal, Kind, and Magnum; AWS, GCP,
and Azure are rejected as unavailable providers.

## What it does

- Creates and validates schema 2.0 cluster configuration.
- Generates infrastructure, Flux, and application-overlay assets.
- Runs cluster lifecycle, service, worker-pool, backup, drift, and import
  operations.
- Provides SOPS/Age secret and key-management commands.

## Build and try it

This repository builds the CLI from source with [mise](https://mise.jdx.dev/):

```bash
mise install
mise run build

./bin/opencenter cluster init my-cluster --org my-org --type kind
# Edit the generated configuration, then:
./bin/opencenter cluster validate my-cluster
./bin/opencenter cluster generate my-cluster
./bin/opencenter cluster deploy my-cluster
```

Provider prerequisites and configuration details vary. Start with the
[documentation index](docs/index.md), especially [Getting Started](docs/getting-started/getting-started.md)
and the [provider reference](docs/reference/providers.md).

### Deploy prerequisites

Before changing infrastructure, `cluster deploy` verifies that SOPS is
available in `PATH` and that Helm 4 is available. Missing prerequisites are
reported together so they can be installed before retrying the deployment.

## Documentation

- [Documentation index](docs/index.md) — reader-facing source index.
- [CLI command reference](docs/reference/cli-commands.md) — command groups,
  flags, and generated per-command references.
- [Configuration schema](docs/reference/configuration-schema.md) — fields and
  validation surface.
- [Provider reference](docs/reference/providers.md) — provider boundaries.
- [Documentation maintenance guide](docs/README.md) — contributor workflow.
- [Glossary](docs/glossary.md) — repository terminology.

Code-oriented navigation is available in [llms.txt](llms.txt). It complements
the documentation index and is not a replacement for command reference pages.

## Development

See [Development Environment Setup](docs/contributing/development-setup.md),
[Testing Guide](docs/contributing/testing-guide.md), and
[Contributing](docs/contributing/contributing.md).

## Support and security

- [GitHub Issues](https://github.com/opencenter-cloud/openCenter-cli/issues)
- [Security policy](SECURITY.md)

## License

Licensed under the [Apache License 2.0](LICENSE).
