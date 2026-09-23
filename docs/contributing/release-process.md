---
id: release-process
title: "Release Process"
sidebar_label: Release Process
description: Cut, sign, and publish a release of openCenter-cli using the GitHub Actions release workflow.
doc_type: how-to
audience: "maintainers"
tags: [contributing, release]
---
# Release Process

**Purpose:** For maintainers, shows how to create and publish releases of openCenter-cli.

`.github/workflows/release.yml` is the source of truth for published artifacts -- it is what actually builds, signs, and publishes a release. The `release`/`publish` mise tasks are local preflight helpers only; they do not publish anything. See [GitHub Actions Workflows](../reference/github-actions-workflows.md) for the workflow's full trigger/permission/job breakdown.

## Prerequisites

* Maintainer access to the repository (the workflow needs `contents: write` and `id-token: write`, which come from the repository's default `GITHUB_TOKEN` -- no extra secrets to configure for signing, since cosign runs keyless).
* `gh` CLI installed, for watching the run and inspecting the resulting release (optional).
* All required CI checks green on `main` (see [Testing Guide](testing-guide.md) and [GitHub Actions Workflows](../reference/github-actions-workflows.md)).

## Versioning

Releases are tagged `v<version>` (e.g. `v1.2.0`) and pushed to trigger the workflow. There is no `CHANGELOG.md` to update -- release notes are generated automatically by the workflow (`--generate-notes` from `gh release create`) from the commit range since the previous tag. Historical release notes are also kept as static pages under `docs/release/` (currently `1.0.0-rc01.md` through `1.0.0-rc06.md`); add a new page there if the project wants a durable copy of the notes outside GitHub's release page.

## Step 1: Run the local preflight build (optional)

Before tagging, you can build and inspect the release artifacts locally without publishing anything:

```bash
mise run release v1.2.0
```

This cross-compiles four `dist/opencenter-1.2.0-<os>-<arch>` binaries into `bin/release/`, writes `bin/release/RELEASE_NOTES_1.2.0.md` (grouped from `git log <last-tag>..HEAD --oneline --no-merges` by `feat`/`fix`/`docs` subject prefix), and prints the manual tag/push/`gh release create` commands as a reminder. Nothing is pushed or published by this task.

## Step 2: Verify tests pass

```bash
mise run verify   # test, test-race, test-properties, govulncheck
mise run godog
```

## Step 3: Tag and push

```bash
git tag -a v1.2.0 -m "Release 1.2.0"
git push origin v1.2.0
```

Pushing a `v*` tag triggers `.github/workflows/release.yml`. A manual `workflow_dispatch` trigger is also available from the Actions tab for re-running a release.

## Step 4: What the workflow does

`release.yml` runs three jobs:

1. **`build-cli`** -- matrix over `{linux/amd64, linux/arm64, darwin/amd64, darwin/arm64}`, self-hosted runners. Builds `dist/opencenter-<version>-<os>-<arch>` with full `-ldflags` version metadata (version is the tag with its leading `v` stripped). Uploads each as an artifact.
2. **`build-plugin`** -- same matrix, builds `./cmd/opencenter-local` -> `dist/opencenter-local-<version>-<os>-<arch>` (no ldflags). Uploads each as an artifact.
3. **`release`** (needs both build jobs) -- downloads all artifacts into `dist/`, computes `sha256sum opencenter-* | sort > checksums.txt`, installs `sigstore/cosign-installer`, runs `cosign sign-blob --yes --bundle <artifact>.bundle <artifact>` (keyless signing, `COSIGN_YES=true`) for every artifact and for `checksums.txt`, installs `syft` and generates `dist/opencenter.spdx.json`, then runs `gh release create "$GITHUB_REF_NAME" dist/* --generate-notes` using the workflow's own `GITHUB_TOKEN`.

The resulting GitHub release contains: 4 CLI binaries, 4 `opencenter-local` plugin binaries, `checksums.txt`, a `.bundle` cosign signature next to every signed file, and `opencenter.spdx.json`. The workflow does not build or push a container image.

## Step 5: Watch the run

```bash
gh run list --workflow release.yml
gh run watch
```

## Step 6: Verify the release

* Confirm all expected files are attached to the GitHub release.
* Spot-check a binary:

  ```bash
  curl -L https://github.com/opencenter-cloud/opencenter-cli/releases/download/v1.2.0/opencenter-1.2.0-linux-amd64 -o opencenter
  chmod +x opencenter
  ./opencenter version
  ```
* Optionally verify a cosign bundle:

  ```bash
  cosign verify-blob --bundle opencenter-1.2.0-linux-amd64.bundle \
    --certificate-identity-regexp '.*' --certificate-oidc-issuer-regexp '.*' \
    opencenter-1.2.0-linux-amd64
  ```

## Hotfix releases

```bash
git checkout -b hotfix/1.2.1 v1.2.0
# fix, commit
git checkout main
git merge hotfix/1.2.1   # or open a PR
git push origin main
git tag -a v1.2.1 -m "Hotfix 1.2.1"
git push origin v1.2.1   # triggers release.yml the same way
```

## Pre-releases

Tag with a suffix (`v1.2.0-rc1`) and push; `release.yml` treats any `v*` tag the same way. Mark it as a pre-release afterward with `gh release edit v1.2.0-rc1 --prerelease`, or build it locally first with `mise run release v1.2.0-rc1` to sanity-check binaries before tagging.

## Rollback

GitHub releases are not deleted for a bad release -- mark it as a pre-release (`gh release edit <tag> --prerelease`) with a note pointing at the last good version, then ship a hotfix release per above. Reverting the underlying commits on `main` and cutting a new patch tag is the mechanism; there is no separate "unpublish" workflow.

## Common issues

**Tag already exists:**

```bash
git tag -d v1.2.0
git push origin :refs/tags/v1.2.0
git tag -a v1.2.0 -m "Release 1.2.0"
git push origin v1.2.0
```

**Binary doesn't run on the target platform** -- check `GOOS`/`GOARCH` match the download; rebuild with `mise run build-all` or `mise run release <version>` locally to reproduce.

**Release notes look wrong** -- `gh release create ... --generate-notes` derives notes from merged PR titles/commits since the previous tag; edit the release description on GitHub directly (`gh release edit <tag> --notes "..."`) if it needs correction. This does not affect the signed binaries or checksums.
