---
id: mise-tasks
title: "Mise Tasks Reference"
sidebar_label: Mise Tasks
description: Flat reference of every task and tool pin defined in .mise.toml.
doc_type: reference
audience: "developers, contributors"
tags: [mise, build, tasks, reference]
---
# Mise Tasks Reference

**Purpose:** For developers, provides a complete, flat reference of every tool pin and task defined in `.mise.toml`. For the *why* behind this build system, see [Build System (Mise)](../contributing/build-system.md).

## Tool pins

| Tool | Version |
| --- | --- |
| `golang` | `1.26.6` |
| `golangci-lint` | `2.11.4` |
| `kubectl` | `latest` |
| `kind` | `latest` |
| `helm` | `latest` |
| `sops` | `3.13.3` |
| `go:golang.org/x/vuln/cmd/govulncheck` | `latest` |
| `aqua:gitleaks/gitleaks` | `latest` |

## Environment defaults

| Variable | Value | Purpose |
| --- | --- | --- |
| `KIND_EXPERIMENTAL_PROVIDER` | `podman` | Tells `kind` to use Podman instead of Docker. |
| `CONTAINER_RUNTIME` | `podman` | Container runtime used by local Kind/Gitea workflows. |

`.mise.toml` also documents (commented out) opt-in overrides for `OPENCENTER_CONFIG_DIR`, `OPENCENTER_DEBUG`, and `OPENCENTER_PLUGINS_DIR` for reproducible local test runs.

## Tasks

| Task | Command(s) |
| --- | --- |
| `build-cli` | `go build -ldflags "-X main.version=... -X main.gitCommit=... -X main.gitBranch=... -X main.gitTag=... -X main.buildDate=..." -o bin/opencenter` |
| `build-info` | `go version`; `go env GOOS GOARCH CGO_ENABLED GOROOT GOENV GOFLAGS` |
| `build-local-plugin` | `go build -o bin/opencenter-local ./cmd/opencenter-local` |
| `build` | `mise run build-cli` then `mise run build-local-plugin` |
| `build-linux` | Same as `build-cli`, cross-compiled `GOOS=linux GOARCH=amd64` -> `bin/opencenter-linux` |
| `build-all` | Cross-compiles `bin/opencenter-{linux,darwin}-{amd64,arm64}` |
| `local-install` | Builds both binaries; installs to `~/.local/bin/opencenter` and `<plugins-dir>/opencenter-local`; updates `checksums.txt` |
| `release <version> [target]` | Cross-compiles 4 release binaries into `bin/release/`, then runs `publish`, then prints manual tag/push/`gh release create` steps |
| `publish <version> [target]` | Generates `bin/release/RELEASE_NOTES_<version>.md` from `git log <last-tag>..HEAD` grouped by `feat`/`fix`/`docs` |
| `fmt` | `gofmt -w .` |
| `tidy` | `go mod tidy` |
| `upgrade-deps` | `go get -u ./...` then `go mod tidy` |
| `vet` | `go vet ./...` |
| `lint` | `golangci-lint run ./...` |
| `test` | `go test ./internal/config/... ./cmd/... -count=1`; `go test ./internal/cloud/... -count=1` |
| `test-race` | `go test ./internal/... ./cmd/... -count=1 -race` |
| `test-build` | `go build ./...` |
| `test-remediation` | `go test ./internal/sops ./internal/secrets ./internal/secretartifacts ./internal/gitops ./cmd -count=1` |
| `test-remediation-race` | `go test -race ./internal/secrets -run 'TestSyncSecretsSerializesConcurrentTransactions\|TestReconcileStateWriteFailureRollsBackAllMutationsAndRetry' -count=3` |
| `test-docs` | `go test -tags tools ./cmd/docs -count=1` |
| `test-kustomize` | `go test ./internal/gitops -run '^TestGeneratedDefaultOverlayKustomizeFailureMatrix$' -count=1` |
| `test-diff` | `git diff --check` |
| `godog` | `go test ./tests/features/steps/... -v -- --godog.tags=~@wip --godog.paths=tests/features` |
| `godog-wip` | Same, with `--godog.tags=@wip` |
| `godog-tag <tag>` | Same, with `--godog.tags=@<tag>` |
| `property` (alias `test-properties`) | `go test ./internal/... ./cmd/... -v -run "TestProperty" -count=1` |
| `govulncheck` | `govulncheck ./...` |
| `gitleaks` | `gitleaks detect --source . -c .gitleaks.toml --redact --no-banner` |
| `integration` | Runs `TestClusterSetup` (`./cmd`), then `TestRetry\|TestCircuitBreaker\|TestLockManager\|TestProperty` (`./internal/resilience/...`), then `TestDriftDetector\|TestBackupManager\|TestProperty` (`./internal/operations/...`) |
| `perf` | `go test -tags perf ./internal/config -run "TestMemoryUsageRegression" -count=1` |
| `test:all` | `test`, `test-race`, `vet`, `godog`, `property`, `govulncheck` |
| `verify` | `test`, `test-race`, `test-properties`, `govulncheck` |
| `test-remediation-all` | `test-build`, `vet`, `test`, `test-remediation`, `test-race`, `test-remediation-race`, `godog`, `test-docs`, `test-kustomize`, `test-docs-idempotency`, `test-docs-frontmatter-remediation`, `test-diff` |
| `schema` | `go run ./cmd/schema-gen/main.go --version 2.0 --output schema/cluster.schema.json` |
| `schema-gen` | `go run ./cmd/schema-gen/main.go --version 2.0 --output schema/cluster.schema.json` |
| `schema-v2` | Writes and runs a throwaway `TestRegenSchema` against `internal/config/v2schema`, writes `schema/opencenter-v2.schema.json`, deletes the test file |
| `validate` | `./bin/opencenter cluster validate` |
| `schema-verify` | Build, generate schema, `cluster init`/`update`/`validate` against `OPENCENTER_CONFIG_DIR=./testdata/config`, `mise run test`, `mise run godog` |
| `docs-gen` | `go run cmd/docs/generate.go` |
| `test-docs-idempotency` | Runs `docs-gen` twice, diffs the two results, fails if unstable |
| `test-docs-frontmatter` | `python3 hack/scripts/audit_doc_frontmatter.py --strict` |
| `test-docs-frontmatter-remediation` | Same, with an `--ignore` list of documented legacy pages |
| `tag-wip-failures` | `python3 hack/tag_wip_failures.py` |
| `gitea-up` | `go run ./cmd/opencenter-local gitea up` |
| `gitea-cleanup` | `go run ./cmd/opencenter-local gitea destroy` |
| `active` | `./bin/opencenter cluster status` |
| `terraform-generate <cluster> [dir]` | `mise run build`; `./bin/opencenter cluster terraform-generate <cluster> --output-dir=<dir>` |
| `preflight` | `./bin/opencenter cluster validate` |
| `install-shell-integration` | `./hack/install-shell-integration.sh` |
| `install-hooks` | Verifies and `chmod +x`s `.git/hooks/pre-commit` |
| `clean` | Removes `bin/`, `testdata/`, `new-schema.json`, `terraform-output/` |
| `kind-cleanup [cluster]` | Destroys the named Kind cluster (default `my-cluster`) and local Gitea; removes a stale lock file |
| `demo-cleanup` | `kind-cleanup` then `clean` |
| `openstack-reset` | `hack/scripts/openstack-reset.sh` (accepts `--os-cloud`, `--force`, `--dry-run` after `--`) |

## Discovering tasks interactively

```bash
mise tasks                # list all tasks
mise task show <name>      # show a task's full definition
mise run -v <name>          # verbose run
mise run --dry-run <name>    # show what would run
```

See [Testing Guide](../contributing/testing-guide.md) for which of these tasks CI actually runs, and [GitHub Actions Workflows](github-actions-workflows.md) for the workflows themselves.
