#!/usr/bin/env bash
# e2e-kind.sh — end-to-end test for the Kind provider with a local Gitea.
#
# Full lifecycle: build CLI, start local services (RustFS + Gitea), init the
# cluster config, point GitOps at the local Gitea, fill service secrets,
# validate, generate, sync secrets, deploy, then verify nodes and Flux
# reconciliation from inside the cluster. Tears everything down on exit
# unless --keep is passed. On failure the script exits non-zero; partial
# state may remain in place for inspection (teardown is best-effort).
#
# Usage:
#   scripts/e2e-kind.sh [--runtime docker|podman] [--org ORG] [--cluster NAME] \
#                       [--managed-cni] [--debug] [--keep]
#
# Requirements:
#   - container runtime (docker or podman) with a running daemon
#   - go, kind, kubectl, flux, python3 on PATH
#   - ~8 GiB free RAM; roughly 10 minutes wall clock at default sizing
set -uo pipefail

# ---------------------------------------------------------------------------
# Flags
# ---------------------------------------------------------------------------
RUNTIME="${CONTAINER_RUNTIME:-podman}"
ORG="e2e"
CLUSTER="e2e"
KEEP=0
MANAGED_CNI=0
DEBUG=0
args=("$@")
i=0
while [ $i -lt ${#args[@]} ]; do
  case "${args[$i]}" in
    --runtime)     i=$((i+1)); RUNTIME="${args[$i]:-}";;
    --org)         i=$((i+1)); ORG="${args[$i]:-}";;
    --cluster)     i=$((i+1)); CLUSTER="${args[$i]:-}";;
    --keep)        KEEP=1;;
    --managed-cni) MANAGED_CNI=1;;
    --debug)       DEBUG=1;;
    *) echo "unknown flag: ${args[$i]}" >&2; exit 2;;
  esac
  i=$((i+1))
done

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN="$SCRIPT_DIR/bin/opencenter"
LOCAL_BIN="$SCRIPT_DIR/bin/opencenter-local"
HOST_IP="$(hostname -I 2>/dev/null | awk '{print $1}' || ipconfig getifaddr en0 2>/dev/null)"
TEARDOWN_DONE=0

say()  { printf '\n\033[1;34m== %s ==\033[0m\n' "$*"; }
fail() { printf '\n\033[1;31mFAIL: %s\033[0m\n' "$*" >&2; exit 1; }
run()  { say "RUN: $*"; "$@" || fail "command failed: $*"; }

teardown() {
  if [ "$TEARDOWN_DONE" = "1" ] || [ "$KEEP" = "1" ]; then
    [ "$KEEP" = "1" ] && echo "(--keep: leaving local state in place)"
    return
  fi
  TEARDOWN_DONE=1
  say "Teardown"
  "$BIN" cluster destroy "$ORG/$CLUSTER" --force --remove-files >/dev/null 2>&1 || true
  "$LOCAL_BIN" gitea destroy >/dev/null 2>&1 || true
  "$LOCAL_BIN" rustfs destroy >/dev/null 2>&1 || true
}
trap teardown EXIT

[ -n "$HOST_IP" ] || fail "could not determine host IP (tried hostname -I, ipconfig getifaddr en0)"

# ---------------------------------------------------------------------------
# 1. Build
# ---------------------------------------------------------------------------
say "Build CLI"
# Build both binaries directly (no mise) so the script runs on plain CI
# toolchains. The main binary handles cluster/secrets commands; the local
# plugin is invoked from its own fresh build so 'local' subcommands always
# match this checkout (the main binary's embedded plugin may be stale).
( cd "$SCRIPT_DIR" && \
  git_commit=$(git rev-parse HEAD 2>/dev/null || echo unknown) && \
  go build -trimpath \
    -ldflags "-X main.version=0.0.1 -X main.gitCommit=${git_commit}" \
    -o bin/opencenter . && \
  go build -trimpath -o bin/opencenter-local ./cmd/opencenter-local ) \
  || fail "go build failed"
"$BIN" --version >/dev/null || fail "built binary does not run"

# ---------------------------------------------------------------------------
# 2. Local services
# ---------------------------------------------------------------------------
say "Start RustFS (S3 backend for Loki/Tempo)"
# The plugin's strict drift checks can lock a leftover container out of up and
# destroy; on failure, remove it + state and retry once clean.
if ! "$LOCAL_BIN" rustfs up --runtime "$RUNTIME"; then
  echo "rustfs up failed (likely drift); removing stale container + state and retrying"
  "$RUNTIME" rm -f rustfs >/dev/null 2>&1 || true
  rm -rf "$HOME/.config/opencenter/local/rustfs"
  run "$LOCAL_BIN" rustfs up --runtime "$RUNTIME"
fi
RUSTFS_CREDS="$HOME/.config/opencenter/local/rustfs/credentials.json"
[ -f "$RUSTFS_CREDS" ] || fail "rustfs credentials file not written"
say "Start local Gitea"
run "$LOCAL_BIN" gitea up
"$LOCAL_BIN" gitea status | grep -qE "^Running: *true" || fail "gitea is not running"
USER_TOKEN="$HOME/.config/opencenter/local/tokens/gitea-user.token"
[ -f "$USER_TOKEN" ] || fail "user token not found at $USER_TOKEN"
# Extract S3 credentials written by 'rustfs up'; refuse to run with empty values.
PY="$(command -v /usr/bin/python3 || command -v python3 || true)"
[ -n "$PY" ] || fail "python3 not found on PATH"
ACCESS_KEY=$("$PY" -c "import json,sys;print(json.load(open('$RUSTFS_CREDS'))['access_key'])") \
  || ACCESS_KEY=""
SECRET_KEY=$("$PY" -c "import json,sys;print(json.load(open('$RUSTFS_CREDS'))['secret_key'])") \
  || SECRET_KEY=""
[ -n "$ACCESS_KEY" ] && [ -n "$SECRET_KEY" ] || fail "could not read access_key/secret_key from $RUSTFS_CREDS"
GIT_URL="https://$HOST_IP:3001/newuser/test-repo.git"

# ---------------------------------------------------------------------------
# 3. Init configuration
# ---------------------------------------------------------------------------
say "Init cluster config ($ORG/$CLUSTER)"
init_args=("cluster" "init" "$CLUSTER" "--org" "$ORG" "--type" "kind")
[ "$MANAGED_CNI" = "1" ] && init_args+=("--kind-disable-default-cni")
run "$BIN" "${init_args[@]}"

# ---------------------------------------------------------------------------
# 4. Point GitOps at local Gitea + fill secrets, make valid
# ---------------------------------------------------------------------------
say "Configure GitOps + service secrets"
run "$BIN" cluster set "$ORG/$CLUSTER" \
  "opencenter.gitops.repository.url=$GIT_URL" \
  "opencenter.gitops.auth.token.token_file=$USER_TOKEN" \
  "opencenter.services.loki.s3_endpoint=http://$HOST_IP:9000" \
  "opencenter.services.tempo.s3_endpoint=http://$HOST_IP:9000" \
  "secrets.loki.s3_access_key_id=$ACCESS_KEY" \
  "secrets.loki.s3_secret_access_key=$SECRET_KEY" \
  "secrets.tempo.access_key=$ACCESS_KEY" \
  "secrets.tempo.secret_key=$SECRET_KEY" \
  "secrets.keycloak.admin_password=e2e-keycloak-admin-password" \
  "opencenter.services.keycloak.instances=2"   # DoNotSchedule spread: cap at worker count (kind default 2)

say "Validate (expect: passed, 0 failed)"
out=$("$BIN" cluster validate "$ORG/$CLUSTER" 2>&1) || fail "validation failed:
$out"
printf '%s\n' "$out" | tail -3
printf '%s\n' "$out" | grep -qiE "[0-9]+ ?failed" && \
  ! printf '%s\n' "$out" | grep -qE "0 ?failed" && fail "validation reported failures:
$out"

# ---------------------------------------------------------------------------
# 5. Generate, sync secrets, commit gitops tree
# ---------------------------------------------------------------------------
GITOPS_DIR="$HOME/.config/opencenter/clusters/gitops/$ORG"
say "Generate manifests + sync secrets"
run "$BIN" cluster generate "$ORG/$CLUSTER"
run "$BIN" secrets sync "$ORG/$CLUSTER"

git -C "$GITOPS_DIR" add -A
git -C "$GITOPS_DIR" -c user.name=e2e -c user.email=e2e@example.com \
  commit -m "e2e: $CLUSTER gitops manifests" >/dev/null \
  || fail "committing gitops tree failed"

# ---------------------------------------------------------------------------
# 6. Deploy
# ---------------------------------------------------------------------------
say "Deploy ($ORG/$CLUSTER via $RUNTIME; several minutes)"
deploy_args=(cluster "deploy" "$ORG/$CLUSTER" "--container-runtime" "$RUNTIME")
[ "$DEBUG" = "1" ] && deploy_args+=("--debug")
run "$BIN" "${deploy_args[@]}"
KUBECONFIG_PATH="$HOME/.config/opencenter/clusters/state/$ORG/$CLUSTER/kubeconfig.yaml"
[ -f "$KUBECONFIG_PATH" ] || fail "kubeconfig not written at $KUBECONFIG_PATH"

# ---------------------------------------------------------------------------
# 7. Verify
# ---------------------------------------------------------------------------
say "Verify cluster"
export KUBECONFIG="$KUBECONFIG_PATH"

kubectl wait --for=condition=Ready nodes --all --timeout=600s \
  || fail "not all nodes Ready within 10m"
ready_count=$(kubectl get nodes --no-headers | grep -c Ready)
[ "$ready_count" -ge 3 ] || fail "expected >=3 Ready nodes, got $ready_count"
echo "nodes Ready: $ready_count"

# Flux source must reconcile from a URL reachable from inside the cluster:
# either the configured host-routable IP (port-forwarded into the podman
# bridge, used when gitea stays attached to the Kind network) or the
# in-cluster "gitea" DNS alias.
src_status=$(flux get sources git -n flux-system | sed -n '2p')
printf '%s\n' "$src_status" | grep -q "True" \
  || fail "GitRepository flux-system not True:
$(flux get sources git -n flux-system)"
repo_url=$(kubectl get gitrepository flux-system -n flux-system -o jsonpath='{.spec.url}')
case "$repo_url" in
  *"$HOST_IP":3001*|*"gitea:3001"*) echo "GitRepository url reachable in-cluster: $repo_url";;
  *) fail "GitRepository url not reachable in-cluster (got: $repo_url)";;
esac

kubectl get kustomization flux-system -n flux-system \
  -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' | grep -q True \
  || fail "kustomization flux-system not Ready"

# Source-secret CA must match the current Gitea CA (the rotation-race fix)
diff <(openssl x509 -in "$HOME/.config/opencenter/local/gitea/gitea/certs/ca.pem" -noout -fingerprint) \
     <(kubectl -n flux-system get secret flux-system -o jsonpath='{.data.ca\.crt}' | base64 -d | openssl x509 -noout -fingerprint) \
  || fail "source-secret CA does not match the current Gitea CA"

say "PASS: $ORG/$CLUSTER deployed and reconciled from local Gitea"
