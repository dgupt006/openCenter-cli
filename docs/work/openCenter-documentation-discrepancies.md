# openCenter Documentation Discrepancies

## Scope and authority

Compared on 2026-07-31 at commit `df18cf04013a7063ff2b30f1bb368407c6fbec84`. Runtime behavior of the freshly built CLI remains authoritative, followed by source, schema/tests/fixtures, `mise` tasks, and documentation. Hosted pages were fetched from `https://docs.opencenter.cloud`; their unavailable `.dev` canonical links were not used as evidence.

## Material discrepancies

| Area | Existing documentation/task behavior | Current evidence and execution decision |
| --- | --- | --- |
| Installation/build | Hosted quickstart suggests downloading v1.0.0 or direct `go install`; CLI reference suggests direct `go run`. | Current checkout defines Go 1.26.4 and `build-cli` in `.mise.toml`. This run uses only `mise install` and `mise run ...`; artifact path is `./bin/opencenter`. |
| Deployment workflow | Hosted Kind/OpenStack pages use the removed setup/render and bootstrap subcommands under `cluster`. | Neither subcommand is registered in the current Cobra hierarchy. Current lifecycle is validate, doctor, generate/dry-run deploy, deploy, status, and resumable deploy flags. Removed commands are not used. |
| Active target choice | Troubleshooting uses the removed select subcommand under `cluster`. | Current command is `cluster use`; `cluster active` and `cluster list` are also registered. Runtime help will decide exact use. |
| Configuration mutation | `schema-verify` invokes the removed update subcommand with a legacy dotted provider flag. | No current update subcommand exists under `cluster`. Configuration commands are `configure`, `edit`, `set`, and `normalize`. AWS is explicitly rejected as planned. |
| Provider list | Schema enum includes OpenStack, AWS, GCP, Azure, bare metal, vSphere, VMware, and Kind. | Provider availability code supports OpenStack, VMware/vSphere, Kind, and bare metal; AWS/GCP/Azure are planned and rejected. Documentation must distinguish schema vocabulary from implemented providers. |
| Kind lifecycle | Hosted Kind install uses direct `kubectl` and `kind delete cluster`. | Operator-side direct Kubernetes/runtime tooling is prohibited for this run. Only generated-CLI deploy/status/diagnostic/destroy capabilities may be used. |
| OpenStack lifecycle | Hosted OpenStack install invokes OpenStack CLI and direct Terraform init/apply. | Current generated CLI owns provider provisioning and internally orchestrates its implementation. No direct provider or IaC invocation is allowed. |
| Post-deploy validation | Hosted guides use direct `kubectl get nodes` and `flux get all`; service troubleshooting adds Helm. | Validation must be performed through generated-CLI status/service/diagnostic commands. Missing proof is a CLI capability defect, not permission to bypass it. |
| Preflight task | `.mise.toml` defines a stale task for the removed preflight subcommand under `cluster`. | Current CLI registers `cluster doctor`. The stale task is not used; cluster operations also must not be wrapped in `mise`. |
| Schema verification task | `schema-verify` catches schema/init/update/validation/test failures with `|| echo`, uses the removed update subcommand, and selects unsupported AWS. | It is stale and fail-open, so it cannot be an artifact validation gate. Current fail-closed `schema-v2`, lint, test, and verify tasks are used. A future task repair should delete unsupported flows and propagate every failure. |
| Lint task | `.mise.toml` invoked a repository-local `bin/golangci-lint` that no task generated and no tool declaration installed. | The smallest task repair pins repository-evidenced golangci-lint v2.11.4 and invokes it from mise's managed PATH. The repaired task is operational but fails on the documented 102-finding baseline; this run does not represent that baseline as a passing gate. |
| CLI validation semantics | Documentation treats successful status commands as sufficient proof. | `cluster status --refresh` checks live API/nodes, but current source cannot prove all required pod/component categories. `status --sync` records pending/failed readiness without reliably producing a failing exit. Mandatory health proof may require a small tested CLI enhancement. |
| Sync command discoverability | Source implements `NewClusterSyncStatusCmd`. | Its standalone sync-status form is not registered under `cluster`; the current public path is `cluster status --sync`. |
| Troubleshooting flags | Hosted troubleshooting mentions `validate --verbose`, `init --regenerate-keys`, `rotate-keys`, and `setup --log-level debug`. | Current generated help must be used. The command audit found no `rotate-keys` or `setup`; documented flags are not assumed. |
| Recommended next steps | Current status/deploy output can recommend `eval`, installed `opencenter`, direct `kubectl`, or direct Git actions. | Those messages conflict with this controlled process. They are informational only and are not executed. Reproducible instructions use explicit `./bin/opencenter`. |
| Config layout | Older material uses legacy flat or user-selected layouts. | Current init uses the v2 blueprint-oriented directory under the openCenter config root, and also supports an explicit config file. Runtime CLI resolution will determine the active target without exposing secrets. |
| Kubernetes defaults | General defaults and provider-specific documentation can diverge. | Current Kind code defaults to Kubernetes 1.35.0, while other current default paths reference 1.35.4. The active configuration and runtime status, not docs, decide the validation target. |

## Documentation pages inspected

- Welcome/quickstart
- Kind installation
- First OpenStack cluster
- OpenStack installation
- CLI reference
- CLI troubleshooting
- Service troubleshooting
- Checked-in getting-started, provider, operations, generated command, schema, and `mise` reference documentation

## Deferred documentation correction

The proven how-to must not be created until deployment, full health validation, and safe idempotency validation succeed. If the literal input placeholders cannot be resolved through the generated CLI, the how-to remains intentionally absent rather than describing an unexecuted process.
