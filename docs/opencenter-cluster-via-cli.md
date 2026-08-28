---
id: openstack-cluster-via-cli
title: "Create and Deploy an OpenStack Cluster with the CLI"
sidebar_label: OpenStack Cluster via CLI
description: Create, configure, validate, generate, and deploy an OpenStack cluster with explicit provider discovery and per-service storage provisioning.
doc_type: tutorial
audience: "platform engineers, cluster operators"
tags: [openstack, cluster, cli, provider, storage, gitops]
---
# Create and deploy an OpenStack cluster with the CLI

**Purpose:** Follow this tutorial to create one OpenStack-backed cluster configuration, reconcile provider metadata safely, optionally provision storage for individual services, and deploy the generated GitOps repository.

## Prerequisites

- Go installed and the CLI built from source:
  ```bash
  git clone https://github.com/opencenter-cloud/opencenter-cli.git
  cd opencenter-cli
  go build -o opencenter .
  ```
- A selected `clouds.yaml` profile for the target OpenStack project. Provider discovery uses authenticated read operations, so an application-credential profile can be used. Storage apply additionally requires permission to ensure the object-store container and create the credential type required by the selected backend.
- An empty Git repository for the cluster's GitOps configuration and a GitHub token with write access if you use token authentication. The optional helper below can create a disposable private GitHub repository and SSH deploy key instead.
- `kubectl`, and `flux` if you will perform the post-deploy checks.

The commands below use `<org>/<cluster-name>` as the cluster identifier and `<clouds.yaml-profile>` as the profile name.

## 1. Initialize and select the cluster

```bash
opencenter cluster init <cluster-name> --org <org-name> --type openstack
opencenter cluster use <org>/<cluster-name>
```

`cluster init` creates a schema `2.0` configuration and generates SOPS Age and SSH keys unless key generation is disabled. `cluster use` makes the target explicit for commands that accept the active cluster.

Check the generated file and confirm that `opencenter.infrastructure.ssh.authorized_keys` is populated. The [init reference](reference/opencenter/opencenter_cluster_init.md) lists the available initialization options.

## 2. Discover and apply OpenStack provider metadata

Provider reconciliation is deliberately split into a read-only plan and a local apply. Start with discovery:

```bash
opencenter cluster provider openstack plan <org>/<cluster-name> \
  --os-cloud <clouds.yaml-profile>
```

The command authenticates to OpenStack and reads provider inventory. It can propose typed values for fields such as `auth_url`, `region`, `project_id`, `image_id`, `network_id`, `subnet_id`, `router_external_network_id`, and `availability_zone`. The plan does not write the cluster file and does not create or change any OpenStack resource, container, or credential. Its text output reports `Changes`, any `Selection required` entries, warnings, and `Remote actions: none`.

If the plan reports ambiguity, rerun it with the ID for each required selection. The selector flags correspond to the typed OpenStack fields:

```bash
opencenter cluster provider openstack plan <org>/<cluster-name> \
  --os-cloud <clouds.yaml-profile> \
  --image-id <linux-image-id> \
  --windows-image-id <windows-image-id> \
  --network-id <internal-network-id> \
  --external-network-id <external-network-id> \
  --subnet-id <internal-subnet-id> \
  --availability-zone <availability-zone>
```

Use only the selectors that the plan requires. The subnet candidates are narrowed to the selected internal network. Do not use an external network as `--network-id` or an external subnet as `--subnet-id`.

To have generated OpenTofu create and manage the internal network and subnet instead of selecting existing OpenStack resources, use the shared flag on both plan and apply:

```bash
opencenter cluster provider openstack plan <org>/<cluster-name> \
  --os-cloud <clouds.yaml-profile> \
  --create-internal-network
```

Create mode intentionally leaves already-empty internal network and subnet mirrors empty and clears populated top-level and nested `network_id`/`subnet_id` mirrors (and `network_name`) so OpenTofu owns those values. It skips internal network and subnet discovery and reports `internal_network_mode: tofu-managed` in structured output or `Internal network mode: tofu-managed` in text, including a no-op plan. If any internal selection is already populated, add `--replace` to both plan and apply before it can be cleared. Create mode rejects `--network-id` and `--subnet-id`, and it cannot be used while `opencenter.infrastructure.cloud.openstack.networking.vlan.id` is set. Provider plan/apply remains read-only against OpenStack; resource creation occurs later through generated OpenTofu.

A populated provider field is protected from replacement. If the desired discovered value differs from a populated value, add `--replace` to the plan and the corresponding apply. `--replace` permits replacing populated provider selections; it is not required merely to fill blank or placeholder values.

To preview optional profile imports, add the relevant flags to the plan:

```bash
opencenter cluster provider openstack plan <org>/<cluster-name> \
  --os-cloud <clouds.yaml-profile> \
  --import-auth \
  --import-tls
```

- `--import-auth` imports the selected profile's complete application-credential ID and secret into the typed provider fields. It requires both values in `clouds.yaml`; a name-only application credential cannot be persisted.
- `--import-tls` imports the profile CA and TLS settings. If the profile disables verification and the configuration would change to `insecure: true`, use `--replace --import-tls` deliberately.

Apply the reviewed plan with the same selectors and import flags:

```bash
opencenter cluster provider openstack apply <org>/<cluster-name> \
  --os-cloud <clouds.yaml-profile> \
  --image-id <linux-image-id> \
  --network-id <internal-network-id> \
  --external-network-id <external-network-id> \
  --subnet-id <internal-subnet-id> \
  --availability-zone <availability-zone> \
  --import-auth \
  --import-tls
```

For OpenTofu-managed internal networking, apply the same mode instead of supplying network or subnet selectors:

```bash
opencenter cluster provider openstack apply <org>/<cluster-name> \
  --os-cloud <clouds.yaml-profile> \
  --create-internal-network
```

If populated internal selections must be cleared, include `--replace` on both the plan and apply commands. `apply` recomputes the plan, validates the candidate, checks that the source file did not change, writes a backup, and atomically persists the typed configuration. In text mode it prints an apply review and asks `Apply the provider-only OpenStack patch? [y/N]`; use the global `--yes` for non-interactive execution. Structured `--output json` or `--output yaml` apply requires `--yes`. Global `--dry-run` stops before persistence. Provider plan/apply has no remote mutation path; storage provisioning is a separate workflow below. See the [provider plan](reference/opencenter/opencenter_cluster_provider_openstack_plan.md) and [provider apply](reference/opencenter/opencenter_cluster_provider_openstack_apply.md) references for the complete flag set.

Discovery is field-aware. Populated non-placeholder, non-network cluster values are preserved, including values configured manually. Blank or recognized placeholder values can be filled from discovery. If discovery proposes a different value for a populated field, request that explicit replacement with `--replace` on both plan and apply; do not use `--replace` merely to fill blanks.

## 3. Edit the typed cluster configuration

Open the configuration and complete the values that are specific to the cluster, GitOps repository, and enabled services:

```bash
opencenter cluster edit <org>/<cluster-name>
```

Use the v2 field names below. The provider operation may already have filled some OpenStack values; preserve them unless you intentionally replace them through the provider plan/apply flow.

```yaml
opencenter:
  infrastructure:
    compute:
      master_count: 3
      worker_count: 3
    cloud:
      openstack:
        auth_url: https://keystone.example.com/v3
        region: DFW3
        project_id: <project-id>
        project_name: <project-name>
        application_credential_id: <application-credential-id>
        application_credential_secret: <application-credential-secret>
        image_id: <linux-image-id>
        image_id_windows: <windows-image-id>
        network_id: <internal-network-id>
        subnet_id: <internal-subnet-id>
        floating_network_id: <external-network-id>
        router_external_network_id: <external-network-id>
        networking:
          network_id: <internal-network-id>
          subnet_id: <internal-subnet-id>
          floating_network_id: <external-network-id>
          router_external_network_id: <external-network-id>
    bastion:
      image: <linux-image-id>
    ssh:
      authorized_keys:
        - <ssh-public-key-generated-by-init>
  gitops:
    repository:
      url: https://github.com/<org>/<gitops-repo>.git
      path: applications/overlays/<cluster-name>
    auth:
      token:
        token: <github-token>
  services:
    loki:
      enabled: true
    tempo:
      enabled: true
secrets:
  keycloak:
    admin_password: <generated-keycloak-password>
  headlamp:
    oidc_client_secret: <real-secret-if-enabled>
```

The `network_id` and `subnet_id` values are the internal node network and subnet. The external values belong in `floating_network_id` and `router_external_network_id`. Replace every placeholder required by the enabled services; do not leave `CHANGEME` values in the configuration or generated manifests. Generate local secret material with, for example:

```bash
openssl rand -base64 24
```

If a service is not present in `opencenter.services`, enable it before using the storage workflow:

```bash
opencenter cluster service enable loki
opencenter cluster service enable tempo
```

As an alternative to editing YAML, use `opencenter cluster set` for individual values. Each invocation applies its assignments atomically. For example, set the repository URL and generate a local Keycloak password without putting either value in the tutorial:

```bash
opencenter cluster set <org>/<cluster-name> \
  opencenter.gitops.repository.url=https://github.com/<org>/<gitops-repo>.git

opencenter cluster set <org>/<cluster-name> \
  secrets.keycloak.admin_password="$(openssl rand -base64 24)"
```

To switch an existing HTTPS token configuration to SSH in one atomic update, use the exact terminal pointer `opencenter.gitops.auth.token=null` and set both SSH key paths in the same command:

```bash
opencenter cluster set <org>/<cluster-name> \
  opencenter.gitops.repository.url=ssh://git@github.com/<org>/<gitops-repo>.git \
  opencenter.gitops.auth.token=null \
  opencenter.gitops.auth.ssh.private_key=<ssh-private-key-path> \
  opencenter.gitops.auth.ssh.public_key=<ssh-public-key-path>
```

The exact `null` value clears the token pointer block; it is not the same as clearing only a nested token value. Use this pattern for local standalone secrets such as a generated Keycloak password. Externally issued storage credentials must come from the storage provisioning workflow, not from random values.

If you need a disposable GitHub target for testing, run the repo-local helper with the generic signature `hack/create-test-gitops-repo.sh [org] [repo] [key-path]`:

```bash
./hack/create-test-gitops-repo.sh <org> <gitops-repo> <ssh-key-path>
```

The helper creates or verifies a private repository, creates an Ed25519 keypair, registers a write-enabled deploy key, and is idempotent and fail-safe. It requires authenticated `gh` and `ssh-keygen`. It does not print the private key; keep the generated private key path local and use the public key only where a deploy key or configuration requires it.

## 4. Optionally provision storage for one service

Storage is explicit and independent from provider reconciliation. Run the plan/apply pair once for each service that needs an OpenStack object store; the CLI does not infer storage from the provider plan and does not provision all services in one operation.

Supported pairs are:

- `loki` with `swift` or `s3`
- `tempo` with `swift` or `s3`
- `etcd-backup` with `s3` only
- `velero` with `s3` only

For example, plan and apply Swift storage for Loki:

```bash
opencenter cluster service storage plan loki \
  --cluster <org>/<cluster-name> \
  --backend swift \
  --os-cloud <clouds.yaml-profile>

opencenter cluster service storage apply loki \
  --cluster <org>/<cluster-name> \
  --backend swift \
  --os-cloud <clouds.yaml-profile>
```

For an S3-compatible backend, use the same pair with `--backend s3`. Add `--container <container-or-bucket-name>` to choose the name explicitly, or `--s3-endpoint <https-url>` when the endpoint is distinct from the profile's endpoint:

```bash
opencenter cluster service storage plan velero \
  --cluster <org>/<cluster-name> \
  --backend s3 \
  --os-cloud <clouds.yaml-profile> \
  --container <cluster-name>-velero \
  --s3-endpoint <s3-endpoint-url>
```

Storage plan performs preflight and reports the typed changes, redacted secret paths, and ordered remote actions without ensuring a container, creating a credential, persisting the configuration, or revoking a credential. When credential creation, rotation, or revocation is required, preflight resolves the credential owner from the explicit `auth.user_id` override in `clouds.yaml`; otherwise it derives the owner from `token.user.id` in the already-authenticated Keystone v3 response. No identity API lookup is performed, and the resolved owner is omitted from text and structured output. Owner resolution and all other preflight checks complete before confirmation, container creation, or any local write, so an unresolved owner is an ordinary preflight error.

Existing complete credentials are reused. A partial credential pair blocks both plan and apply until `--rotate-credentials` is supplied to both commands. Credential values for functional Loki and Tempo storage must be created by storage apply; do not put random placeholders in the configuration or use a generated standalone password as a substitute for an externally issued storage credential.

As with provider apply, text mode prompts for confirmation and `--yes` accepts it non-interactively. Structured apply with `--output json` or `--output yaml` requires `--yes`. Global `--dry-run` prevents storage remote actions and local persistence. A failure before credential creation is an ordinary error. Failures after credential creation, configuration persistence, or revocation tracking can leave a partial operation; the command reports status `partial`, exits with code `4`, and retains a recovery record at the cluster config path with suffix `.storage-recovery.json`. Preserve that record and use its redacted status to guide recovery. Sensitive values are never included in output. See the [storage plan](reference/opencenter/opencenter_cluster_service_storage_plan.md) and [storage apply](reference/opencenter/opencenter_cluster_service_storage_apply.md) references for all options.

This workflow is different from `opencenter secrets sync`: storage apply provisions OpenStack object-store containers and credentials and writes the typed service/secret fields; `secrets sync` only reads the configured secrets and writes SOPS-encrypted Kubernetes manifests. Run storage provisioning before generation and `secrets sync` when the generated manifests should include the resulting credentials.

## 5. Validate and generate GitOps manifests

Run the offline validation and generation steps in order:

```bash
opencenter cluster validate <org>/<cluster-name>
opencenter cluster generate <org>/<cluster-name>
```

By default, `cluster validate` uses offline validation. Use `--validation online` only when you also want provider and Git remote checks. `cluster generate` validates before generation unless `--skip-validation` is supplied. Generation may encrypt overlay content, so treat its output as sensitive. Re-run both commands after editing the configuration.

Check that validation reports no errors and that generation creates or updates the expected local repository files. The generated repository should contain the cluster's application overlays and Flux configuration.

## 6. Synchronize encrypted Kubernetes secrets

```bash
opencenter secrets sync <org>/<cluster-name>
```

This reads secrets from the cluster configuration, preserves non-secret manifest fields, and writes SOPS-encrypted service manifests. It does not create OpenStack credentials or containers.

Check the GitOps repository for new or updated `secret.yaml` files and verify that secret data is encrypted as `ENC[...]`, not stored as plaintext.

## 7. Validate manifests and publish the GitOps repository

```bash
opencenter cluster validate <org>/<cluster-name> --manifests
```

Resolve any security findings before publishing the GitOps repository. `openCenter` currently has no GitOps commit or push command, so publication remains an explicit external Git step from the generated repository:

```bash
git add -A
git commit -m "deploy <org>/<cluster-name>"
git push
```

Confirm the GitOps repository contains the generated and encrypted files and that its CI checks pass. `cluster deploy` does not commit or push this repository for you.

## 8. Deploy and verify

Preview the deployment with the global dry-run flag:

```bash
opencenter --dry-run cluster deploy <org>/<cluster-name>
```

This previews the deployment workflow without mutating remote or local state, but it does not fully validate every deployment prerequisite. Complete the offline, generation, secret-sync, manifest-validation, and external publication steps before deploying:

```bash
opencenter cluster deploy <org>/<cluster-name>
```

Deploy provisions infrastructure, bootstraps Kubernetes, and hands off ongoing reconciliation to Flux. It consumes the already-published GitOps repository; it does not publish it. The operation is resumable: after fixing a failed step, rerun `cluster deploy` for the same cluster.

When the cluster is reachable, verify the initial state:

```bash
kubectl get nodes
kubectl get kustomization -n flux-system
kubectl get pods -A
```

Flux reconciliation can continue after `deploy` returns. For targeted diagnosis, use:

```bash
flux get kustomizations -A
flux get helmreleases -A
flux get sources git -A
flux logs --tail=50
flux reconcile kustomization <name> --with-source
```

The command references for [validate](reference/opencenter/opencenter_cluster_validate.md), [generate](reference/opencenter/opencenter_cluster_generate.md), [secrets sync](reference/opencenter/opencenter_secrets_sync.md), and [deploy](reference/opencenter/opencenter_cluster_deploy.md) document the complete option sets.
