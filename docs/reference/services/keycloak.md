---
id: service-keycloak
title: "Keycloak"
sidebar_label: Keycloak
description: Identity and access management service configuration, defaults, secrets, and enforced dependencies.
doc_type: reference
audience: "platform engineers, operators"
tags: [keycloak, identity, oidc, authentication, services]
---

> **Purpose:** For platform engineers and operators, documents Keycloak's full configuration surface, secrets, enforced dependencies, and rendering.

## Overview

Keycloak provides OIDC identity and access management for the cluster.

## Configuration

```yaml
opencenter:
  services:
    keycloak:
      enabled: true                        # default: true
      namespace: keycloak                  # default: keycloak
      hostname:                            # default: auth.<cluster_fqdn>
      frontend_url:
      realm:
      client_id: opencenter                # default: opencenter
      realm_import_enabled: true           # default: true
      realm_groups: []
      realm_admin_email:
      start_optimized: false               # default: false
      cache_enabled: true                  # default: true
      cache_stack: kubernetes               # default: kubernetes (kubernetes | ispn)
      resource_requests_cpu: 500m           # default: 500m
      resource_requests_memory: 1250M       # default: 1250M
      resource_limits_cpu: "2"              # default: 2
      resource_limits_memory: 2250M         # default: 2250M
      instances: 3                          # default: 3
      min_replicas: 3                       # default: 3
      max_replicas: 10                      # default: 10
      database_host:
      database_port: 5432                   # default: 5432
      database_name:
      database_user:
      db_pool_min_size: 30                   # default: 30
      db_pool_initial_size: 30               # default: 30
      db_pool_max_size: 30                   # default: 30
      metrics_enabled: true                  # default: true
      event_metrics_enabled: true            # default: true
      health_enabled: true                   # default: true
      log_level: INFO                        # default: INFO (INFO | DEBUG | WARN | ERROR | TRACE)
      log_format: json                       # default: json (default | json)
      tls_secret_name: keycloak-tls-secret    # default: keycloak-tls-secret
      tls_enabled: true                       # default: true
      backup_enabled: true                    # default: true
      backup_schedule: "0 2 * * *"            # default: 0 2 * * *
      smtp_host:
      smtp_port: 587                          # default: 587
      smtp_from:
      smtp_starttls: true                     # default: true
```

### Validation

Enforced both by `internal/services/plugins/validators.go`'s dead-code validator (documentation of intent only — see [Platform services architecture](../platform-services.md)) and, for the fields that matter operationally, by the plugin's own `validate()` and by `cmd/cluster_service.go`:

- `start_optimized: true` requires `instances >= 2`.
- `min_replicas` must not exceed `max_replicas`.
- `db_pool_min_size` must not exceed `db_pool_max_size`.
- `frontend_url`, if set, must start with `http://` or `https://`.
- `secrets.keycloak.admin_password` is required when enabling via the CLI.

## Secrets

```yaml
secrets:
  keycloak:
    admin_password:      # required
    client_secret:        # OIDC client secret
```

## Dependencies

`internal/config/services/dependency_validator.go` enforces: **`keycloak` requires `olm` and `postgres-operator` to be enabled.** This is checked by `opencenter cluster service enable|disable`.

## Rendering

Keycloak has a dedicated descriptor (`internal/services/descriptors/data/service-keycloak.yaml`, `service: keycloak`) covering the `services/keycloak` template root, with two conditional files: the Keycloak backup CronJob (rendered when `backup_enabled: true`) and the HPA manifest (rendered when `max_replicas` is set). It aggregates into `services-fluxcd-aggregate` and `services-sources-aggregate`.

## CLI commands

```bash
opencenter cluster service enable keycloak --secret="admin_password=..."
opencenter cluster service disable keycloak
opencenter cluster service status
opencenter cluster service options keycloak
```
