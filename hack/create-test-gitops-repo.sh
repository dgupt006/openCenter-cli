#!/usr/bin/env bash
set -euo pipefail

DEFAULT_ORG="opencenter-cloud"
DEFAULT_REPO="dev-vp-gitops"
DEFAULT_KEY_PATH="${HOME:?HOME must be set}/.ssh/dev-vp-gitops"
DEPLOY_KEY_TITLE="openCenter dev-vp GitOps test"
KEY_COMMENT="openCenter dev-vp GitOps test"

usage() {
    cat <<'EOF'
Usage: create-test-gitops-repo.sh [org] [repo] [key-path]

Creates or verifies a private GitHub repository and a write-enabled deploy key.
EOF
}

fail() {
    printf 'error: %s\n' "$*" >&2
    exit 1
}

require_command() {
    command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"
}

validate_org() {
    [[ "$1" =~ ^[[:alnum:]]([[:alnum:]-]{0,37}[[:alnum:]])?$ ]] \
        || fail "invalid organization name: $1"
}

validate_repo() {
    [[ "$1" =~ ^[[:alnum:]]([[:alnum:]_.-]{0,98}[[:alnum:]_.-])?$ ]] \
        || fail "invalid repository name: $1"
}

validate_key_path() {
    local path="$1"

    [[ -n "$path" ]] || fail "key path must not be empty"
    [[ "$path" == /* ]] || fail "key path must be absolute: $path"
    [[ "$path" != "/" && "$path" != */ ]] || fail "unsafe key path: $path"
    case "$path" in
        *$'\n'*|*$'\r'*|*/../*|*/..|../*)
            fail "unsafe key path: $path"
            ;;
    esac
    [[ "$path" != *.pub ]] || fail "key path must name the private key, not a .pub file: $path"
}

path_present() {
    [[ -e "$1" || -L "$1" ]]
}

read_deploy_keys() {
    gh api --paginate "repos/$1/keys" \
        --jq '.[] | [.id, .title, .read_only, .key] | @tsv'
}

public_key_material() {
    local key="$1"
    local key_type key_body ignored

    read -r key_type key_body ignored <<< "$key"
    [[ -n "$key_type" && -n "$key_body" ]] || return 1
    printf '%s %s' "$key_type" "$key_body"
}

deploy_key_matches() {
    local repo="$1"
    local expected_key="$2"
    local deploy_keys
    local id title read_only existing_key
    local expected_material existing_material
    local matches=0

    expected_material="$(public_key_material "$expected_key")" \
        || fail "invalid expected public key"

    if ! deploy_keys="$(read_deploy_keys "$repo")"; then
        fail "unable to list deploy keys for repository: $repo"
    fi
    while IFS=$'\t' read -r id title read_only existing_key; do
        [[ "$title" == "$DEPLOY_KEY_TITLE" ]] || continue
        matches=$((matches + 1))
        [[ "$read_only" == "false" ]] \
            || fail "deploy key '$DEPLOY_KEY_TITLE' exists but is read-only"
        existing_material="$(public_key_material "$existing_key")" \
            || fail "deploy key '$DEPLOY_KEY_TITLE' contains an invalid public key"
        [[ "$existing_material" == "$expected_material" ]] \
            || fail "deploy key '$DEPLOY_KEY_TITLE' exists with a different public key"
    done <<< "$deploy_keys"

    if [[ "$matches" -eq 0 ]]; then
        return 1
    fi
    [[ "$matches" -eq 1 ]] \
        || fail "multiple deploy keys use the title '$DEPLOY_KEY_TITLE'"
}

verify_deploy_key() {
    deploy_key_matches "$@" \
        || fail "deploy key '$DEPLOY_KEY_TITLE' was not found after setup"
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
    usage
    exit 0
fi
[[ "$#" -le 3 ]] || { usage >&2; exit 2; }

org="${1:-$DEFAULT_ORG}"
repo="${2:-$DEFAULT_REPO}"
key_path="${3:-$DEFAULT_KEY_PATH}"
validate_org "$org"
validate_repo "$repo"
validate_key_path "$key_path"

require_command gh
require_command ssh-keygen
gh auth status --active >/dev/null 2>&1 \
    || fail "gh is not authenticated with an active account"

public_key_path="$key_path.pub"
private_present=false
public_present=false
path_present "$key_path" && private_present=true
path_present "$public_key_path" && public_present=true

if [[ "$private_present" != "$public_present" ]]; then
    fail "private and public key files must both exist or both be absent"
fi

if [[ "$private_present" == false ]]; then
    key_dir="$(dirname "$key_path")"
    mkdir -p "$key_dir"
    ssh-keygen -q -t ed25519 -N '' -C "$KEY_COMMENT" -f "$key_path" >/dev/null
fi

[[ -f "$key_path" && -f "$public_key_path" ]] \
    || fail "key paths must reference regular files: $key_path and $public_key_path"
chmod 600 "$key_path"
chmod 644 "$public_key_path"
ssh-keygen -q -lf "$public_key_path" >/dev/null \
    || fail "invalid public key: $public_key_path"
public_key="$(<"$public_key_path")"
[[ -n "$public_key" ]] || fail "public key is empty: $public_key_path"

repo_name="$org/$repo"
repo_response=""
repo_exists=false
if repo_response="$(gh api --include --silent "repos/$repo_name" 2>/dev/null)"; then
    repo_exists=true
elif ! printf '%s\n' "$repo_response" \
    | grep -Eq '^HTTP/[0-9.]+[[:space:]]+404([[:space:]]|$)'; then
    fail "unable to determine whether repository exists: $repo_name"
fi

if [[ "$repo_exists" == false ]]; then
    gh repo create "$repo_name" --private >/dev/null
fi

repo_metadata="$(gh api "repos/$repo_name" --jq '[.full_name, .private] | @tsv')"
IFS=$'\t' read -r actual_repo actual_private <<< "$repo_metadata"
expected_repo_lower="$(printf '%s' "$repo_name" | tr '[:upper:]' '[:lower:]')"
actual_repo_lower="$(printf '%s' "$actual_repo" | tr '[:upper:]' '[:lower:]')"
[[ "$actual_repo_lower" == "$expected_repo_lower" ]] \
    || fail "repository owner/name mismatch: expected $repo_name, got $actual_repo"
[[ "$actual_private" == "true" ]] \
    || fail "repository is not private: $actual_repo"

if ! deploy_key_matches "$actual_repo" "$public_key"; then
    gh api --method POST "repos/$actual_repo/keys" \
        -f "title=$DEPLOY_KEY_TITLE" \
        -f "key=$public_key" \
        -F 'read_only=false' >/dev/null
fi

verify_deploy_key "$actual_repo" "$public_key"
printf 'Repository: %s (private)\n' "$actual_repo"
printf 'SSH URL: git@github.com:%s.git\n' "$actual_repo"
printf 'Deploy key: %s (write-enabled)\n' "$DEPLOY_KEY_TITLE"
printf 'Private key path: %s\n' "$key_path"
printf 'Public key path: %s\n' "$public_key_path"
