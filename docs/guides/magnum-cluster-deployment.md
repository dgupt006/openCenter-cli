# Deploying a Kubernetes Cluster with OpenStack Magnum and openCenter

## A repeatable COE workflow with Magnum, Flux, and GitOps service reconciliation

OpenStack Magnum provides the Container Orchestration Engine (COE) layer for creating Kubernetes clusters. In an openCenter deployment, Magnum creates and owns the control-plane and worker infrastructure, while openCenter completes the workflow by exporting the kubeconfig, bootstrapping Flux, and reconciling the configured services.

This is a different deployment path from the OpenStack/Kubespray provider. Do not copy Kubespray networking, storage, or cloud-controller settings into a Magnum blueprint without confirming that the Magnum cluster template supports them.

## The key authentication requirement

Magnum creates a Keystone trustee and trust for the cluster. In environments that enforce Keystone trust rules, application-credential authentication cannot create that trust. The Magnum provider must therefore use Keystone username/password authentication for cluster creation.

This requirement is the most important preflight check. A deployment using only an application credential may pass configuration and manifest validation, then fail during COE creation with:

```text
Failed to create trustee or trust for Cluster
```

Use a dedicated approved password-auth cloud configuration for Magnum. Keep the username, password, domain values, and cloud configuration outside the blueprint and out of Git.

## Deployment architecture

The workflow consists of these stages:

1. openCenter validates the Magnum configuration and credentials.
2. Magnum creates the COE cluster using the selected cluster template, flavors, keypair, and worker count.
3. openCenter waits for the COE to become healthy.
4. openCenter exports the Kubernetes kubeconfig.
5. Flux is bootstrapped into the new cluster.
6. The SOPS age secret and other generated service secrets are reconciled.
7. Flux reconciles the configured services, including Weave GitOps and Calico.

Magnum health alone is not the final success condition. The cluster is ready only after Kubernetes, Flux, and the selected services pass their health checks.

## Before you begin

Prepare the following:

- An openCenter CLI containing the Magnum password-auth support and current readiness fixes.
- A fresh cluster name and unique GitOps repository path.
- An approved password-auth OpenStack cloud profile.
- A Magnum cluster template compatible with the target Kubernetes version.
- An OpenStack keypair registered in the target project.
- Valid master and worker flavors.
- Access to create a GitOps repository and credentials that Flux can use.
- SOPS age keys generated and stored through the approved local secret workflow.

Never place passwords, tokens, private keys, or application-credential secrets in this document or in the GitOps repository.

## Confirm Magnum resources

Check the target cloud before creating the blueprint:

```sh
export OS_CLOUD=<magnum-password-auth-cloud>

openstack coe cluster template list
openstack flavor show <master-flavor>
openstack flavor show <worker-flavor>
openstack keypair list
```

Record the selected template, master and worker flavors, keypair, master count, worker count, and anti-affinity policy. Do not reuse a stale keypair or template value from another region or an earlier cluster.

## Create the cluster configuration

Set the organization and a new, unique cluster name:

```sh
export ORG=<organization>
export CLUSTER=<fresh-cluster-name>
export GH_USER=<github-user-or-organization>
```

For a first deployment, initialize a complete configuration from the schema:

```sh
opencenter cluster init "$CLUSTER" \
  --org "$ORG" \
  --type magnum \
  --full-schema

CONFIG="$HOME/.config/opencenter/clusters/blueprints/$ORG/$CLUSTER/$CLUSTER-config.yaml"
${EDITOR:-vi} "$CONFIG"
```

The blueprint should include:

- Magnum as the infrastructure provider.
- The target region and cluster template.
- At least one master node.
- The intended worker count and flavors.
- A valid Magnum keypair.
- Soft anti-affinity when supported by the environment.
- Flux enabled.
- Weave GitOps enabled when required by the service profile.
- A unique GitOps repository path for the cluster.

If the blueprint enables an OpenStack cloud integration or CCM service, it must also contain the correctly rendered cloud credentials section expected by that service. Do not enable a provider-owned service without verifying that its credentials are generated for the Magnum cluster.

If a complete, secret-free Magnum profile already exists, it can be used instead:

```sh
opencenter cluster init "$CLUSTER" \
  --org "$ORG" \
  --type magnum \
  --config-file /path/to/sjc3-magnum-profile.yaml \
  --force
```

Create the empty remote GitOps repository and confirm that its URL is configured in `$CONFIG`:

```sh
gh repo create "$GH_USER/$CLUSTER-gitops" --private --disable-wiki
```

Do not initialize the remote repository with a README, `.gitignore`, or license.

## Provision storage for enabled services

Before validation and generation, provision object storage for each enabled S3-backed service. For the standard SJC3 service set:

```sh
export S3_ENDPOINT=https://swift.api.sjc3.rackspacecloud.com

for service in harbor loki mimir tempo velero etcd-backup; do
  opencenter cluster service storage apply "$service" \
    --cluster "$ORG/$CLUSTER" \
    --backend s3 \
    --os-cloud "$OS_CLOUD" \
    --s3-endpoint "$S3_ENDPOINT" \
    --rotate-credentials \
    --yes
done
```

Run this only for services enabled in the cluster configuration. `--rotate-credentials` creates or replaces service credentials; omit it when a complete existing credential pair should be reused.

## Validate the configuration

```sh
opencenter cluster validate "$ORG/$CLUSTER"
```

Resolve every validation error before generating or deploying the cluster.

## Generate, synchronize, and publish GitOps

Generate the cluster manifests and synchronize generated secrets:

```sh
opencenter cluster generate "$ORG/$CLUSTER" --force
opencenter secrets sync "$ORG/$CLUSTER"
```

If generation is repeated, run `secrets sync` again. Generated service manifests can refer to newly rendered encrypted values, and deployment should not begin while required secret manifests are missing.

The deployment command does not commit or push the generated GitOps repository. Publish it before starting deployment:

```sh
GITOPS_DIR="$HOME/.config/opencenter/clusters/gitops/$ORG"
cd "$GITOPS_DIR"

git init
git branch -M main
git remote add origin "https://github.com/$GH_USER/$CLUSTER-gitops.git"
git add -A
git commit -m "Deploy $ORG/$CLUSTER"
git push -u origin main
```

If `origin` already exists, verify it with `git remote -v` instead of adding it again. Whenever generation or secret synchronization changes the GitOps tree, commit and push those changes before deploying.

## Deploy Magnum and bootstrap Kubernetes

Use a writable temporary Helm cache on macOS:

```sh
cache_dir=$(mktemp -d /private/tmp/opencenter-helm-cache.XXXXXX)
HELM_CACHE_HOME="$cache_dir" opencenter cluster deploy "$ORG/$CLUSTER" \
  --break-lock \
  --yes \
  --log-level info
```

The expected Magnum sequence is:

```text
magnum-create
magnum-wait-ready
magnum-export-kubeconfig
flux-bootstrap
service reconciliation
```

The deployment log is stored beneath:

```text
~/.local/state/opencenter/logs/bootstrap/<org>/<cluster>/
```

Keep the log path for release reporting. The workflow is resumable; after correcting a failed prerequisite, resume the same deployment instead of manually installing a second Flux instance or creating duplicate Kubernetes resources.

The generated kubeconfig is written to:

```sh
export KUBECONFIG="$HOME/.config/opencenter/clusters/state/$ORG/$CLUSTER/kubeconfig.yaml"
```

Monitor bootstrap and, once the Kubernetes API is reachable, Flux reconciliation:

```sh
tail -f "$HOME/.local/state/opencenter/logs/bootstrap/$ORG/$CLUSTER/"*.log

kubectl get nodes
kubectl get kustomizations -A
kubectl get helmreleases -A
```

## Bootstrap ordering matters

On a fresh Magnum cluster, Flux creates the `flux-system` namespace during bootstrap. The SOPS age secret must be available in the correct order so that encrypted service manifests can reconcile cleanly.

The supported order is:

1. Export the kubeconfig.
2. Bootstrap Flux.
3. Reconcile the SOPS age secret.
4. Reconcile the remaining services.

Do not repair a clean deployment by manually creating the secret or manually installing Flux. If the order is wrong, fix the bootstrap provider and resume the deployment.

## Verify the completed cluster

First confirm the Magnum COE status:

```sh
openstack --os-cloud <magnum-password-auth-cloud> \
  coe cluster show <cluster-name>
```

Then verify the exported Kubernetes cluster:

```sh
kubectl get nodes \
  -o custom-columns=NAME:.metadata.name,PROVIDER_ID:.spec.providerID,TAINTS:.spec.taints

kubectl get pods -A
kubectl get gitrepositories -A
kubectl get kustomizations,helmreleases -A
```

The release is healthy when:

- Magnum reports the COE as healthy.
- All expected nodes are Ready.
- Nodes have provider IDs where cloud integration requires them.
- Flux GitRepositories and Kustomizations reconcile successfully.
- HelmReleases become Ready.
- Weave GitOps, Calico, and other selected services are healthy.

## Release evidence

Capture the following for each Magnum deployment:

- Organization and cluster name.
- CLI commit and GitOps base release.
- Magnum region, cluster template, flavors, counts, keypair, and anti-affinity policy.
- Cluster validation output.
- Deployment log path.
- Magnum COE status and status reason.
- Exported kubeconfig location, stored securely.
- Node readiness and provider IDs.
- Flux, Calico, Weave GitOps, and HelmRelease status.

Do not include passwords, application-credential secrets, tokens, age private keys, or SSH private keys in the report.

## Final checklist

```text
[ ] Password-auth Magnum cloud profile selected
[ ] Cluster template and flavors verified in the target region
[ ] Fresh cluster name and unique GitOps path selected
[ ] Valid Magnum keypair configured
[ ] At least one master and intended worker count configured
[ ] Flux and required GitOps services enabled
[ ] Empty GitOps repository created and its URL added to the cluster configuration
[ ] Object storage provisioned for every enabled S3-backed service
[ ] Cluster validation passed
[ ] Cluster generated and secrets synchronized after final generation
[ ] Application overlays committed and pushed
[ ] Magnum COE created and healthy
[ ] Generated kubeconfig exported
[ ] Flux bootstrapped before service reconciliation
[ ] Nodes Ready with expected provider IDs
[ ] GitRepositories, Kustomizations, and HelmReleases healthy
[ ] Deployment evidence captured without secrets
```

The result is a controlled handoff between Magnum and openCenter: Magnum owns the cluster infrastructure, while openCenter and Flux complete the Kubernetes platform bootstrap and service lifecycle.
