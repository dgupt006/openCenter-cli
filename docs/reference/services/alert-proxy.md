---
id: service-alert-proxy
title: "Alert Proxy"
sidebar_label: Alert Proxy
description: Alert forwarding proxy configuration, secrets, and rendering as a managed service.
doc_type: reference
audience: "platform engineers, operators"
tags: [alerting, monitoring, proxy, managed-service, services]
---

> **Purpose:** For platform engineers, documents the alert proxy service, which is designed to run as a managed service.

## Overview

The alert proxy forwards Alertmanager webhook payloads to an external alert-management system. Its descriptor (`internal/services/descriptors/data/service-alert-proxy.yaml`) declares it under `managed_service: alert-proxy`, so it is intended for `opencenter.managed_services.alert-proxy` (enable with `--managed`), though the schema also defines an identical `opencenter.services.alert-proxy` shape. It has no default entry in generated configuration (`internal/config/v2/defaults.go`) — it is opt-in.

## Configuration

```yaml
opencenter:
  managed_services:
    alert-proxy:
      enabled: true
      namespace:
      alert_manager_base_url: http://alertmanager.monitoring.svc:9093
      http_route_fqdn: alerts.example.com
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | — (opt-in) | Whether the alert proxy is deployed |
| `alert_manager_base_url` | string | — | Alertmanager service base URL (`AlertProxyConfig.AlertManagerBaseURL`) |
| `http_route_fqdn` | string | — | External FQDN for the alert proxy's HTTPRoute |

## Secrets

```yaml
secrets:
  alert_proxy:
    core_device_id:
    account_service_token:
    core_account_number:
```

## Dependencies

None enforced by `opencenter cluster service enable|disable`.

## Rendering

`alert-proxy` has a dedicated descriptor (`service-alert-proxy.yaml`, `managed_service: alert-proxy`) that owns everything under the `managed-services/alert-proxy` template root plus its Flux source and Kustomization files, and aggregates into `managed-services-fluxcd-aggregate` and `managed-services-sources-aggregate`.

## CLI commands

```bash
opencenter cluster service enable alert-proxy --managed --secret="core_device_id=..." --secret="account_service_token=..." --secret="core_account_number=..."
opencenter cluster service disable alert-proxy --managed
opencenter cluster service status
opencenter cluster service options alert-proxy --managed
```
