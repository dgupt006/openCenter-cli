---
id: refactor-audit
title: "Understand the Refactor Audit"
sidebar_label: Refactor Audit
description: Records repository structure, duplication findings, risk assessments, and proposed refactoring boundaries for the openCenter CLI.
doc_type: explanation
audience: "contributors, maintainers"
tags: [refactoring, audit, architecture, maintenance]
---
# Refactor audit report

> Intended path: `docs/refactor-audit-report.md`  
> Audit date: 2026-07-30  
> Scope: read-only inspection of `github.com/opencenter-cloud/opencenter-cli`  
> Audited revision: `94d8cb5` (`main`, one commit ahead of `upstream/main`)

## 1. Executive summary

The repository is a single-module Go CLI with 65 default-build packages and approximately 224,000 lines of Go. Its main architecture is sound: Cobra commands delegate into `internal/` domain packages, configuration v2 provides the primary configuration pipeline, and existing code maps document several major subsystems.

The strongest findings are:

- The test, race, vet, package-load, dependency-load, and module-tidiness baselines pass when run with Go 1.26.5.
- The shell-selected Go executable reports 1.26.4 while cached build artifacts and related tools use 1.26.5. This initially caused repository-wide compiler-version failures unrelated to the source.
- `deadcode -test ./...` reports 216 unreachable symbols. Many are exported-but-internal feature families, interface methods, examples, reflection-adjacent code, or test infrastructure and must not be removed mechanically.
- The safest initial removals are a no-op self-assignment, an unused test helper, and a small number of isolated internal wrappers or complete unused adapter families after one final reference check.
- `golangci-lint` reports 102 issues: 50 `errcheck`, 50 `staticcheck`, and 2 `ineffassign`. These should be handled as independent correctness/maintenance work, not mixed wholesale into dead-code removal.
- The only production build tag is the documentation generator’s `tools` tag. The `perf` test tag is currently broken and references APIs that no longer exist.
- No tracked Go file identifies itself as generated, and there are no `go:generate` directives. Generation still exists through `.mise.toml`, the documentation generator, embedded templates, service descriptors, schema output, and a fixture generator.
- There is no evidence of gRPC servers, protobuf definitions, Go HTTP servers, Kubernetes controllers, webhooks, CRDs, or database migration frameworks. Kubernetes manifests and service descriptors are nevertheless dynamic compatibility surfaces.
- The most concrete semantic duplication defect is secrets code that reconstructs `~/.config/opencenter` instead of using the canonical configuration path system.
- Several apparent duplicates—atomic writes, state-directory resolution, retries, and reflection traversal—have materially different contracts. They should not be merged merely because their implementations look similar.
- No `pkg/` extraction is warranted. All identified reusable code remains specific to this CLI and should stay under `internal/`.

The worktree already contained a modification to `.mise.toml`. This audit did not change it or any other repository file.

## 2. Repository map

### Module and workspace boundaries

| Item | Finding |
|---|---|
| Go workspace | No `go.work` file found |
| Module | `github.com/opencenter-cloud/opencenter-cli` |
| Module definition | Root `go.mod` |
| Go directive | `go 1.26.4` |
| Self replacement | The module replaces itself with `.` |
| Default-build packages | 65 |
| Public `pkg/` tree | None |
| Reuse boundary | `internal/` throughout |
| Host inspected | Darwin/arm64 with CGO enabled |
| Repository size | 1,095 tracked files; 703 Go files; approximately 223,782 Go lines |

### Major source areas

| Area | Responsibility |
|---|---|
| `main.go` | Primary CLI entrypoint and initial application wiring |
| `cmd/` | Cobra command definitions, flags, console interaction, and some remaining workflows |
| `cmd/opencenter-local/` | Separate local-development executable |
| `cmd/docs/` | Build-tagged CLI documentation generator |
| `hack/` | Standalone fixture-generation program |
| `internal/di/` | Typed application dependencies plus legacy reflection-based container support |
| `internal/config/` | Configuration paths, settings, compatibility, persistence, flags, defaults, overlays, registry, and services |
| `internal/config/v2/` | Authoritative v2 parse, normalize, resolve, default, and validate pipeline |
| `internal/core/paths/` | Low-level path discovery and validation |
| `internal/cluster/` | Cluster lifecycle, configuration, bootstrap, initialization, and destruction |
| `internal/cluster/orchestration/` | Higher-level lifecycle orchestration |
| `internal/cluster/provider/openstack/`, `internal/cluster/storage/openstack/` | Typed OpenStack provider planning and explicit one-service storage provisioning |
| `internal/cloud/` | Kind, OpenStack, and VMware integrations |
| `internal/gitops/` | GitOps generation, stages, workspaces, atomic transactions, and templates |
| `internal/template/` | Template rendering, composition, metadata, sandboxing, and dependency concepts |
| `internal/secrets/` | Secret management, multi-cluster handling, rotation, backup, and auditing |
| `internal/sops/` | SOPS and age key operations |
| `internal/credentials/` | Cloud credential parsing and conversion |
| `internal/plugins/` | External executable plugin discovery and loading |
| `internal/localdev/` | Local Flux, Gitea, API, and GitOps development services |
| `internal/operations/` | Reusable operational workflows, including drift-related functionality |
| `internal/services/` | Service descriptors and plugin-related service handling |
| `internal/provision/` | Provisioning and embedded templates |
| `internal/tofu/` | OpenTofu integration |
| `internal/ui/` | Terminal UI behavior |
| `internal/util/` | Existing filesystem, crypto, error, and security support packages |
| `internal/testing/`, `internal/testenv/` | Repository test infrastructure |
| `tests/features/steps/` | Godog/BDD step implementations |
| `docs/CODEMAPS/` | Existing subsystem code maps |

### Important dependency direction

The observed dependency direction is broadly:

```text
main
  -> config path bootstrap
  -> di application construction
  -> cmd Cobra composition

cmd
  -> cluster / operations / config / gitops / secrets / sops / plugins
  -> UI and output adapters

cluster
  -> config/v2
  -> cloud providers
  -> gitops
  -> provisioning

gitops
  -> config/v2
  -> template
  -> filesystem abstractions

secrets and sops
  -> config/v2 and configuration paths
  -> crypto and filesystem support

config/v2
  -> lower-level validation, path, and error support
```

The intended layering is sometimes obscured by large command files and by the coexistence of typed dependency injection with the legacy reflection container.

### Tests and non-production assets

The repository contains:

- Unit tests and external-package tests.
- Property tests.
- Integration-oriented tests.
- Benchmarks.
- Go example functions.
- Godog feature files and step definitions.
- Fixtures, golden files, embedded test templates, and testdata.
- Provider-specific tests and local-development tests.

Example functions without `// Output:` comments are reported as unreachable by `deadcode`; they remain documentation/test assets and are not ordinary production dead code.

## 3. Commands, entrypoints, and runtime flows

### Executable entrypoints

| Entrypoint | Purpose |
|---|---|
| `main.go` | Main `opencenter` CLI |
| `cmd/opencenter-local/` | Local-development CLI |
| `cmd/docs/generate.go` | Documentation generator under `//go:build tools` |
| `hack/generate_relaypoint_fixture_configs.go` | Standalone fixture generator |

### Primary CLI flow

1. `main.go` resolves the clusters/configuration directory.
2. Application dependencies are assembled through `internal/di`.
3. `cmd.ExecuteWithContext` builds and executes the Cobra hierarchy.
4. Root setup pre-parses `--config-dir`, installs application dependencies, and registers the command tree.
5. External executable plugins are discovered and loaded.
6. Commands delegate to cluster, configuration, GitOps, secrets, SOPS, provider, and operation packages.
7. Configuration commands predominantly use the v2 parse/normalize/reference-resolution/default/validation flow.

### Main command surface

Top-level commands include:

- `cluster`
- `settings`
- `secrets`
- `plugins`
- `version`
- `shell-init`

Important `cluster` commands include:

- `list`, `use`, `active`, `env`, `status`, `describe`
- `init`, `configure`, `edit`, `set`, `normalize`
- `export`, `validate`, `doctor`, `generate`, `deploy`
- `template`, `destroy`, `service`, `pool`, `drift`
- `backup`, `lock`, `unlock`, `import`
- `migrate-layout`, `sync`, `validate-manifests`

The `settings`, `secrets`, service, pool, drift, backup, import, and sync commands also expose nested subcommands.

### Configuration flow

The principal configuration flow is:

```text
CLI flags and environment
  -> configuration path resolution
  -> YAML/JSON parsing
  -> v2 normalization
  -> reference resolution
  -> defaults
  -> validation
  -> domain operation
```

Compatibility risks remain in:

- Deprecated configuration fields.
- V1-to-v2 migration behavior.
- Reflection-driven flag integration.
- Registry initialization through `init`.
- Manually constructed paths outside the canonical path packages.

### Workers, services, and network endpoints

- Local-development packages manage Gitea, Flux, API, and GitOps-related services.
- Drift and similar operations contain scheduling/background behavior.
- Cloud provider packages perform outbound API calls.
- HTTP client use exists, but no Go HTTP server registration was found.
- No gRPC server registration or protobuf service implementation was found.
- No controller-runtime controller, Kubernetes webhook server, CRD generator, or database migration framework was found.

## 4. Public API and compatibility surfaces

Most exported Go symbols are under Go’s `internal/` boundary and cannot be imported by unrelated modules. That reduces external Go API risk but does not make exported symbols automatically removable.

The effective public and compatibility surfaces are:

1. **CLI contract**

   Command names, aliases, flags, defaults, help text, positional arguments, output formats, exit behavior, shell completion, and shell initialization output.

2. **Configuration contract**

   YAML and JSON field names, tags, accepted legacy forms, defaults, validation rules, references, normalization output, and `schema/opencenter-v2.schema.json`.

3. **Environment contract**

   Runtime literals found in code and documentation include:

   - `OPENCENTER_ACTIVE_CLUSTER`
   - `OPENCENTER_CONFIG_DIR`
   - `ALLOW_INSECURE_FILE_MODES`
   - `BLUEPRINTS_DIR`, `CACHE_FILE`, `CLUSTER`, `CLUSTERS_DIR`
   - `CLUSTER_STATE_DIR`, `CONFIG_DIR`, `GITOPS_DIR`, `PLUGINS_DIR`
   - `SECRETS_DIR`, `STATE_DIR`
   - `DEBUG`, `LOG_LEVEL`, `SKIP_HOOKS`, `WORKER_COUNT`
   - `SESSION_FILE`, `SESSION_ID`, `TEST_MODE`
   - `SECURE_*` variables
   - AWS credential, profile, and region variables
   - OpenStack `OS_*` and `OPENSTACK_*` variables
   - `AGE_KEY`, `SOPS_AGE_KEY`, `SOPS_AGE_KEY_FILE`, `SOPS_AGE_RECIPIENTS`, `SOPS_CONFIG`
   - `VSPHERE_SERVER`, `VSPHERE_USER`, `VSPHERE_PASSWORD`, and TLS verification settings

   Some literals are test-only or CI-only; each must be checked at the call site before changing it.

4. **Filesystem contract**

   Cluster layout, active-cluster state, configuration/state/cache paths, GitOps paths, generated filenames, secret stores, lock files, backups, and migration behavior.

5. **Plugin contract**

   External executables are discovered dynamically. Search paths, executable names, metadata, argument conventions, environment, and output parsing are compatibility surfaces. Documentation generation also loads discovered plugins.

6. **Embedded-data contract**

   Embedded GitOps templates, provisioning templates, service descriptor IDs, template filenames, and renderer registration names can be selected through configuration strings rather than Go references.

7. **Reflection and interface contracts**

   Reflection is used in configuration flags, v2 schema/reference handling, cluster configuration, DI, importer logic, and service descriptors. Error interfaces, `fmt.Stringer`, validator interfaces, and similar method sets may be invoked without an explicit textual call.

8. **Initialization registration**

   Service and renderer registration performed from `init` functions is dynamically reachable. Registration files must not be removed based only on static call searches.

No `go:linkname` use was found.

## 5. Generated-code and code-generation areas

### Generated Go detection

The audit found:

- No tracked Go file containing the standard `Code generated ... DO NOT EDIT.` marker.
- No `//go:generate` directive.
- No protobuf, gRPC, GraphQL, sqlc, ent, mockgen, clientset, informer, lister, deepcopy, controller-gen, or OpenAPI-generated Go source.
- No root Makefile, Taskfile, Magefile, or Justfile governing generation.

Therefore, no tracked Go source is currently classified as generated. This does not eliminate non-Go generation risks.

### Generation inputs and outputs

| Area | Evidence | Risk |
|---|---|---|
| JSON Schema | `.mise.toml` tasks and `schema/opencenter-v2.schema.json` | Generated artifact can drift from v2 types |
| CLI reference docs | `cmd/docs/generate.go`, `docs/reference/opencenter/` | Generator loads external plugins, so output may depend on host state |
| Fixture configs | `hack/generate_relaypoint_fixture_configs.go` | Test fixture generation path |
| Embedded templates | `go:embed` in GitOps, provisioning, template tests, service descriptors, and shell-init code | Names are runtime compatibility surfaces |
| Service descriptors | Embedded YAML under service descriptor packages | Configuration selects descriptors dynamically |
| Shell initialization | Embedded shell files under `cmd/` | User-facing generated output |

### Stale or unsafe generation paths

The current `.mise.toml` contains generation-related tasks with signs of drift:

- A schema task invokes a removed schema-related cluster subcommand, but no current equivalent was found.
- A schema generator task references `cmd/schema-gen/main.go`, which does not exist.
- The v2 schema task temporarily creates a Go test file, generates the schema, and removes the temporary file.
- The documentation task runs `go run cmd/docs/generate.go`.
- A cleanup task includes broad removal targets such as `testdata`, which is unsafe because tracked testdata exists.

Generated artifacts and embedded inputs are unsafe for manual deletion until their authoritative inputs and reproducible regeneration commands are repaired and documented.

## 6. Build tags and platform-specific code

| Location | Tag | Finding |
|---|---|---|
| `cmd/docs/generate.go` | `tools` | Documentation generator; not part of the default build |
| `internal/config/memory_target_test.go` | `perf` | Stale performance test target |

No OS-specific files such as `*_linux.go`, `*_darwin.go`, or `*_windows.go` were found. No architecture-specific `*_amd64.go` or `*_arm64.go` files were found.

`deadcode -tags=tools -test ./...` did not materially change the unreachable-symbol result.

The `perf` tag does not compile. It references APIs that no longer exist:

- `GetCachedDefaultConfig`
- `OptimizedYAMLMarshal`
- `Config`
- `OptimizedYAMLUnmarshal`
- `GetMemoryPool`
- `GetAllocationOptimizer`

The tagged test is not safe for automatic deletion. A maintainer must decide whether it represents a still-required performance contract. It should either be updated to the current configuration API or deliberately retired with documented rationale.

No enterprise, cloud, e2e, acceptance, experimental, OS, or architecture build tags were found.

## 7. Test and lint baseline

The matching Go 1.26.5 binary was:

```text
/Users/victor.palma/.local/share/mise/installs/go/1.26.5/bin/go
```

| Command | Result |
|---|---|
| Shell-selected `go test ./...` | Failed because Go 1.26.4 attempted to consume Go 1.26.5 build artifacts |
| Go 1.26.5 `go test ./...` | Passed |
| Go 1.26.5 `go test -race ./...` | Passed; no races reported |
| Go 1.26.5 `go vet ./...` | Passed |
| Go 1.26.5 `go list ./...` | Passed; 65 default-build packages |
| Go 1.26.5 `go list -deps ./...` | Passed |
| Go 1.26.5 `go mod tidy -diff` | Passed with no diff |
| Standalone `staticcheck ./...` | Unavailable; not installed as a standalone executable |
| `golangci-lint run` | Ran with v2.11.4; failed with 102 findings |
| `deadcode -test ./...` | Ran; 216 unreachable-symbol findings |
| `dupl` | Unavailable |
| `govulncheck` | Unavailable locally; CI installs it |
| Coverage artifact inspection | No tracked coverage profile or report found |
| `perf` tagged test | Does not compile due to stale API references |

No `.golangci.yml`, `.golangci.yaml`, `.golangci.toml`, or `.golangci.json` was found, so lint behavior currently depends on tool defaults.

The 102 lint findings consist of:

- 50 `errcheck`
- 50 `staticcheck`
- 2 `ineffassign`

Notable production findings include:

- Unchecked close, remove, environment, and response-body operations.
- A self-assignment in `internal/importer/scanner.go`.
- An empty branch in `internal/config/v2/loader.go`.
- An empty validation branch in GitOps code.
- Deprecated APIs such as `strings.Title`, `io/ioutil`, and Redis `SetNX`.
- Potentially misleading error capitalization and boolean expressions.
- Staticcheck warnings around compatibility fields and copied flag values.

Tests also contain unchecked environment changes, possible nil dereferences, a malformed JSON tag, and deprecated Godog use.

The repository’s CI tests selected package groups, race behavior, vet, and some property tests. A separate workflow installs `govulncheck`. No existing coverage file was available to establish a line or branch coverage baseline.

## 8. Dead-code candidate matrix

### Search methodology

For each candidate family, exact symbol names and related strings were checked across:

- Go source and tests.
- Tagged source.
- YAML, JSON, TOML, INI, HCL, templates, and embedded data.
- Shell scripts and CI workflows.
- Documentation and code maps.
- Registration maps, `init` functions, reflection sites, interface implementations, and plugin code.

`deadcode` reachability is supporting evidence, not sole deletion authority. The matrix groups symbols that share the same implementation, reachability evidence, and recommended action.

| Candidate | Package and location | Why it appears unused; evidence and search terms | Risk category | Recommended action |
|---|---|---|---|---|
| Self-assignment `result.Confidence = result.Confidence` | `internal/importer/scanner.go:411` | `ineffassign` identifies a semantic no-op. Search: `Confidence = result.Confidence`. | **Safe to remove** | Remove as an isolated no-op and rerun importer tests. |
| `NewLegacyConfigurationMerger` | `internal/config/flags/backward_compatibility_property_test.go:315` | Test-only helper reported unreachable; exact-name search found no caller. | **Safe to remove** | Remove in Prompt 2 with the containing property tests run before and after. |
| `DefaultsCache` constructor and all five methods | `internal/config/cache/defaults.go:18-59` | `NewDefaultsCache`, `GetDefaultConfig`, `Invalidate`, `InvalidateAll`, and `Stats` are unreachable. Searches found declarations and code-map mentions but no construction or calls. The cache’s completion state is not populated by live code. | **Probably safe but requires confirmation** | Confirm no planned cache rollout; delete the complete family as one batch rather than leaving a partial type. |
| `NewDestroyServiceWithRunner` | `internal/cluster/destroy_service.go:57` | Exact-name search found no production or test caller. The alternate constructor is described as test-oriented but is not used by tests. | **Probably safe but requires confirmation** | Confirm it is not intended as an injection seam; otherwise remove and preserve the live constructor. |
| `config/persistence.ResolveStateDir`, `MarshalYAML`, `UnmarshalYAML` | `internal/config/persistence/paths.go:88`; `yaml.go:9-16` | Exact searches found no calls. A different live `config.ResolveStateDir` has broader behavior, so the two same-named functions are not equivalent. | **Probably safe but requires confirmation** | Remove only these unreachable wrappers. Do not redirect callers or merge state-directory semantics. |
| `registry.IsRegistered` | `internal/config/registry/registry.go:42` | Exact search found no caller. Registry contents are populated dynamically, but this query helper is not referenced. | **Probably safe but requires confirmation** | Confirm no external tooling relies on source-level access; then remove only the query helper. |
| `ErrorBuilder` unused methods, templated error creation, and unused `ErrorReporter` queries | `internal/config/flags/errors.go` | `WithCode`, `WithPath`, `Build`, `CreateTemplatedError`, `AddError`, `Clear`, `GetErrorsByType`, and `GetErrorsByPath` are unreachable. | **Probably safe but requires confirmation** | Treat as a cohesive flags-error API review. Remove only after confirming reflection and interface registration do not retain the methods. |
| Output-format methods | `internal/config/flags/integration.go:307-339`; `output_formatter.go:361` | `CLIIntegration.FormatOutput`, `FormatDiff`, `FormatConflicts`, and `ParseOutputMode` have no static callers. Related output strings and flag values were searched. | **Probably safe but requires confirmation** | Verify no CLI mode is intended to expose them, then remove as one behavior-neutral batch. |
| Security flag interface methods and handlers | `internal/config/flags/interfaces.go`; `security_flag_handler.go` | Five `GetSecurityType` methods and the handler’s parse/capability methods are unreachable. They implement interface-shaped APIs and are adjacent to reflection. | **Keep because public/API/dynamic/generated/build-tag/test-only usage is possible** | Do not delete in Prompt 2. First document actual handler registration and remove the obsolete interface or implementation only with characterization tests. |
| Enhanced flag parser/reflection methods | `internal/config/flags/parser.go:69`; `reflection_engine.go:507` | `SetPrecedence` and `SupportedSyntax` have no explicit caller, but their packages are reflection-heavy. | **Keep because public/API/dynamic/generated/build-tag/test-only usage is possible** | Leave until the flags architecture is simplified and the reflective contract is covered. |
| `SecureTemplateProcessor` unused method family | `internal/config/flags/secure_template_processor.go` | Seven methods, including environment loading, validation, masking, and state queries, are unreachable. Search terms included each method and associated secure-variable strings. | **Unsafe to remove** | Security and environment semantics require an owner decision and focused tests before removal. |
| Entire unused `SOPSIntegration` method family | `internal/config/flags/sops_integration.go` | Fourteen methods are unreachable, including encryption, parsing, serialization, age-key loading, status, and inspection. No live constructor-to-method path was found. | **Unsafe to remove** | Determine whether this is an abandoned integration or a planned extension. If abandoned, remove the whole type and tests in a dedicated security-reviewed change. |
| V2 deployment validators | `internal/config/v2/deployment_validator.go:116-193` | `ValidateKamajiControlPlane`, `ValidateKamajiWorkerPool`, and `ValidateClusterAPIProviders` are unreachable. Configuration selection can be string- and schema-driven. | **Keep because public/API/dynamic/generated/build-tag/test-only usage is possible** | Confirm whether the schema or service registry is expected to invoke equivalent validation before considering removal. |
| V2 error constructors, wrappers, and classifiers | `internal/config/v2/errors.go:240-547` | Eleven exported helpers are unreachable: `NewConfigError`, the `Wrap*` family, `Is*` classifiers, and structured field/path/suggestion accessors. | **Probably safe but requires confirmation** | Review as one obsolete error-API surface. Remove only after checking documentation and all error-interface behavior. |
| `PathType.String` | `internal/core/paths/types.go:166` | No explicit call found, but it is a conventional `fmt.Stringer` method and can be invoked implicitly. | **Keep because public/API/dynamic/generated/build-tag/test-only usage is possible** | Keep unless the type itself is removed or tests establish that string formatting is not part of diagnostics. |
| Fifteen `internal/core/paths` examples | `internal/core/paths/example_test.go` | Reported unreachable because they lack executable `// Output:` checks. Names range from `Example_basicUsage` through `Example_batchResolution`. | **Keep because public/API/dynamic/generated/build-tag/test-only usage is possible** | Keep as documentation, or convert selected examples to verified examples by adding stable output assertions. |
| Global validation registry helpers | `internal/core/validation/registry.go:146,181` | `MustRegister` and `GlobalRegistry` are unreachable, but they are designed for registration/global access. | **Unsafe to remove** | First determine whether the global registry is an abandoned design. Prefer explicit dependency injection if it is retired. |
| Validator `Name`/`Priority` methods | `internal/core/validation/validators/` | Five methods are unreachable according to static analysis but form validator-shaped method sets. | **Keep because public/API/dynamic/generated/build-tag/test-only usage is possible** | Retain until validator interface satisfaction and registration are explicitly mapped. |
| AWS/OpenStack conversion and environment helpers | `internal/credentials/aws.go`; `openstack.go` | Seven exported helpers—`ToMap`, `ToTerraform`, `ToCloudsYAML`, and environment inventories—have no static callers. Environment and provider compatibility remain relevant. | **Probably safe but requires confirmation** | Confirm no command, template, or planned provider flow consumes their output formats; remove per-provider families rather than individual methods. |
| GitOps transaction methods | `internal/gitops/atomic.go:230,299` | `Transaction.RemoveFile` and `Rollback` are unreachable, but rollback is part of the transaction concept. | **Unsafe to remove** | Characterize failure behavior and interface expectations before changing transaction capabilities. |
| `IsGitOpsInitialized` | `internal/gitops/copy.go:41` | Exact search found no caller. | **Probably safe but requires confirmation** | Confirm no scripts/documentation refer to it as a planned API, then remove. |
| `DryRunWorkspaceManager` family | `internal/gitops/dryrun.go:406-467` | Constructor and four workspace methods are unreachable. Searches found no construction path. | **Probably safe but requires confirmation** | Confirm the current dry-run path uses the newer implementation, then remove the complete obsolete manager. |
| Unused `DryRunAtomicWriter` methods | `internal/gitops/dryrun_writer.go:87-108` | `RemoveAll`, `Commit`, `Rollback`, and `GetWorkspace` are unreachable. Some may exist for interface symmetry. | **Unsafe to remove** | Establish the actual writer interface and expected dry-run transaction behavior first. |
| `DryRunAtomicWriterAdapter` constructor and all methods | `internal/gitops/dryrun_writer.go:124-176` | The complete adapter family has no construction or call site. Exact type and constructor searches found declarations only. | **Probably safe but requires confirmation** | Verify no interface assertion or downstream tagged build requires it; then remove the complete adapter. |
| GitOps string/accessor/rollback methods | `internal/gitops/generator.go:156`; `pipeline.go:298,398-403`; `stages/validation_stage.go:249` | `GenerationResult.String`, pipeline rollback/accessors, and stage validation are unreachable. Several are interface- or diagnostics-shaped. | **Keep because public/API/dynamic/generated/build-tag/test-only usage is possible** | Keep until pipeline/stage interfaces and logging behavior are mapped. |
| Four GitOps examples | `internal/gitops/progress_example_test.go` | Documentation examples lack runnable output assertions. | **Keep because public/API/dynamic/generated/build-tag/test-only usage is possible** | Retain or turn into verified examples. |
| `localdev/gitea.Service.Layout` | `internal/localdev/gitea/service.go:142` | Exact search found no caller. | **Probably safe but requires confirmation** | Confirm layout access is not intended for local-dev extensions, then remove. |
| Logging reconfiguration and fatal helpers | `internal/logging/logging.go:130,251-254` | `LoggerManager.Reconfigure`, `Fatal`, and `Fatalf` have no caller. Fatal helpers also terminate execution and are undesirable library behavior. | **Probably safe but requires confirmation** | Confirm no plugin or local-development path depends on runtime reconfiguration; then remove. |
| `plugins.Discover` | `internal/plugins/loader.go:95` | Exact search found no caller; live code uses `DiscoverDetailed`. Plugin discovery itself is dynamic. | **Probably safe but requires confirmation** | Verify `DiscoverDetailed` fully supersedes it, then remove only the wrapper. |
| `ErrEncryptionFailed.Error` and `Unwrap` | `internal/secrets/errors.go:64-68` | Static reachability reports them unused, but the Go error machinery invokes these methods implicitly. | **Keep because public/API/dynamic/generated/build-tag/test-only usage is possible** | Keep while the error type exists. |
| `GitIntegrator` method family | `internal/sops/git.go` | Fourteen methods are unreachable in the production graph. Tests exercise portions of the type, and the family represents a latent SOPS/Git feature. | **Keep because public/API/dynamic/generated/build-tag/test-only usage is possible** | Make an explicit product decision: wire and support it, or remove the complete feature with its tests. Do not prune individual methods opportunistically. |
| `EnhancedKeyManager.SetAuditLogger`, `SetActor` | `internal/sops/key_manager.go:97-102` | No caller found, but these are security/audit injection points. | **Unsafe to remove** | Confirm the intended audit integration before removal. |
| Template composition convenience functions | `internal/template/composition.go:903-909`; `context.go:212` | `ComposeWithEngine`, `RenderComposedTemplate`, and `ContextBuilder.WithFunctions` have no caller. | **Probably safe but requires confirmation** | Verify templates do not select these through a function registry, then remove together. |
| Complete `DependencyResolver` family | `internal/template/dependencies.go` | Constructor and eight dependency/graph methods have no external references. Exact type and method searches found only declarations. | **Probably safe but requires confirmation** | Confirm template dependency metadata is not a planned feature; remove the entire file/family if retired. |
| Template metadata query/graph helpers | `internal/template/metadata.go` | Eight exported functions are unreachable, including filtering and circular-dependency checks. Templates and descriptors may encode related names dynamically. | **Unsafe to remove** | Decide whether metadata dependency validation is dormant or required before deleting it. |
| `NewTemplateSandboxWithTimeout` | `internal/template/sandbox.go:82` | No static caller. It is a security/resource-limit constructor variant. | **Unsafe to remove** | Preserve until sandbox timeout policy is documented and tested. |
| Internal testing helper family | `internal/testing/benchmarks.go`, `config_helpers.go`, `framework.go` | Nine test helpers are unreachable across current tests. | **Probably safe but requires confirmation** | Remove in a test-infrastructure-only batch after checking documentation and tagged tests. |
| Two `internal/testing` examples | `internal/testing/example_test.go` | Documentation examples without executable output checks. | **Keep because public/API/dynamic/generated/build-tag/test-only usage is possible** | Retain or convert to verified examples. |
| Crypto key validation and SSH generation helpers | `internal/util/crypto/` | Five exported helpers have no callers. They concern key validity and key generation. | **Unsafe to remove** | Require a security review and confirmation that no planned credential flow relies on them. |
| Error middleware constructor/recovery | `internal/util/errors/middleware.go` | `NewErrorMiddleware` and `RecoverPanic` have no callers. | **Probably safe but requires confirmation** | Confirm no command or service wrapper is expected to install the middleware; remove the complete unused middleware if retired. |
| Masking helpers | `internal/util/security/credential_masker.go:249-279` | `MaskPartial`, `MaskEmail`, and `MaskURL` have no callers. They are security-output helpers. | **Probably safe but requires confirmation** | Confirm no logging/output contract expects these exact masking forms, then remove together. |
| Secure temporary file and writer families | `internal/util/security/interfaces.go`; `secure_temp_file.go` | Seventeen related constructor, lifecycle, cleanup, and writer methods are unreachable. The entire family is isolated, but it embodies security-sensitive deletion and permission behavior. | **Unsafe to remove** | Either adopt it as the canonical secure-temp implementation with tests or remove it in a dedicated security-reviewed change. |
| `perf` tagged test file | `internal/config/memory_target_test.go` | Not part of the default graph and currently fails to compile because its target APIs were removed. | **Keep because public/API/dynamic/generated/build-tag/test-only usage is possible** | Product/maintainer decision required: update the benchmark target or deliberately retire the tag and file. |

No generated file, embedded descriptor, registration file, migration command, Cobra command, flag, configuration field, route, or provider API was classified as safe to remove.

## 9. Duplicate-logic candidate matrix

| Candidate and locations | Current behavior and semantic assessment | Tests and risk | Proposed shape and disposition |
|---|---|---|---|
| Canonical config paths versus manual `~/.config/opencenter` construction in `internal/secrets/multi_cluster.go`, `rotation.go`, and `manager.go` | Both intend to locate OpenCenter configuration/state. Secrets code bypasses environment overrides, XDG/platform behavior, and canonical configuration resolution. The semantic intent is the same; behavior is inconsistent and likely accidental. | Config path tests and secrets tests exist. Risk: medium-high because changing paths can alter existing installations. | Add characterization tests for explicit config-dir, environment, default, and legacy locations. Route secrets through `internal/config/persistence` or the appropriate existing resolver. **Prompt 3 candidate.** |
| `config.ResolveStateDir` versus `config/persistence.ResolveStateDir` | The live wrapper resolves from configured state behavior; the dead persistence helper starts from `DefaultStateDir`. Same name does not mean same contract. | Path and persistence tests exist. Risk: high if merged blindly. | Remove the dead helper in Prompt 2 after confirmation. Document the live resolver’s contract. **Do not deduplicate in Prompt 3.** |
| Age-key loading in `internal/sops/manager.go` and `internal/sops/git.go` | Both expand `~` and read an age key. One returns untrimmed content; the other trims whitespace. Both can index the first path byte without an obvious empty-path guard. Same domain and broad intent, but observably different whitespace behavior. | SOPS manager and GitIntegrator tests exist. Risk: medium, with security implications. | First decide whether `GitIntegrator` remains. If retained, create one SOPS-specific key-file loader with explicit empty-path and whitespace policy. **Conditional Prompt 3 candidate.** |
| Atomic writes in `internal/util/files`, `internal/util/fs`, `internal/gitops/atomic.go`, OpenStack provider/storage persistence, and `cmd/secrets_sops_helpers.go` | All aim to avoid partial replacement, but contracts differ: retry policy, temp naming, permission preservation, `fsync`, staged workspace semantics, rollback, recovery records, structured errors, and external SOPS execution. Much of the duplication is intentional. | Each major domain has tests, but cross-platform filesystem semantics are not expressed as one contract. Risk: high. | Do not merge wholesale. At most extract a narrowly specified `internal/atomicfile` replacement primitive after tests define mode preservation, durability, cleanup, and Windows behavior. Keep GitOps transactions and storage recovery separate. **Leave alone unless Prompt 3 explicitly characterizes the contract.** |
| Retry classification in `internal/util/errors/error_handler.go` and `internal/config/v2/errors.go` | The general handler uses string patterns such as timeout, connection refused, network, and device busy. V2 uses timeout classification and a `Temporary()` interface. They answer related but different questions. | Error tests exist. Risk: medium because retry changes can repeat destructive operations. | Keep separate. If later unified, define typed retry reasons rather than merging string predicates. **Not a Prompt 3 priority.** |
| Reflection traversal in `internal/config/flags/integration.go` and `internal/cluster/configure_service.go` | Both traverse configuration fields, but flags applies precedence, security, and CLI semantics while cluster configuration applies domain-specific mutation and validation. Mechanical similarity is real; semantic responsibility differs. | Extensive config/flags and cluster tests exist. Risk: high. | First retire unused flags subsystems and document the v2 canonical mutation path. Do not introduce a generic reflection helper now. **Leave alone in Prompt 3.** |
| Path parsing wrappers across `internal/config`, `internal/config/persistence`, and `internal/core/paths` | These are deliberate compatibility/layering wrappers: low-level parsing, persistence policy, and legacy-facing APIs. | Strong path/config test coverage. Risk: high if flattened. | Preserve until a compatibility deprecation plan exists. **Intentional duplication; leave alone.** |
| Command-level SOPS workflows in `cmd/secrets_sops_helpers.go` versus `internal/sops` and `internal/secrets` | The command file performs discovery, encryption/decryption/status workflows, file replacement, and console interaction. Some domain behavior overlaps existing internal packages. The intent is partly the same; the command layer also owns CLI policy. | Command, secrets, and SOPS tests exist, though behavior is spread across packages. Risk: medium-high. | Move reusable workflows behind narrow `internal/sops` or `internal/secrets` services while retaining Cobra argument parsing and output in `cmd`. **Prompt 3 candidate.** |
| Drift scheduling/orchestration in `cmd/cluster_drift.go` versus `internal/operations` | Command code contains substantial orchestration alongside existing drift operations. Detection logic and CLI presentation should remain distinct, but scheduling/persistence can be reusable. | Drift operation/property and command tests exist. Risk: medium. | Extract orchestration into an `internal/operations/drift` service; leave Cobra binding and rendering in `cmd`. **Prompt 3 candidate.** |

## 10. Reusable package extraction candidates

### 10.1 Secrets and SOPS command workflow

- **Evidence:** `cmd/secrets_sops_helpers.go` is approximately 835 lines and mixes Cobra-facing behavior with file discovery, SOPS execution, status inspection, and replacement workflows.
- **Target:** Extend `internal/sops` or introduce a narrow `internal/secrets/sopsworkflow` package.
- **Boundary:** Inputs should be paths, explicit options, filesystem/runner dependencies, and progress callbacks. Returned values should be domain results, not Cobra output.
- **Do not move:** Flag parsing, prompts, terminal formatting, exit decisions, or command registration.
- **Risk:** Medium-high.
- **Stage:** Prompt 3 after characterization tests.

### 10.2 Drift orchestration

- **Evidence:** `cmd/cluster_drift.go` is approximately 899 lines and includes scheduling and workflow behavior beyond command wiring.
- **Target:** `internal/operations/drift`.
- **Boundary:** Detection, scheduling, persistence, and lifecycle coordination behind an injected service.
- **Do not move:** Cobra definitions and user-facing rendering.
- **Risk:** Medium.
- **Stage:** Prompt 3.

### 10.3 Canonical OpenCenter layout policy

- **Evidence:** Secrets code manually constructs paths already governed by config and persistence packages.
- **Target:** Improve existing `internal/config/persistence` and `internal/core/paths`; do not add another path package.
- **Boundary:** `core/paths` should remain mechanical path discovery/validation. `config/persistence` should own OpenCenter configuration/state layout policy.
- **Risk:** Medium-high because of compatibility.
- **Stage:** Prompt 3 with legacy-path tests.

### 10.4 Atomic file replacement

- **Evidence:** Multiple domains implement temp-file-plus-rename behavior.
- **Target:** Only if characterization justifies it, a small `internal/atomicfile` package with an explicit durability and permission contract.
- **Boundary:** Atomic replacement of one file. It must not absorb GitOps transactions, retries, SOPS execution, or workspace rollback.
- **Risk:** High.
- **Stage:** Deferred or a separately approved portion of Prompt 3.

### 10.5 No `pkg/` proposal

No package has evidence of an intentionally supported external consumer. Creating `pkg/` would prematurely turn internal implementation into a compatibility promise. New or extracted code should remain under `internal/`.

Avoid names such as `utils`, `common`, or `helpers`. Prefer names that describe one responsibility, such as `drift`, `sopsworkflow`, or `atomicfile`.

## 11. LLM-friendly maintainability findings

### Existing strengths

- `docs/CODEMAPS/INDEX.md` and subsystem maps cover CLI commands, cluster lifecycle, configuration, DI, GitOps, providers, secrets, and services.
- Major domain packages are under `internal/`.
- The v2 configuration pipeline has identifiable phases.
- Property and race tests provide meaningful safety signals.

### Findings

1. **Package documentation is incomplete**

   Only 26 of 65 default-build packages expose package documentation through `go list`; 39 do not. Missing examples include root `main`, `cmd/opencenter-local`, `hack`, `config/v2`, configuration subpackages, importer, local-development subpackages, SOPS, security, and feature steps.

   Add concise package comments explaining responsibility, dependencies, and explicit non-responsibilities.

2. **Large files hide multiple responsibilities**

   Highest-priority examples include:

   - `internal/config/cli_settings.go` — 2,189 lines
   - `internal/secrets/manager.go` — 1,417
   - `internal/config/flags/integration.go` — 1,357
   - `internal/config/v2/defaults.go` — 1,034
   - `internal/cluster/init_service.go` — 1,005
   - `internal/secrets/rotation.go` — 971
   - `internal/security/audit_logger.go` — 938
   - `internal/template/composition.go` — 916
   - `cmd/cluster_drift.go` — 899
   - `internal/cluster/bootstrap_service.go` — 862
   - `internal/cloud/openstack/provider.go` — 856
   - `cmd/secrets_sops_helpers.go` — 835

   Splits should follow stable responsibilities, not arbitrary line counts.

3. **Two dependency-injection styles coexist**

   Typed `di.App` wiring and a legacy reflection container make runtime dependencies harder to trace. Document which one is canonical and prohibit new legacy-container registrations.

4. **Dynamic registration is difficult to discover**

   Service registrations and renderers use `init`. Add a code-map table listing registration keys, implementation packages, and selection paths. Longer term, prefer explicit composition-root registration.

5. **Configuration authority is not obvious enough**

   V2 appears authoritative, but flags, compatibility, defaults, persistence, and legacy wrappers remain distributed. Add one document defining the exact canonical read/mutate/write pipeline and the role of each package.

6. **Generation instructions have drifted**

   Stale schema tasks and a broad cleanup task undermine trust. Generation commands should be reproducible, non-destructive, and checked in CI.

7. **Documentation generation depends on local plugins**

   Loading external plugins while generating checked-in CLI docs makes output host-dependent. Provide a deterministic “built-ins only” generation mode.

8. **Toolchain versions are inconsistent**

   The Go directive, selected binary, cached artifacts, and lint tooling must agree. Otherwise maintainers and agents can misdiagnose version errors as source failures.

9. **No lint policy is committed**

   A repository-owned golangci-lint configuration should record enabled checks and an intentional baseline. Avoid introducing all 102 fixes into an unrelated refactor.

10. **The `perf` tag silently rotted**

    CI should compile every supported build tag even if it does not execute expensive benchmarks.

11. **No current coverage baseline exists**

    Absence of a coverage artifact does not prove missing tests, but it prevents quantitative comparison during refactoring. Add a reproducible package coverage command and retain summaries in CI.

12. **Existing code maps need freshness ownership**

    Each code map should state its authoritative entrypoints and include a lightweight verification checklist. Update maps in the same change that alters package boundaries or runtime wiring.

## 12. Risk register

| Risk | Severity | Evidence | Mitigation |
|---|---:|---|---|
| CLI/config compatibility regression | High | Large command and schema surface; legacy fields and migration command | Golden CLI tests, config fixtures, explicit deprecation policy |
| Dynamic plugin regression | High | Executable discovery and docs generation load plugins dynamically | Test plugin fixtures; deterministic built-in-only docs mode |
| Reflection false-positive dead code | High | Flags, config v2, DI, importer, descriptors | Interface/reflection mapping before deletion |
| Embedded resource deletion | High | Templates and YAML descriptors selected by string | Inventory embedded names and configuration references |
| Filesystem durability/permission regression | High | Multiple atomic-write contracts | Characterization tests before consolidation |
| Secret path behavior change | High | Manual default paths bypass canonical overrides | Tests for explicit, environment, default, and legacy paths |
| Security helper removal | High | SOPS, crypto, masking, and secure-temp families | Dedicated security review; no opportunistic pruning |
| Stale generation process | High | Missing schema command/source and broad cleanup task | Repair tasks, document inputs/outputs, CI drift check |
| Build-tag rot | Medium-high | `perf` test no longer compiles | Compile supported tags in CI |
| Toolchain mismatch | Medium-high | Go 1.26.4 versus 1.26.5 caused false failures | Pin and validate one exact toolchain |
| Lint cleanup changes behavior | Medium | 102 findings, including error paths and deprecated APIs | Separate themed changes with targeted tests |
| Example false positives | Medium | Examples without `Output:` reported unreachable | Retain or convert deliberately |
| Global state and `init` wiring | Medium | DI adapter, registries, logging/config globals | Explicit composition-root documentation and gradual removal |
| Dirty worktree interference | Medium | Pre-existing `.mise.toml` modification | Preserve it; inspect diffs before any future edit |

## 13. Recommended staged plan

### Stage 0: Establish a trustworthy baseline

1. Pin the intended Go patch version across `go.mod`, local tool configuration, and CI.
2. Commit a golangci-lint configuration without silently changing the repository’s lint policy.
3. Make all supported build tags compile, especially `perf`.
4. Add reproducible coverage and tagged-build commands.
5. Repair or explicitly disable stale generation tasks.

### Stage 1: Prompt 2 — conservative dead-code removal

Use small independent batches:

1. Remove the importer self-assignment.
2. Remove `NewLegacyConfigurationMerger`.
3. Confirm and remove isolated unreachable wrappers.
4. Confirm and remove complete unused families such as `DefaultsCache`, `DependencyResolver`, or `DryRunAtomicWriterAdapter`.
5. Rerun exact-name and related-string searches before each deletion.
6. Run focused tests after every batch and the full test/race/vet/tidy/deadcode baseline at the end.
7. Do not touch reflection, interfaces, examples, security families, tagged files, embedded data, commands, flags, schema fields, plugins, or registration code.

### Stage 2: Stabilize compatibility behavior

Before deduplication:

1. Add tests for configuration and state path precedence.
2. Add tests for secrets path resolution and legacy layouts.
3. Characterize age-key whitespace and empty-path behavior.
4. Characterize atomic replacement permissions, cleanup, and durability.
5. Record current CLI outputs for commands affected by extraction.
6. Document the authoritative configuration mutation pipeline.

### Stage 3: Prompt 3 — semantic deduplication and extraction

Preferred order:

1. Route secrets path lookup through canonical configuration/persistence APIs.
2. Extract SOPS workflows from `cmd`.
3. Extract drift orchestration from `cmd`.
4. Consolidate age-key loading only if `GitIntegrator` remains supported.
5. Consider an atomic-file primitive only if the new characterization tests prove the contracts can be shared.
6. Leave retry classification and reflection traversal separate unless a typed semantic contract is designed.

### Stage 4: Prompt 4 — maintainability and documentation

1. Add package comments to the 39 undocumented packages.
2. Split large files along stable domain responsibilities.
3. Update code maps and add a concise runtime/composition map.
4. Document dynamic registries, embedded assets, compatibility surfaces, and generation.
5. Mark typed DI as canonical and document legacy DI retirement.
6. Add deterministic generation and tag-compilation checks to CI.
7. Address lint findings in themed batches rather than a mass rewrite.

## 14. Exact handoff instructions for Prompt 2

```text
You are a senior Go refactoring agent working in the openCenter CLI repository.

Use docs/refactor-audit-report.md as the authoritative audit. Implement only the conservative dead-code-removal stage described in sections 8 and 13.

Before editing:
1. Inspect git status and preserve all pre-existing user changes.
2. Use the repository’s pinned/matching Go toolchain.
3. Rerun exact symbol searches and `deadcode -test ./...`.
4. Present the proposed deletion batches and their evidence.

Allowed scope:
- Candidates classified “Safe to remove”.
- Candidates classified “Probably safe but requires confirmation” only when a fresh repository-wide search confirms there are no Go, test, tagged, documentation, configuration, template, script, CI, reflection, registration, or plugin references.
- Remove complete obsolete families instead of leaving partial types.
- Remove the self-assignment at internal/importer/scanner.go:411.

Forbidden scope:
- Anything classified “Unsafe to remove”.
- Anything classified “Keep because public/API/dynamic/generated/build-tag/test-only usage is possible”.
- Cobra commands, flags, configuration fields, schema symbols, environment variables, plugin behavior, embedded assets, registration/init files, examples, build-tagged files, security APIs, migrations, or generated outputs.
- Semantic deduplication, package extraction, broad lint cleanup, reformatting unrelated files, or dependency updates.

Work in small batches. For each batch:
1. Record exact symbols and search evidence.
2. Run focused package tests.
3. Run `go test ./...`.
4. Stop and report if behavior or compilation changes unexpectedly.

At completion run:
- go test ./...
- go test -race ./...
- go vet ./...
- go list ./...
- go list -deps ./...
- go mod tidy -diff
- deadcode -test ./...
- golangci-lint run, reporting pre-existing versus new findings

Do not commit unless explicitly requested. Report every deleted symbol, why removal was safe, and the before/after deadcode counts.
```

## 15. Exact handoff instructions for Prompt 3

```text
You are a senior Go architect performing behavior-preserving semantic deduplication and internal package extraction in the openCenter CLI repository.

Prerequisites:
- Read docs/refactor-audit-report.md.
- Start only after Prompt 2’s dead-code cleanup is complete and tests pass.
- Preserve all pre-existing user changes.
- Do not infer equivalence from syntax alone.

First add characterization tests for:
1. Config/state path precedence: explicit option, CLI config-dir, OPENCENTER_CONFIG_DIR, platform/XDG default, and legacy layout.
2. Secrets manager, rotation, and multi-cluster path behavior.
3. SOPS age-key empty paths, home expansion, trailing newline, and whitespace.
4. Any command output or exit behavior affected by extraction.
5. Any atomic file behavior proposed for sharing: mode preservation, fsync/durability, cleanup, overwrite behavior, and failure rollback.

Implement in this order:
1. Replace hard-coded ~/.config/opencenter construction in secrets code with the correct existing config/persistence path API.
2. Extract reusable SOPS workflows from cmd/secrets_sops_helpers.go into a narrow internal/sops or internal/secrets/sopsworkflow service. Keep Cobra flags, prompts, rendering, and exit behavior in cmd.
3. Extract drift scheduling/orchestration from cmd/cluster_drift.go into internal/operations/drift. Keep Cobra wiring and presentation in cmd.
4. If GitIntegrator remains supported, consolidate age-key file loading behind one SOPS-specific helper with explicit whitespace and empty-path semantics.
5. Consider internal/atomicfile only if characterization proves a common contract. Do not merge GitOps transactions into it.

Do not:
- Create pkg/.
- Create utils, common, or helpers packages.
- Merge config.ResolveStateDir with config/persistence.ResolveStateDir.
- Merge retry classifiers merely because both inspect errors.
- Generalize reflection traversal.
- Change CLI names, flags, configuration schema, environment precedence, plugin behavior, rendered filenames, or embedded resource names.
- Mix broad lint or documentation cleanup into the refactor.

After each step run focused tests and `go test ./...`. At completion run the full test, race, vet, tidy-diff, deadcode, and lint baselines. Report changed dependency direction, preserved behavior, and any deliberately retained duplication.
```

## 16. Exact handoff instructions for Prompt 4

```text
You are a senior Go maintainer improving LLM-friendly and human-friendly maintainability in the openCenter CLI repository.

Read docs/refactor-audit-report.md and the completed Prompt 2 and Prompt 3 change summaries. This prompt is for documentation, structure, and repository hygiene; it must not intentionally change runtime behavior.

Required work:
1. Add concise package comments to the packages that still have empty `go list` documentation. Each comment must state responsibility and key boundary, not repeat the package name mechanically.
2. Update docs/CODEMAPS for every package boundary or runtime flow changed by Prompts 2 and 3.
3. Add or improve a top-level runtime/composition map covering:
   - main startup
   - typed DI construction
   - Cobra command registration
   - config v2 processing
   - plugin discovery
   - service/renderer init registration
   - embedded templates and descriptors
4. Document the authoritative config path and read/mutate/write pipeline.
5. Document generated inputs, outputs, and exact reproducible commands.
6. Make CLI documentation generation deterministic with respect to external plugins.
7. Align the Go patch version across local tooling and CI.
8. Ensure every supported build tag compiles, including an explicit decision for `perf`.
9. Add a repository-owned golangci-lint configuration and a reproducible coverage command.
10. Split oversized files only along clear responsibility boundaries and only when imports and tests demonstrate that the split is behavior-neutral.

Constraints:
- Do not perform mass lint autofixes.
- Do not rename public CLI commands, flags, config fields, environment variables, plugin identifiers, template names, or generated filenames.
- Do not introduce generic utils/common/helpers packages.
- Do not move internal code to pkg/.
- Do not delete examples or tagged tests without an explicit recorded decision.
- Preserve all pre-existing user changes.

Verification:
- go test ./...
- go test -race ./...
- go vet ./...
- compile every supported build tag
- go list ./...
- go list -deps ./...
- go mod tidy -diff
- golangci-lint run
- deterministic regeneration check for schema and CLI docs
- coverage command documented and executed

Report the documentation map, structural changes, toolchain decision, supported build tags, lint policy, and any remaining maintenance debt.
```
