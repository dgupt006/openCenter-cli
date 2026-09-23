---
id: service-gateway
title: "Gateway"
sidebar_label: Gateway
description: Envoy Gateway implementation configuration fields, listeners, and defaults.
doc_type: reference
audience: "platform engineers, operators"
tags: [networking, gateway, envoy, routing, tls, services]
---

> **Purpose:** For platform engineers and operators, documents the Envoy Gateway service configuration, covering listeners and defaults.

## Overview

`gateway` deploys an Envoy-based `Gateway` resource implementing the Kubernetes Gateway API, consuming the CRDs installed by [gateway-api](gateway-api.md).

## Configuration

```yaml
opencenter:
  services:
    gateway:
      enabled: true                        # default: true
      namespace: gateway                   # default: gateway
      gateway_name: rmpk-gateway           # default: rmpk-gateway
      gateway_namespace: rackspace-system   # default: rackspace-system
      gateway_class: eg                     # default: eg
      default_issuer: ""
      listeners:
        - name: https
          port: 443
          protocol: HTTPS                  # HTTP | HTTPS
          hostname: "*.example.com"
          tls_secret_name: wildcard-tls
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Whether the Gateway resource is deployed |
| `namespace` | string | `gateway` | Namespace for the Gateway service records |
| `gateway_name` | string | `rmpk-gateway` | Name of the generated `Gateway` resource (`GatewayConfig.GatewayName`) |
| `gateway_namespace` | string | `rackspace-system` | Namespace the `Gateway` resource is created in |
| `gateway_class` | string | `eg` | `GatewayClass` name |
| `default_issuer` | string | — | Default cert-manager `ClusterIssuer` for listener TLS |
| `listeners` | list of `GatewayListener` | `[]` | Listener definitions |
| `listeners[].name` | string | required | Listener identifier |
| `listeners[].port` | int | required | Port number |
| `listeners[].protocol` | string | required | `HTTP` or `HTTPS` |
| `listeners[].hostname` | string | — | Hostname pattern for the listener |
| `listeners[].tls_secret_name` | string | — | TLS secret name (HTTPS listeners) |

## Bring-your-own TLS

By default the generated platform Gateway carries a `cert-manager.io/cluster-issuer`
annotation and every HTTPS listener references a cert-manager-managed leaf Secret
(`keycloak-tls`, `harbor-tls`, `longhorn-tls`, …). Operators who manage TLS
themselves — a wildcard certificate, or pre-existing per-listener Secrets — can
override this from cluster config instead of hand-editing the Gateway on-cluster
(which drifts back on the next reconcile).

Setting **any** bring-your-own option removes the `cert-manager.io/cluster-issuer`
annotation from the Gateway so cert-manager no longer manages those leaves.

```yaml
opencenter:
  services:
    gateway:
      enabled: true
      tls:
        # A single pre-existing Secret (e.g. a wildcard cert) for every HTTPS listener.
        wildcard_secret_name: star-rax-io-tls
        # Optional namespace of the referenced Secret(s).
        secret_namespace: rackspace-system
        # Override specific listeners by logical name; wins over the wildcard.
        per_listener_secrets:
          harbor: harbor-byo-tls
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `tls.wildcard_secret_name` | string | — | One pre-existing TLS Secret used by every HTTPS listener; drops the cert-manager annotation |
| `tls.secret_namespace` | string | — | Namespace of the pre-existing Secret(s); empty means same-namespace lookup |
| `tls.per_listener_secrets` | map | — | Override the TLS Secret for specific listeners by logical name (`keycloak`, `gitops`, `headlamp`, `prometheus`, `alertmanager`, `grafana`, `harbor`, `longhorn`) |

Precedence for each HTTPS listener: `per_listener_secrets[<listener>]`, then
`wildcard_secret_name`, then the built-in `<service>-tls` default. When no `tls`
block is configured the output is unchanged.

## Dependencies

None enforced by `opencenter cluster service enable|disable`, though `gateway` is functionally dependent on `gateway-api`'s CRDs being present. The render catalog lists `envoy-gateway-api-base` as an extra rendering-order dependency for `gateway`.

## Rendering

`gateway` has no dedicated YAML descriptor; it is rendered as a single-stage entry in the built-in render catalog (`internal/gitops/render_catalog.go`), which generates `namespace.yaml`, `gateway-class.yaml`, `gateway.yaml`, and `envoy-proxy-config.yaml` from a fixed Kustomization content template rather than Helm values.

## CLI commands

```bash
opencenter cluster service enable gateway
opencenter cluster service disable gateway
opencenter cluster service status
opencenter cluster service options gateway
```
