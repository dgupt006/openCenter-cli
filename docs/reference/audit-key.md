---
id: audit-key
title: "Audit Signing Key"
sidebar_label: Audit Signing Key
description: The HMAC-SHA256 signing key that protects the openCenter audit log's integrity -- generation, storage, and verification, verified against internal/security/audit_logger.go.
doc_type: reference
audience: "operators, security engineers"
tags: [audit, security, hmac, integrity, signing-key]
---
# Audit Signing Key

**Purpose:** For operators and security engineers, documents the HMAC signing key that protects openCenter's audit log integrity: how it's generated, where it's stored, how it's used, and how to verify a log against it.

## What gets audited

`internal/security/audit_logger.go`'s `AuditLogger` is wired up via `internal/di/providers.go` and `cmd/secrets_helpers.go`, and is actually called from the secrets subsystem (`internal/secrets/manager.go`, `internal/secrets/rotation.go`, `internal/secrets/revocation.go`). Logged event types include: `key.generated`, `key.accessed`, `key.rotated`, `key.revoked`, `key.expired`, `validation.failed`, `input.rejected`, `template.validation.failed`, `secrets.sync`, `secrets.drift_detected`, `secrets.validated`, `secret.decrypted`. This is not a general command-audit trail -- it covers key lifecycle and secrets-sync/validation/drift events, not every CLI invocation.

## Default locations

| What | Path | Function |
| --- | --- | --- |
| Audit log | `<state-dir>/audit/audit.log` | `GetDefaultAuditLogPath()` |
| Signing key | `<config-dir>/audit/audit.key` | `GetDefaultAuditSigningKeyPath()` |

(`<state-dir>` and `<config-dir>` follow the normal [Configuration Precedence](configuration-precedence.md) / [File Locations](file-locations.md) resolution -- `OPENCENTER_STATE_DIR`/`OPENCENTER_CONFIG_DIR` override them.)

## Key generation and storage

* The signing key is exactly **32 random bytes** (`crypto/rand`).
* `loadOrCreateSigningKey(path)` reads the key file if it exists (rejecting it if its length isn't exactly 32 bytes); if the file doesn't exist, it generates a new 32-byte key, creates the parent directory (`0700`), and writes the key to disk with mode **`0600`**.
* This happens automatically the first time an `AuditLogger` is constructed via `NewDefaultAuditLogger()` -- there is no separate `opencenter` subcommand to pre-generate it. If the key file is lost, a new one is silently generated on next use, and previously-written log entries can no longer be verified against the new key (see Verification below).
* A caller can also supply an in-memory key directly (`AuditLoggerConfig.SigningKey`) instead of a key file, bypassing disk storage entirely -- not the default path, but available to embedders of the `security` package.

## How signing works

Each `AuditEvent` (`id`, `timestamp`, `event_type`, `actor`, `resource`, `action`, `result`, `details`, `correlation_id`, `signature`) is signed with **HMAC-SHA256** over a pipe-joined string of exactly six fields:

```
timestamp | event_type | actor | resource | action | result
```

The hex-encoded HMAC is stored in the event's own `signature` field, and the whole event (including its signature) is appended to the log file as one JSON object per line. Note that `details` is **not** part of the signed data -- tampering with `details` after the fact would not be caught by signature verification. Sensitive values inside `details` are masked before signing/writing: known-sensitive field names (`password`, `secret`, `token`, `api_key`, `private_key`, `age_key`, `aws_secret_access_key`, `application_credential_secret`, `bearer`, `authorization`, and similar) are replaced with `***MASKED***`; everything else goes through pattern-based masking via the credential masker.

## Verification

```go
logger.VerifyIntegrity()          // checks every line in the log file
logger.QueryEvents(ctx, filter)   // also verifies each event's signature while querying
```

Both re-derive the HMAC from the same six fields and compare with `hmac.Equal`. A mismatch is reported (to stderr, and as a nonzero invalid-event count from `VerifyIntegrity`) but does not stop processing of the remaining log -- a single tampered or re-keyed entry does not hide the rest of the log from inspection.

There is no dedicated `opencenter` CLI subcommand exposed for running `VerifyIntegrity`/`QueryEvents` from the command line as of this writing -- these are library methods on `*security.AuditLogger`, currently reached only through code that constructs one (e.g. the secrets subsystem). If you need to inspect or verify the log, read `internal/security/audit_logger_test.go` and `audit_logger_query_test.go` for example call patterns, or use `QueryEventsSince`/`ExportEventsToJSON` from Go code.

## Operational notes

* **Rotation:** the log file rotates automatically once it reaches 100MB (`MaxLogSize`), renaming the current file with a timestamp suffix and starting a fresh one.
* **Retention:** rotated log files older than 30 days (`LogRetentionDays`) are deleted automatically on the next rotation.
* **Key rotation:** there is no automated *signing-key* rotation. Replacing `audit.key` invalidates verification of every previously-written entry (their signatures were computed with the old key) -- if you need to rotate the signing key, archive/verify the existing log first.
* **Backup:** back up `audit.key` if you need long-term ability to re-verify historical logs; losing it does not lose the log contents (still readable/queryable), only the ability to cryptographically confirm they weren't altered.

## Cross-reference

* [File Locations](file-locations.md) -- how `<config-dir>`/`<state-dir>` are resolved.
* [Overlay Rendering Security Policy](../contributing/overlay-security-policy.md) -- the separate audit-trail discussion for GitOps overlay rendering (Git history, not this HMAC log).
