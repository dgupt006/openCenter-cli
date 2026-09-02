---
id: github-actions-workflows
title: "GitHub Actions Workflows"
sidebar_label: GitHub Actions Workflows
description: Reference the repository's GitHub Actions triggers, jobs, runner contracts, permissions, artifacts, and release behavior.
doc_type: reference
audience: "maintainers, release engineers, platform engineers"
tags: [github-actions, ci-cd, runners, releases, podman, kind]
---
# GitHub Actions Workflows

**Purpose:** For maintainers, release engineers, and platform engineers, documents the current repository automation defined under `.github/workflows/`.

## Scope and authority

The workflow files under `.github/workflows/` are authoritative for this repository. This page describes their current names, triggers, jobs, permissions, commands, and outputs. The YAML examples in [Integrate CI/CD](../operations/integrate-ci-cd.md) are illustrative end-user patterns; they are not copies of this repository's automation.

The workflow set currently contains seven files:

| File | Workflow name | Trigger, path filter, or schedule | Jobs, dependencies, and matrix | Runner | Timeout / concurrency | Explicit permissions | Artifacts, releases, and secrets |
| --- | --- | --- | --- | --- | --- | --- | --- |
| [`build-binaries.yml`](../../.github/workflows/build-binaries.yml) | Publish Binaries | Manual `workflow_dispatch` | `build`; 2-entry matrix: Linux amd64 and Linux arm64 | `self-hosted` | Not set | `contents: write` | Uploads one artifact per matrix entry; no release; no secrets |
| [`deploy-kind.yml`](../../.github/workflows/deploy-kind.yml) | Deploy Disposable Kind Cluster | Manual `workflow_dispatch`; inputs for cluster name, Docker/Podman, managed CNI, cleanup, and debug | `deploy-kind`; no matrix or dependency | `[self-hosted, self-hosted-kvm]` | 90 minutes; repository-scoped concurrency, no cancellation | `contents: read` | No uploaded artifacts or release; no secrets |
| [`docs-p0.yml`](../../.github/workflows/docs-p0.yml) | Docs P0 Checks | `pull_request` with `**/*.md` path filter | `docs-p0`; no matrix or dependency | `self-hosted` | Not set | Not declared; inherited repository defaults apply | No uploaded artifacts or release; no secrets |
| [`pre-commit.yaml`](../../.github/workflows/pre-commit.yaml) | Run pull-request syntax workflows | Any `pull_request` | `pre_commit`; one-entry Python 3.10 matrix | `self-hosted` | Not set | Not declared; inherited repository defaults apply | No uploaded artifacts or release; no secrets |
| [`release.yml`](../../.github/workflows/release.yml) | Release | Push of tags matching `v*`, or manual `workflow_dispatch` | `build-cli` and `build-plugin` run independently; `release` needs both | `self-hosted` | Not set | `contents: write`, `id-token: write`; release job declares the same two permissions | Uploads build artifacts, then creates a GitHub release; keyless cosign `.bundle` files and SPDX JSON are included; uses `secrets.GITHUB_TOKEN` |
| [`test.yml`](../../.github/workflows/test.yml) | Go Tests | Any `pull_request`; pushes to `main` | `go-test` and `property-tests` run independently | `self-hosted` | 30 minutes per job; no concurrency | Not declared; inherited repository defaults apply | No uploaded artifacts or release; no secrets |
| [`vulncheck.yml`](../../.github/workflows/vulncheck.yml) | Dependency Vulnerability Scan | Any `pull_request`; Monday 06:00 UTC schedule; manual `workflow_dispatch` | `govulncheck`; no matrix or dependency | `self-hosted` | 30 minutes; no concurrency | Not declared; inherited repository defaults apply | No uploaded artifacts or release; no secrets |

A workflow with no `permissions` block uses the repository or organization default token permissions. A job-level declaration narrows or sets permissions for that job; it does not create a secret. Most jobs use the generic `self-hosted` runner label; `deploy-kind.yml` additionally requires the custom `self-hosted-kvm` label. The operating profiles below describe the capabilities each selected runner must provide.

## Common workflow environment

Every workflow sets:

```yaml
env:
  FORCE_JAVASCRIPT_ACTIONS_TO_NODE24: true
```

This opts JavaScript-based actions into the Node 24 runtime compatibility path. It is a workflow environment setting; it does not change the Go, Python, or other tool versions installed by a job.

## Workflow reference

### Publish Binaries

`.github/workflows/build-binaries.yml` runs only when dispatched manually. Its `build` job uses a two-entry `goos`/`goarch` matrix for `linux/amd64` and `linux/arm64`. It checks out with `actions/checkout@v7`, installs the Go version declared by `go.mod` with `actions/setup-go@v6`, builds with `go build -trimpath` and version metadata, and uploads `opencenter-linux-amd64` or `opencenter-linux-arm64` through `actions/upload-artifact@v7`. It has `contents: write`, but it does not create a release and does not read a named secret.

### Deploy Disposable Kind Cluster

`.github/workflows/deploy-kind.yml` is the intended current disposable integration workflow. It is manually dispatched with these inputs:

| Input | Type | Default / behavior |
| --- | --- | --- |
| `cluster_name` | string | Optional; blank derives `kind-<run-id>-<attempt>` and nonblank values must be DNS-1123 and at most 63 characters |
| `container_runtime` | choice | Required; `docker` or `podman`, default `docker` |
| `managed_cni` | boolean | Required, default `true`; disables Kind's default CNI when true |
| `cleanup` | boolean | Required, default `true`; destroys the disposable cluster and Gitea after the run |
| `debug` | boolean | Required, default `false`; enables verbose bootstrap diagnostics |

The single `deploy-kind` job has a 90-minute timeout and uses repository-scoped concurrency `${{ github.repository }}-deploy-kind` with `cancel-in-progress: false`, because the runner-local container daemon and fixed Gitea container are stateful. It checks out the repository, installs the Go version from `go.mod`, installs checksummed pinned SOPS, kubectl, Kind, Helm, and Flux tools, builds the CLI and `opencenter-local`, starts and provisions local Gitea, initializes and validates the cluster, generates GitOps content, creates Kind, exports the kubeconfig through the CLI, attaches Gitea to Kind, refreshes the GitOps URL and Git remote, deploys remaining stages, and verifies nodes and Flux sources/kustomizations.

The workflow sets `contents: read` and uses no GitHub secrets. It deliberately does not upload bootstrap logs, kubeconfig, generated state, or diagnostics; failure diagnostics are printed and the bootstrap log is retained only on the runner. Cleanup is guarded by `always()` and the `cleanup` input.

### Docs P0 Checks

`.github/workflows/docs-p0.yml` runs on pull requests that change at least one Markdown file. The `docs-p0` job checks out with full history, fetches the base branch, computes changed Markdown paths, invokes `./scripts/docs/p0-docs-check.sh` when there are changed files, and then runs `vale-cli/vale-action@v2` for those files. The workflow does not declare permissions, timeout, concurrency, artifacts, release publication, or secrets.

The referenced P0 script is not present in the current repository. Therefore this workflow's P0 check cannot currently be claimed as passing; see [Known limitation](#known-limitation).

### Pull-request syntax checks

`.github/workflows/pre-commit.yaml` runs on every pull request. Its `pre_commit` job has a one-entry Python `3.10` matrix, checks out with `actions/checkout@v7`, installs Python with `actions/setup-python@v6`, fetches all branches and tags, computes `HEAD^` changed files, and runs `pre-commit/action@v3.0.1` at the manual hook stage for those files. Permissions, timeout, concurrency, artifacts, releases, and secrets are not declared.

### Release

`.github/workflows/release.yml` runs for a pushed tag matching `v*` or by manual dispatch. It declares `contents: write` and `id-token: write` at workflow scope. The `release` job repeats those permissions explicitly and needs both build jobs.

`build-cli` has a four-entry matrix for Linux amd64, Linux arm64, macOS amd64, and macOS arm64. `build-plugin` has the same matrix for `opencenter-local`. Each job checks out, installs the Go version from `go.mod`, builds with version metadata, and uploads its binaries with `actions/upload-artifact@v7`. The final `release` job checks out, installs Go, downloads and merges the build artifacts with `actions/download-artifact@v8`, creates sorted SHA-256 checksums with GNU `sha256sum`, installs `sigstore/cosign-installer@v4.1.2`, signs every binary and `checksums.txt` with keyless `cosign sign-blob --bundle`, installs Syft with `go install github.com/anchore/syft/cmd/syft@latest`, writes `dist/opencenter.spdx.json`, and runs `gh release create "${GITHUB_REF_NAME}" dist/* --generate-notes`.

The only named secret is `secrets.GITHUB_TOKEN`, passed to `gh` for release creation. Keyless signing uses GitHub's OIDC token permission (`id-token: write`), not a stored signing key. The release outputs are the four CLI binaries, four plugin binaries, `checksums.txt`, a `.bundle` next to each signed binary and checksum file, and `opencenter.spdx.json`.

### Go Tests

`.github/workflows/test.yml` runs on every pull request and on pushes to `main`. `go-test` and `property-tests` are independent self-hosted jobs, each with a 30-minute timeout. `go-test` checks out, sets up the Go version from `go.mod`, installs SOPS `v3.13.3`, runs `go test ./internal/... ./cmd/... -count=1 -race`, and runs `go vet ./...`. `property-tests` checks out, sets up Go, and runs `go test ./internal/... ./cmd/... -count=1 -run 'TestProperty'`. The workflow declares no permissions, concurrency, artifacts, release publication, or secrets.

This workflow does not run the BDD suite, full integration task, documentation generator checks, or the full `mise run test:all` task. Run those locally when the change requires them; do not describe them as coverage supplied by this workflow.

### Dependency Vulnerability Scan

`.github/workflows/vulncheck.yml` runs on pull requests, every Monday at 06:00 UTC, and by manual dispatch. Its `govulncheck` job has a 30-minute timeout, checks out, sets up the Go version from `go.mod`, installs `golang.org/x/vuln/cmd/govulncheck@latest`, and runs `govulncheck ./...`. Permissions, concurrency, artifacts, releases, and secrets are not declared.

## Runner profiles

Most workflow jobs request `self-hosted` without additional labels. The disposable Kind job requires both `self-hosted` and `self-hosted-kvm`. Register runners and route jobs using profiles that match the workload:

| Profile | Workloads | Operational requirements |
| --- | --- | --- |
| Baseline Linux CI | `test.yml` and ordinary Go checks | Linux self-hosted runner with the repository's Go toolchain and enough CPU/memory for race tests |
| Build/release | `build-binaries.yml` and `release.yml` | Linux self-hosted runner with the Go toolchain, GNU `sha256sum`, Bash with `shopt`, GitHub CLI, and outbound access to Go and Sigstore services |
| Docs/pre-commit | `docs-p0.yml` and `pre-commit.yaml` | Linux self-hosted runner with Bash, Git, Python 3.10, pre-commit, Vale when the P0 script is restored, and the repository's docs tooling |
| Vulnerability scan | `vulncheck.yml` | Linux self-hosted runner with the Go toolchain, outbound Go module access, and the ability to install `govulncheck` |
| Disposable Kind/Gitea | `deploy-kind.yml` | Dedicated Linux amd64 runner meeting the [Kind/Podman contract](#podmankind-runner-contract) |

Keep OpenStack E2E separate from the disposable Kind/Gitea profile. Its network, credentials, cleanup, and runner requirements are documented in [OpenStack End-to-End Testing in CI](../operations/integrate-ci-cd.md#openstack-end-to-end-testing-in-ci). No OpenStack E2E workflow is part of the seven repository workflows described here.

## Podman/Kind runner contract

The disposable workflow is intended for a dedicated Linux amd64 self-hosted runner. The following distinction matters:

* **Workflow-enforced:** the job requests `[self-hosted, self-hosted-kvm]`; the workflow validates the selected runtime as Docker or Podman, installs Linux amd64 tool binaries, checks the runtime, refuses a Kind name collision, serializes runs, times out after 90 minutes, and cleans up when `cleanup` is true.
* **Operational recommendation:** assign `self-hosted-kvm` only to the dedicated runner that satisfies this Linux amd64 and Podman contract. The custom label selects that runner; it does not by itself prove that hardware KVM acceleration is available or required by this workflow.

### Capacity and host configuration

| Requirement | Minimum | Recommended |
| --- | ---: | ---: |
| CPU | 4 vCPU | 8 vCPU |
| Memory | 8 GB | 16 GB |
| Disk | 20 GB free | 50 GB free |
| Architecture | Linux amd64 | Linux amd64 |

Use cgroup v2. Podman 5.x is recommended, and rootful Podman is the simplest supported operational profile. Rootless Podman additionally needs configured subuid/subgid ranges, `uidmap`, `fuse-overlayfs`, `slirp4netns` or `pasta`, `netavark`, `aardvark-dns`, lingering for the runner user, and a systemd service with `Delegate=yes` where systemd manages the runner. The runtime must support the network and bridge behavior required for Kind and Gitea; privileged operations may be required by the selected Kind/Podman configuration and must be accepted as part of the dedicated runner trust boundary.

### Network and host contract

Gitea uses host ports 3000 (HTTP), 3001 (HTTPS), and 2222 (SSH). The container-to-host GitOps path must use the runner host's routable IPv4 address on port 3001, not an address that is reachable only from the runner itself. The runner needs outbound access to the GitHub action and release endpoints, Go module sources, Kubernetes/Kind/Helm/Flux/SOPS download endpoints, and the container registries required to pull the Kind and Gitea images. DNS and the container network must permit the Kind cluster to reach the Gitea host address.

The workflow itself checks for `curl`, `sha256sum`, `tar`, `awk`, `sed`, `grep`, and `install`, then uses `git`, the selected container runtime, and the installed `sops`, `kind`, `kubectl`, `helm`, and `flux` binaries. `jq` is optional because the URL-resolution step falls back to `python3` and then a known GitOps directory.

### Security, cleanup, and capacity

Treat the runner as a trusted boundary for repository code and disposable cluster workloads. Do not place production credentials, persistent cluster state, or unrelated workloads on it. Keep the runner dedicated or enforce equivalent isolation, limit who can dispatch the workflow, and ensure the runner account can remove containers, networks, volumes, temporary directories, and stale Kind clusters. Because the workflow serializes this repository's runs but does not provide cross-repository capacity management, size the host for one active Kind/Gitea run and monitor disk pressure. If a run is cancelled or the host fails before cleanup, inspect and reclaim the disposable resources before the next run.

### Acceptance commands

Run these checks on the runner before enabling the workflow. Commands marked operational verify recommendations; the workflow's own checks remain authoritative for each run.

```bash
uname -srm
uname -m                         # expected: x86_64
nproc                            # minimum: 4
free -h                          # minimum: 8 GB host memory

df -h /                          # minimum: 20 GB free
stat -fc %T /sys/fs/cgroup       # expected: cgroup2fs
podman --version                 # Podman 5.x recommended
podman info --format '{{.Host.CgroupVersion}} {{.Host.Security.Rootless}}'

for command in bash curl sha256sum tar awk sed grep install git go; do
  command -v "$command"
done

for port in 3000 3001 2222; do
  ss -ltn "sport = :$port" || true
done

podman run --rm docker.io/library/alpine:3.20 true
curl --fail --silent --show-error https://github.com/ >/dev/null
```

After a successful dispatch, verify that `kind get clusters` and the selected container runtime show no disposable resources when cleanup is enabled. Confirm that the runner can reach the host-routable `https://<runner-ip>:3001/` path from the container network before treating a bootstrap failure as an application failure.

## Tool authority and version policy

Use these sources in order when deciding what a CI tool should run:

1. The workflow file for a workflow-specific pin or install command.
2. `go.mod` for the Go language version (`1.26.6`).
3. `.mise.toml` for developer-tool declarations and local task names.

Current notable pins and floating tools are:

| Tool | Authority and current declaration |
| --- | --- |
| Go | `go.mod` and `.mise.toml`: `1.26.6`; workflows use `go-version-file: go.mod` |
| SOPS | `.mise.toml`: `3.13.3`; test and Kind workflows install `v3.13.3` |
| Kind, kubectl, Helm, Flux | `deploy-kind.yml` pins Kind `v0.29.0`, kubectl `v1.35.4`, Helm `v3.19.0`, Flux `v2.6.4` for that workflow; `.mise.toml` declares local `latest` for Kind, kubectl, and Helm |
| govulncheck | `.mise.toml` and `vulncheck.yml` currently use `latest`; this is intentionally documented as unpinned |
| Syft | `release.yml` currently installs `github.com/anchore/syft/cmd/syft@latest`; this is unpinned |
| Actions | Workflow files currently pin major action versions such as checkout `v7`, setup-go `v6`, setup-python `v6`, upload-artifact `v7`, download-artifact `v8`, and pre-commit `v3.0.1`; cosign installer is pinned to `v4.1.2` |

The release runner must provide GNU `sha256sum`, Bash features including `shopt`, and GitHub CLI. It also needs outbound access to Sigstore for keyless signing and to Go's module infrastructure for Go and Syft installation. The release contract is keyless cosign `.bundle` files plus `opencenter.spdx.json`; it does not produce certificate and signature files under separate legacy extensions.

## Credentials and artifact handling

* `release.yml` uses the automatic `GITHUB_TOKEN` only for `gh release create`; the required permission is `contents: write`.
* Keyless cosign signing uses GitHub Actions OIDC through `id-token: write`. It does not require a stored cosign private key or a repository secret.
* `deploy-kind.yml` declares `contents: read` and no secrets. It is designed for disposable local Gitea and Kind state, not cloud credentials.
* Workflows with omitted permission blocks inherit repository defaults. Review those defaults and prefer explicit least-privilege permissions when adding new automation.
* Do not upload kubeconfig files, OpenTofu state, SOPS keys, provider credential files, Gitea tokens, bootstrap logs containing sensitive values, or generated secret state. The disposable workflow currently prints only non-secret diagnostics and retains its bootstrap log on the runner.
* If a future workflow needs provider credentials, use the narrowest repository or environment secret scope, prefer short-lived or federated credentials, restrict deployment jobs to trusted refs and protected environments, and keep build/test jobs credential-free.

## Maintenance checklist

When changing a workflow or its runner contract:

1. Update this reference from the changed file under `.github/workflows/`; do not infer repository behavior from illustrative snippets.
2. Record trigger and path changes, job dependencies and matrices, timeout/concurrency behavior, permissions, tool pins, secrets, and artifact or release outputs.
3. Re-check the authoritative versions in `go.mod`, `.mise.toml`, and the workflow itself. Call out any new `latest` install explicitly.
4. For Kind/Gitea changes, re-run the runner acceptance commands, verify the host-IP-to-port-3001 path from the container network, and confirm cleanup after success and failure.
5. For release changes, verify checksum generation, OIDC keyless bundles, SPDX output, and GitHub release permissions without exposing credentials.
6. Update the end-user [CI/CD integration how-to](../operations/integrate-ci-cd.md) only when its illustrative guidance needs to change; keep exact repository facts here.
7. Run the frontmatter and Markdown link checks listed below. Do not claim the Docs P0 workflow passes while its referenced script is absent.

## Validation notes

For local documentation changes, use the repository's remediation audit:

```bash
mise run test-docs-frontmatter-remediation
git diff --check
```

Also check that every relative Markdown link in the changed pages resolves. Run Vale only if it is available locally. The repository's current Docs P0 workflow references `scripts/docs/p0-docs-check.sh`, but that file is absent; this is a known repository limitation, not a successful P0 validation result.

## Known limitation

`docs-p0.yml` invokes `scripts/docs/p0-docs-check.sh`, but `scripts/docs/p0-docs-check.sh` does not exist in the current repository. The workflow can still reach its Vale step when changed files are present only if the missing script is restored or the workflow is changed; neither is part of this documentation update.

## Related documentation

* [Integrate CI/CD](../operations/integrate-ci-cd.md) — end-user CI/CD patterns and the separate OpenStack E2E design.
* [Create a Local Kind Cluster](../operations/create-kind-cluster.md) — local Kind, Gitea, and Flux bootstrap behavior.
* [Kind Cluster Verification Guide](../contributing/kind-cluster-verification.md) — post-bootstrap verification.
* [Build System (Mise)](../contributing/build-system.md) — local tools and tasks.
* [Release Process](../contributing/release-process.md) — release operator procedure.
* [Testing Guide](../contributing/testing-guide.md) — local and CI test coverage.
* [`go.mod`](../../go.mod) — Go module and language version.
* [`.mise.toml`](../../.mise.toml) — local tool and task declarations.
