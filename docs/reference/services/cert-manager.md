---
id: service-cert-manager
title: "cert-manager"
sidebar_label: cert-manager
description: Automated TLS certificate management with ACME/self-signed/CA issuers across multiple DNS providers.
doc_type: reference
audience: "platform engineers, operators"
tags: [cert-manager, tls, certificates, acme, letsencrypt, services]
---

> **Purpose:** For platform engineers and operators, documents cert-manager's configuration surface, secrets, dependencies, and rendering.

## Overview

cert-manager automates TLS certificate provisioning and renewal using ACME (Let's Encrypt), self-signed, or CA-based issuers, and supports DNS-01 challenges across multiple DNS providers.

## Configuration

```yaml
opencenter:
  services:
    cert-manager:
      enabled: true                     # default: true
      namespace: cert-manager           # default: cert-manager
      email:                            # required by the CLI when enabling
      letsencrypt_server: https://acme-v02.api.letsencrypt.org/directory
      region:
      dns_zones: []
      create_cluster_issuer: true
      dns_provider:                      # route53 | designate | cloudflare | clouddns | azuredns
      issuers:
        - name: letsencrypt-prod
          type: letsencrypt              # letsencrypt | selfsigned | ca
          server: https://acme-v02.api.letsencrypt.org/directory
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Whether cert-manager is deployed |
| `namespace` | string | `cert-manager` | Namespace for cert-manager resources |
| `email` | string | — | ACME registration contact email; required by `opencenter cluster service enable cert-manager` |
| `letsencrypt_server` | string | `https://acme-v02.api.letsencrypt.org/directory` | ACME directory URL |
| `region` | string | — | Cloud region for DNS provider API calls |
| `dns_zones` | list of strings | — | DNS zones managed for certificate validation |
| `create_cluster_issuer` | bool | `true` | Create the default `ClusterIssuer` |
| `dns_provider` | string | — | `route53` \| `designate` \| `cloudflare` \| `clouddns` \| `azuredns` |
| `issuers` | list of `CertIssuer` | — | Additional issuers |
| `issuers[].name` | string | required | Issuer name |
| `issuers[].type` | string | required | `letsencrypt` \| `selfsigned` \| `ca` |
| `issuers[].server` | string | — | ACME server URL (`letsencrypt` type) |

### Validation

- `email` is required when enabling via the CLI (`cmd/cluster_service.go`).
- `letsencrypt_server`, if set, must start with `https://`.
- `email`, if set, must contain `@`.

## Secrets

`schema/opencenter-v2.schema.json` defines `secrets.cert_manager` with a multi-credential shape:

```yaml
secrets:
  cert_manager:
    aws_access_key:                     # legacy flat field
    aws_secret_access_key:
    cloudflare_api_token:                # legacy flat field
    aws:
      <credential-name>:
        enabled: true
        aws_access_key:
        aws_secret_access_key:
        region:
        dns_zones: []
    cloudflare:
      <credential-name>:
        enabled: true
        api_token:
        dns_zones: []
```

## Dependencies

None enforced by `opencenter cluster service enable|disable`.

## Rendering

cert-manager has a dedicated descriptor (`internal/services/descriptors/data/service-cert-manager.yaml`, `service: cert-manager`) that owns everything under the `services/cert-manager` template root plus its Flux source/Kustomization files, and aggregates into `services-fluxcd-aggregate` and `services-sources-aggregate`.

## CLI commands

```bash
opencenter cluster service enable cert-manager --param="email=admin@example.com"
opencenter cluster service disable cert-manager
opencenter cluster service status
opencenter cluster service options cert-manager
```
