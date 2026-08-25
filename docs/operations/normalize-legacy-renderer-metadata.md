---
id: normalize-legacy-renderer-metadata
title: "Normalize Legacy v2 Renderer Metadata"
sidebar_label: Normalize Legacy Renderer Metadata
description: How to remove legacy v2 renderer metadata from a cluster configuration before validation and generation.
doc_type: how-to
audience: "platform engineers, operators"
tags: [configuration, normalization, migration, rendering]
---
# Normalize Legacy v2 Renderer Metadata

**Purpose:** For operators maintaining older v2 cluster files, explains how to
remove internal renderer metadata while preserving supported typed service
configuration.

Older v2 files may contain renderer selection, topology, or raw override keys
that are no longer public configuration. The v2 load/normalize path removes
those keys; the supported replacement is a typed service field or a file under
the service overlay's user-owned `custom/` directory.

## Procedure

1. Run a dry run to review the normalization summary. The command reports the
   file path, current and normalized byte counts, approximate added bytes, and
   whether a backup would be created; it does not print a YAML diff or the
   proposed normalized content:

   ```bash
   opencenter cluster normalize my-cluster --dry-run
   ```

2. Normalize the file. The command creates a timestamped backup before writing:

   ```bash
   opencenter cluster normalize my-cluster
   ```

   Use `org/my-cluster` when the cluster identifier requires an organization
   prefix. Normalization may also add missing default fields; existing supported
   values are preserved.

3. Validate the normalized configuration:

   ```bash
   opencenter cluster validate my-cluster
   ```

4. Regenerate the GitOps output and review the resulting diff:

   ```bash
   opencenter cluster generate my-cluster --dry-run
   ```

## After normalization

Do not re-add `renderer`, `single_stage`, `base_only`, `source_name`,
`override_values_renderer`, `overlay_files_renderer`, or raw `override_values`
keys. Use the service's documented typed fields for supported settings. Put
additional hand-authored manifests or values in `custom/`; do not edit
renderer-owned generated files.

If normalization produces an unexpected result, compare the file with its
`.backup.<timestamp>` copy, correct only supported fields, and repeat validation.
See [Customize Services](customize-services.md) for the supported customization
boundary and [Configuration Schema](../reference/configuration-schema.md) for
service fields.
