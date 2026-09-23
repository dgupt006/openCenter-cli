---
id: github-actions-workflows
title: "GitHub Actions Workflows"
sidebar_label: GitHub Actions Workflows
description: Complete reference of every workflow under .github/workflows -- triggers, jobs, steps, permissions, and runners.
doc_type: reference
audience: "developers, maintainers, devops engineers"
tags: [ci, github-actions, workflows, reference]
---
# GitHub Actions Workflows

**Purpose:** For developers and maintainers, documents every CI/CD workflow in `.github/workflows/`.

Common pattern across all workflows: `env: FORCE_JAVASCRIPT_ACTIONS_TO_NODE24: true`; `actions/checkout@v7`; `actions/setup-go@v6` (`go-version-file: go.mod`, `cache: true`); `runs-on: self-hosted` (except `deploy-kind.yml`, which uses `self-hosted-kvm`). No GitHub-hosted runners are used anywhere in this repository.

## `build-binaries.yml` -- "Publish Binaries"

* Trigger: `workflow_dispatch` only.
* Permissions: `contents: write`.
* Job `build`, self-hosted, matrix `{linux/amd64, linux/arm64}`, `fail-fast: false`.
* Steps: checkout -> setup-go -> `go build -trimpath -ldflags "-X main.version=<tag without v> -X main.gitCommit=<HEAD> -X main.gitBranch=<branch> -X main.buildDate=<UTC now>" -o dist/opencenter-${GOOS}-${GOARCH}` -> `actions/upload-artifact@v7`.
* Does not use `mise run` -- calls `go build` directly.

## `deploy-kind.yml` -- "Deploy Disposable Kind Cluster"

The most complex workflow (521 lines) -- a fully automated, disposable Kind + Gitea + FluxCD deployment exercised entirely through the `opencenter` and `opencenter-local` CLIs.

* Trigger: `workflow_dispatch` with inputs `cluster_name` (optional, DNS-1123, derived from the run ID/attempt if blank), `container_runtime` (`docker`|`podman`, default `docker`), `managed_cni` (bool, default `true`), `cleanup` (bool, default `true`), `debug` (bool, default `false`).
* Permissions: `contents: read`.
* Pinned tool versions (`env`): `SOPS_VERSION=3.13.3`, `KIND_VERSION=v0.29.0`, `KUBECTL_VERSION=v1.35.4`, `FLUX_VERSION=v2.6.4`, `HELM_VERSION=v3.19.0`.
* Concurrency group `${{ github.repository }}-deploy-kind`, `cancel-in-progress: false`.
* Job `deploy-kind`, `runs-on: [self-hosted-kvm]`, `timeout-minutes: 90`.

Step sequence:

1. Checkout.
2. **Initialize runner-local environment**: creates isolated directories under `${RUNNER_TEMP}` and exports the full set of `OPENCENTER_*_DIR` variables to `$GITHUB_ENV`: `OPENCENTER_CONFIG_DIR`, `OPENCENTER_CLUSTERS_DIR`, `OPENCENTER_STATE_DIR`, `OPENCENTER_PLUGINS_DIR`, `OPENCENTER_BLUEPRINTS_DIR`, `OPENCENTER_GITOPS_DIR`, `OPENCENTER_CLUSTER_STATE_DIR`, `OPENCENTER_SECRETS_DIR`; also `HOME`, `XDG_CONFIG_HOME`/`XDG_CACHE_HOME`/`XDG_STATE_HOME`, `GOCACHE`/`GOMODCACHE`, `KUBECONFIG`.
3. setup-go.
4. **Resolve and validate workflow inputs**: validates enums, derives a safe `cluster_name`, sets `CONTAINER_RUNTIME`, `MANAGED_CNI`, `DEBUG_INPUT`, `KIND_EXPERIMENTAL_PROVIDER`, `KUBECONFIG` (now `${OPENCENTER_CLUSTER_STATE_DIR}/local/${cluster_name}/kubeconfig.yaml`), `DEPLOY_LOG`.
5. **Install pinned deployment tools**: `go install sops@v${SOPS_VERSION}`; checksum-verified downloads of kubectl/kind/helm/flux at the pinned versions into `${RUNNER_TEMP}/bin` (added to `PATH`); `${OPENCENTER_PLUGINS_DIR}` also added to `PATH`.
6. **Build CLI and local plugin**: `go build -trimpath -o bin/opencenter .`; `go build -trimpath -o bin/opencenter-local ./cmd/opencenter-local`; installs `opencenter-local` into `${OPENCENTER_PLUGINS_DIR}`; sanity-checks `opencenter --help`, `opencenter local --help`, `opencenter-local --help`, `opencenter version`.
7. **Preflight**: verifies docker/podman, kind, kubectl, flux, helm, sops, git are present; refuses to reuse an existing Kind cluster of the same name.
8. **Start local Gitea and initialize the cluster config**: `opencenter-local gitea --runtime <rt> up` / `status`; parses the bootstrap repo URL and confirms a user token is present; `opencenter cluster init <name> --org local --type kind [--kind-disable-default-cni]`; `opencenter cluster set <name>` with dotted keys for `opencenter.infrastructure.kind.disable_default_cni`, `opencenter.gitops.repository.url`, `opencenter.gitops.auth.token.provider=gitea`, `opencenter.gitops.auth.token.token_file`.
9. **Validate and generate**: `opencenter cluster validate <name>`; `opencenter cluster generate <name> --force --gitops-auth=token`.
10. **Create Kind through the CLI**: `opencenter cluster deploy <name> --container-runtime <rt> --kubeconfig <path> --log <path> --step kind-create [--debug]`.
11. **Export Kind kubeconfig**: same command with `--step kind-export-kubeconfig`.
12. **Refresh Gitea bootstrap URL and re-attach**: `opencenter-local gitea attach-kind --cluster <name>`; re-fetch status; `opencenter cluster set <name> opencenter.gitops.repository.url=<url>`; re-validate/re-generate; `opencenter cluster describe <name> --output json` parsed for `.git_dir`; the workflow updates that local repo's `origin` remote.
13. **Deploy remaining cluster stages**: `opencenter cluster deploy <name> ... --from-step gitea-attach-kind [--debug]`.
14. **Verify live cluster and Flux status**: `opencenter cluster status <name> --refresh`; `kubectl wait --for=condition=Ready nodes --all`; `flux check`; `flux get sources git`/`kustomizations` in `flux-system`.
15. **Print non-secret diagnostics on failure** (`if: failure()`): `opencenter cluster status <name> --refresh --output json`, kind/docker/podman `ps`, `kubectl get nodes/pods/events`, `flux check`/`get`.
16. **Cleanup** (`if: always() && inputs.cleanup`): `opencenter cluster destroy <name> --force --remove-files`; `kind delete cluster --name <name>`; `opencenter-local gitea destroy`.

This workflow is the authoritative confirmation of several real CLI subcommand shapes:

- `cluster init`
- `cluster set <dotted.key>=<value>...`
- `cluster validate`
- `cluster generate --gitops-auth=token` (with `--force`)
- `cluster deploy --container-runtime --kubeconfig --log --step|--from-step [--debug]`
- `cluster status --refresh [--output json]`
- `cluster describe --output json`
- `cluster destroy --remove-files` (with `--force`)
- plugin subcommands `opencenter-local gitea {up,status,attach-kind,destroy}`

Step names confirmed real: `kind-create`, `kind-export-kubeconfig`, `gitea-attach-kind` (and steps continue after it via `--from-step`).

## `docs-p0.yml` -- "Docs P0 Checks"

* Trigger: `pull_request`, `paths: ["**/*.md"]`.
* Job `docs-p0`, self-hosted.
* Steps: checkout (`fetch-depth: 0`) -> collect changed `.md` files against `origin/<base_ref>` -> "Run P0 doc checks" (`./scripts/docs/p0-docs-check.sh <files>`) -> "Vale" (`vale-cli/vale-action@v2`, `fail_on_error: true`).
* **Known discrepancy**: `scripts/docs/p0-docs-check.sh` does not exist in this repository (only `scripts/shell-integration-test.sh` exists under `scripts/`). A `.vale.ini` does exist at the repository root. This workflow currently references a checked-in-missing script -- do not assume this check is meaningfully enforcing anything until that script exists.

## `release.yml` -- "Release"

* Trigger: `push` on tags `v*`, plus `workflow_dispatch`.
* Permissions: `contents: write`, `id-token: write`.
* Job `build-cli`: self-hosted, matrix `{linux/amd64, linux/arm64, darwin/amd64, darwin/arm64}`; builds `dist/opencenter-${VERSION}-${GOOS}-${GOARCH}` (version from `github.ref_name`, `v`-stripped) with full ldflags; uploads per-arch artifacts.
* Job `build-plugin`: same matrix, builds `./cmd/opencenter-local` -> `dist/opencenter-local-${VERSION}-${GOOS}-${GOARCH}` (no ldflags); uploads.
* Job `release` (needs both, `id-token: write` + `contents: write`): downloads all artifacts into `dist/` (merge-multiple) -> `sha256sum opencenter-* | sort > checksums.txt` -> installs cosign (`sigstore/cosign-installer@v4.1.2`) -> `cosign sign-blob --yes --bundle <artifact>.bundle <artifact>` for every dist file (keyless, `COSIGN_YES=true`) -> installs syft -> `syft dir:dist -o spdx-json=dist/opencenter.spdx.json` -> `gh release create "${GITHUB_REF_NAME}" dist/* --generate-notes` (using `secrets.GITHUB_TOKEN`).
* Does **not** call the `.mise.toml` `release`/`publish` tasks -- CI has its own independent build + sign + SBOM + release pipeline. The `release`/`publish` mise tasks (local binary build plus git-log-based `RELEASE_NOTES_*.md` generation) are a separate, non-CI, local-only helper. See [Release Process](../contributing/release-process.md).

## `test.yml` -- "Go Tests"

* Trigger: `pull_request`, `push` to `main`.
* Job `go-test` (self-hosted, 30 min): checkout -> setup-go -> install `sops` v3.13.3 (added to `PATH`) -> `go test ./internal/... ./cmd/... -count=1 -race` -> `go vet ./...`.
* Job `property-tests` (self-hosted, 30 min): checkout -> setup-go -> `go test ./internal/... ./cmd/... -count=1 -run 'TestProperty'`.
* Note: CI's test scope (`./internal/... ./cmd/...`) is broader than the `mise run test` task's scope (`./internal/config/... ./cmd/... ./internal/cloud/...`) -- CI does not literally invoke `mise run test`, it runs its own, wider command. `sops` is installed here because some `internal/...` tests exercise the real `sops` binary.

## `vulncheck.yml` -- "Dependency Vulnerability Scan"

* Trigger: `pull_request`, `schedule` (`0 6 * * 1` -- Mondays 06:00 UTC), `workflow_dispatch`.
* Job `govulncheck` (self-hosted, 30 min): checkout -> setup-go -> `go install golang.org/x/vuln/cmd/govulncheck@latest` -> `govulncheck ./...`.

## `pre-commit.yaml` -- "Run pull-request syntax workflows"

* Trigger: bare `pull_request`.
* Job `pre_commit` (self-hosted), matrix `python-version: ["3.10"]`.
* Steps: checkout -> `actions/setup-python@v6` -> `git fetch --prune --unshallow` -> compute `CHANGED_FILES` via `git diff --name-only HEAD^` -> `pre-commit/action@v3.0.1` with `--files ${CHANGED_FILES} --hook-stage manual`.
* The repository's `.pre-commit-config.yaml` defines exactly one hook: `gitleaks/gitleaks` rev `v8.24.2`, id `gitleaks`. So this workflow runs gitleaks secret-scanning on changed files per pull request -- distinct in scope from the `mise run gitleaks` task, which runs a full-history scan (`gitleaks detect --source . -c .gitleaks.toml --redact --no-banner`) and is not wired into any CI workflow (local/manual only).

## What CI does not run

No workflow runs the BDD suite (`mise run godog`), the `mise run integration` task, the documentation-generator tests (`mise run test-docs*`), the Kustomize build check, or `mise run test:all`. Run those locally before merging a change that touches the relevant area -- see [Testing Guide](../contributing/testing-guide.md).

## Cross-reference

* Local build/test task definitions: [Mise Tasks Reference](mise-tasks.md), [Build System (Mise)](../contributing/build-system.md).
* What actually publishes a release: [Release Process](../contributing/release-process.md).
