---
id: backup-and-restore
title: "Backup and Restore"
sidebar_label: Backup and Restore
description: How to configure etcd backups and disaster recovery using Velero.
doc_type: how-to
audience: "operators, platform engineers"
tags: [backup, restore, velero, etcd, disaster-recovery]
---
# Backup and Restore

**Purpose:** For operators, shows how to configure etcd backups and disaster recovery using Velero for complete cluster backup and restore capabilities.

This guide covers configuring automated etcd backups, Velero for application backups, and complete disaster recovery procedures.

## Prerequisites

* Existing openCenter cluster
* S3-compatible storage (AWS S3, MinIO, Ceph, etc.)
* S3 credentials (access key, secret key)
* Basic understanding of Kubernetes resources

## Task Summary

Configure automated backups for cluster state (etcd) and application data (persistent volumes, resources) to enable disaster recovery and cluster migration.

## Backup Strategy

openCenter provides two complementary backup solutions:

1. **etcd Backup:** Cluster state (API objects, configurations)
2. **Velero Backup:** Application data (persistent volumes, resources)

**Why both:**

* etcd backup: Fast cluster state recovery
* Velero backup: Application-level backup with PV snapshots
* Together: Complete disaster recovery capability

## Part 1: Configure etcd Backup

### 1. Edit Cluster Configuration

Enable and configure etcd backup:

```bash
opencenter cluster edit my-cluster
```

### 2. Configure etcd Backup

When enabled, etcd-backup requires an absolute HTTP(S) S3 endpoint, bucket, region, and service-specific credentials:

```yaml
opencenter:
  services:
    etcd-backup:
      enabled: true
      s3_endpoint: "https://s3.example.com"
      s3_bucket_name: "my-cluster-etcd-backups"
      s3_region: "us-east-1"

secrets:
  etcd_backup:
    access_key_id: "<access-key-id>"
    secret_access_key: "<secret-access-key>"
```

**Configuration options:**

* `opencenter.services.etcd-backup.enabled`: Disabled by default; enable only with the required storage and secrets configuration.
* `opencenter.services.etcd-backup.s3_endpoint`: Required absolute HTTP(S) S3-compatible endpoint URL. Runtime prefers this full endpoint.
* `opencenter.services.etcd-backup.s3_bucket_name`: Required target bucket name; the configured value is honored.
* `opencenter.services.etcd-backup.s3_region`: Required S3 signing region.
* `secrets.etcd_backup.access_key_id`: Required service-specific S3 access key ID.
* `secrets.etcd_backup.secret_access_key`: Required service-specific S3 secret access key.
* `opencenter.services.etcd-backup.s3_host`: Legacy compatibility field; runtime prefers `s3_endpoint`.
* `opencenter.services.etcd-backup.s3_credential_id`: Non-secret OpenStack credential lifecycle metadata, not a pod secret.

The service does not use global AWS credentials as a fallback. It runs nightly at 01:00, uses SigV4, materializes its SOPS-managed Secret through generated kustomization, and uses a digest-pinned image. Backup retention and encryption-at-rest policies, if required, must be configured in the object-storage service; they are not etcd-backup configuration fields.

**Evidence:** `internal/config/v2/readiness.go` and `internal/gitops/templates/cluster-apps-base/services/etcd-backup/cronjob.yaml`

### 3. Apply Configuration

Render and apply the configuration:

```bash
# Render configuration
opencenter cluster generate my-cluster

# Commit to Git
cd ~/my-cluster-gitops
git add .
git commit -m "Enable etcd backup"
git push

# FluxCD will reconcile automatically (5-15 minutes)
# Or force reconciliation:
flux reconcile kustomization etcd-backup-base
```

### 4. Verify etcd Backup

Verify etcd backup is running:

```bash
# Check the generated CronJob
kubectl get cronjob -n kube-system etcd-backup
# Expected schedule: 0 1 * * *

# Check recent backups in the configured S3-compatible endpoint
aws s3 ls s3://my-cluster-etcd-backups/ \
  --endpoint-url https://s3.example.com

# Expected object names include:
# etcd-snapshot-2026-02-17.db
# etcd-snapshot-2026-02-16.db
```

### 5. Test etcd Backup

Trigger a manual backup job:

```bash
kubectl create job --from=cronjob/etcd-backup etcd-backup-manual -n kube-system
kubectl logs -n kube-system job/etcd-backup-manual -f

# Verify the uploaded snapshot
aws s3 ls s3://my-cluster-etcd-backups/ \
  --endpoint-url https://s3.example.com
```

## Part 2: Configure Velero Backup

### 1. Enable Velero Service

Enable Velero in cluster configuration:

```bash
opencenter cluster edit my-cluster
```

```yaml
opencenter:
  services:
    velero:
      enabled: true

      # S3 configuration
      s3_bucket: "my-cluster-velero-backups"
      s3_region: "us-east-1"
      s3_endpoint: "s3.amazonaws.com"

      # Backup schedule
      backup_schedule: "0 3 * * *"  # Daily at 3 AM

      # Retention policy
      retention_days: 30

      # Volume snapshot location (provider-specific)
      volume_snapshot_location:
        provider: aws  # or openstack, vsphere
        config:
          region: us-east-1
```

**Evidence:** `internal/config/defaults.go:371-376` velero service

### 2. Apply Configuration

```bash
# Render configuration
opencenter cluster generate my-cluster

# Commit to Git
cd ~/my-cluster-gitops
git add .
git commit -m "Enable Velero backup"
git push

# FluxCD will reconcile
flux reconcile kustomization velero-base
```

### 3. Verify Velero Installation

```bash
# Check Velero pods
kubectl get pods -n velero

# Expected output:
# NAME                      READY   STATUS    RESTARTS   AGE
# velero-7d9c4c9f9d-abcde   1/1     Running   0          5m

# Check Velero backup location
velero backup-location get

# Expected output:
# NAME      PROVIDER   BUCKET/PREFIX                PHASE       LAST VALIDATED
# default   aws        my-cluster-velero-backups    Available   2026-02-17 10:00:00
```

### 4. Create Backup Schedule

Create automated backup schedule:

```bash
# Create daily backup schedule
velero schedule create daily-backup \
  --schedule="0 3 * * *" \
  --ttl 720h0m0s  # 30 days retention

# Create weekly full backup
velero schedule create weekly-full-backup \
  --schedule="0 1 * * 0" \
  --ttl 2160h0m0s  # 90 days retention

# List schedules
velero schedule get
```

### 5. Create Manual Backup

Create manual backup for testing:

```bash
# Backup entire cluster
velero backup create manual-backup-$(date +%Y%m%d)

# Backup specific namespace
velero backup create app-backup \
  --include-namespaces my-app

# Backup with volume snapshots
velero backup create full-backup \
  --snapshot-volumes=true

# Watch backup progress
velero backup describe manual-backup-20260217 --details

# Check backup status
velero backup get
```

## Part 3: Restore from Backup

### Scenario 1: Restore etcd (Cluster State)

**Use case:** Cluster state corrupted, need to restore API objects

**Steps:**

```bash
# 1. Stop Kubernetes API server (on all control plane nodes)
ssh ubuntu@<control-plane-1>
sudo systemctl stop kube-apiserver

# 2. Download an etcd backup from the configured S3-compatible endpoint
aws s3 cp s3://my-cluster-etcd-backups/etcd-snapshot-2026-02-17.db /tmp/ \
  --endpoint-url https://s3.example.com

# 3. Restore etcd snapshot
sudo ETCDCTL_API=3 etcdctl snapshot restore /tmp/etcd-snapshot-2026-02-17.db \
  --data-dir=/var/lib/etcd-restore \
  --name=<node-name> \
  --initial-cluster=<cluster-config> \
  --initial-advertise-peer-urls=<peer-url>

# 4. Replace etcd data directory
sudo systemctl stop etcd
sudo mv /var/lib/etcd /var/lib/etcd.backup
sudo mv /var/lib/etcd-restore /var/lib/etcd
sudo systemctl start etcd

# 5. Start Kubernetes API server
sudo systemctl start kube-apiserver

# 6. Verify cluster state
kubectl get nodes
kubectl get pods -A
```

**⚠️ WARNING**\
etcd restore is a destructive operation. Test in non-production first.

### Scenario 2: Restore Application (Velero)

**Use case:** Application deleted, need to restore resources and data

**Steps:**

```bash
# 1. List available backups
velero backup get

# 2. Restore from backup
velero restore create --from-backup manual-backup-20260217

# 3. Watch restore progress
velero restore describe manual-backup-20260217-restore --details

# 4. Verify restored resources
kubectl get all -n my-app

# 5. Verify persistent volumes
kubectl get pvc -n my-app
kubectl get pv
```

### Scenario 3: Restore Specific Namespace

**Use case:** Single namespace deleted or corrupted

**Steps:**

```bash
# Restore specific namespace
velero restore create app-restore \
  --from-backup daily-backup-20260217 \
  --include-namespaces my-app

# Verify restore
velero restore describe app-restore
kubectl get all -n my-app
```

### Scenario 4: Restore to Different Cluster

**Use case:** Migrate application to new cluster

**Steps:**

```bash
# 1. Install Velero on target cluster with same S3 configuration
# (Already done if using openCenter)

# 2. Verify backup location
velero backup-location get

# 3. List backups from source cluster
velero backup get

# 4. Restore to target cluster
velero restore create migration-restore \
  --from-backup daily-backup-20260217

# 5. Verify resources in target cluster
kubectl get all -A
```

## Verification

Verify backup and restore capabilities:

```bash
# 1. Verify etcd backup schedule
kubectl get cronjob -n kube-system etcd-backup

# 2. Verify recent etcd backups
aws s3 ls s3://my-cluster-etcd-backups/ \
  --endpoint-url https://s3.example.com | tail -5

# 3. Verify Velero installation
velero version

# 4. Verify Velero backup location
velero backup-location get

# 5. Verify Velero schedules
velero schedule get

# 6. Verify recent Velero backups
velero backup get

# 7. Test restore (in dev/staging)
velero restore create test-restore --from-backup <backup-name>
```

## Troubleshooting

### etcd Backup Fails

**Symptom:** etcd backup CronJob fails

**Diagnosis:**

```bash
# Check CronJob logs
kubectl logs -n kube-system job/etcd-backup-<timestamp>

# Common errors:
# - S3 authentication failed
# - S3 bucket doesn't exist
# - Insufficient permissions
```

**Solution:**

```bash
# Verify the configured endpoint, bucket, and credentials
aws s3 ls s3://my-cluster-etcd-backups/ \
  --endpoint-url https://s3.example.com

# Create the bucket if missing (using the storage provider's credentials)
aws s3 mb s3://my-cluster-etcd-backups \
  --endpoint-url https://s3.example.com

# Update service-specific credentials and endpoint in configuration
opencenter cluster edit my-cluster
```

### Velero Backup Fails

**Symptom:** Velero backup stuck in InProgress or Failed

**Diagnosis:**

```bash
# Check backup status
velero backup describe <backup-name> --details

# Check Velero logs
kubectl logs -n velero deployment/velero
```

**Common causes:**

1. **S3 authentication failed:** Invalid credentials
2. **Volume snapshot failed:** Provider plugin not configured
3. **Resource too large:** Backup timeout

**Solution:**

```bash
# Fix S3 credentials
kubectl edit secret -n velero cloud-credentials

# Install volume snapshot plugin
velero plugin add velero/velero-plugin-for-aws:v1.9.0

# Increase backup timeout
velero backup create large-backup --timeout 2h
```

### Restore Fails

**Symptom:** Velero restore fails or incomplete

**Diagnosis:**

```bash
# Check restore status
velero restore describe <restore-name> --details

# Check for errors
velero restore logs <restore-name>
```

**Common causes:**

1. **Resource conflicts:** Resources already exist
2. **PV not available:** Volume snapshots not restored
3. **Namespace not created:** Target namespace missing

**Solution:**

```bash
# Delete conflicting resources
kubectl delete namespace my-app

# Restore with namespace mapping
velero restore create --from-backup <backup> \
  --namespace-mappings old-ns:new-ns

# Restore PVs separately
velero restore create pv-restore \
  --from-backup <backup> \
  --include-resources persistentvolumes,persistentvolumeclaims
```

## Best Practices

1. **Test restores regularly:** Verify backups are restorable (monthly)
2. **Multiple backup locations:** Use different S3 buckets/regions for redundancy
3. **Separate etcd and Velero backups:** Different schedules and storage policies
4. **Monitor backup status:** Alert on backup failures
5. **Document restore procedures:** Step-by-step runbooks
6. **Protect backups:** Apply encryption and retention policies in the object-storage service; etcd-backup does not configure these features
7. **Offsite backups:** Store backups in different region/provider
8. **Backup before changes:** Manual backup before major changes

## Backup Schedule Recommendations

**Development:**

* etcd: Nightly at 01:00; configure any retention policy in object storage
* Velero: Daily, 7-day retention

**Staging:**

* etcd: Nightly at 01:00; configure any retention policy in object storage
* Velero: Daily, 14-day retention

**Production:**

* etcd: Nightly at 01:00; configure any retention policy in object storage
* Velero: Daily, 90-day retention
* Velero weekly: Weekly, 1-year retention

## Related Topics

* [Upgrade Kubernetes](upgrade-kubernetes.md) - Backup before upgrades
* [Migrate Clusters](migrate-clusters.md) - Use backups for migration
* [Troubleshoot Deployment](troubleshoot-deployment.md) - Restore from backup
* [Platform Services](../reference/platform-services.md) - etcd-backup and Velero configuration

---

## Evidence

This guide is based on:

* etcd backup service: `internal/config/v2/readiness.go` and generated etcd-backup templates
* Velero service: `internal/config/defaults.go:371-376`
* Backup configuration: Ecosystem.md infrastructure services
* Velero documentation: Official Velero docs
* etcd restore: Kubernetes etcd documentation