# OpenStack Minimal Default Profile Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make newly generated OpenStack configuration match the documented minimal deployment profile exactly, with identity OIDC disabled, while leaving Kind, bare-metal, and VMware behavior unchanged.

**Architecture:** Keep the shared service definitions, but make their initial enabled flags aware of the already-canonicalized infrastructure provider. Only OpenStack suppresses the non-minimal services; all other providers retain their current values. Set identity OIDC from the same provider decision, and leave `applyProviderBehaviorDefaults` unchanged so the existing OpenStack Cinder and VMware vSphere overrides remain intact.

**Tech Stack:** Go 1.26, table-driven Go tests, YAML-backed v2 configuration, Markdown reference documentation.

## Global Constraints

- Change default behavior only when `infrastructureProvider` canonicalizes to `openstack`.
- Do not change the Kind provider branch, Kind service defaults, or Kind test expectations.
- Do not change bare-metal or VMware service and OIDC defaults.
- Preserve the OpenStack Cinder CSI override and the VMware vSphere CSI override in `applyProviderBehaviorDefaults`.
- For OpenStack, the complete enabled service set must be exactly:

  ```text
  calico
  cert-manager
  fluxcd
  gateway
  gateway-api
  kyverno
  openstack-ccm
  openstack-csi
  sources
  ```

- For OpenStack, every other entry in `opencenter.services` must default to disabled.
- Disable `opencenter.identity.oidc.enabled` only for OpenStack; retain `true` for Kind, bare-metal, and VMware.
- Do not alter `opencenter.kubernetes.oidc.enabled`; it already defaults to `false` independently of identity OIDC.
- Keep documentation provider-qualified so it does not imply that the OpenStack behavior applies globally.

---

## Task 1: Add an OpenStack-only regression specification

**Files:**

- Modify: `internal/config/v2/defaults_test.go`
- Test: `internal/config/v2/defaults_test.go`

- [ ] **Step 1: Add a helper that returns all enabled service names in stable order**

  Add `slices` to the imports and define a test helper near the provider-default tests:

  ```go
  func enabledServiceNames(t *testing.T, serviceMap ServiceMap) []string {
  	t.Helper()

  	names := make([]string, 0, len(serviceMap))
  	for name, serviceConfig := range serviceMap {
  		enabledConfig, ok := serviceConfig.(interface{ IsEnabled() bool })
  		if !ok {
  			t.Fatalf("service %q does not expose IsEnabled", name)
  		}
  		if enabledConfig.IsEnabled() {
  			names = append(names, name)
  		}
  	}

  	slices.Sort(names)
  	return names
  }
  ```

  This tests the public behavior shared by every heterogeneous service config without adding a production setter or widening the production interface.

- [ ] **Step 2: Extend `TestNewV2DefaultProviders` with provider-specific OIDC expectations**

  Add these fields to the table:

  ```go
  expectIdentityOIDC    bool
  expectEnabledServices []string
  ```

  Set `expectIdentityOIDC: false` only on the OpenStack row. Set it to `true` explicitly on the Kind, bare-metal, and VMware rows so the regression test proves that OIDC behavior did not change for them.

- [ ] **Step 3: Encode the exact OpenStack service allowlist**

  On the OpenStack row, set:

  ```go
  expectEnabledServices: []string{
  	"calico",
  	"cert-manager",
  	"fluxcd",
  	"gateway",
  	"gateway-api",
  	"kyverno",
  	"openstack-ccm",
  	"openstack-csi",
  	"sources",
  },
  ```

  Leave `expectEnabledServices` nil for Kind, bare-metal, and VMware. This keeps the exact allowlist assertion deliberately scoped to OpenStack instead of redefining other providers in this change.

- [ ] **Step 4: Assert OIDC for every provider and the exact service list for OpenStack**

  Add these assertions inside the existing table subtest, alongside the existing storage and cloud-provider assertions:

  ```go
  if got := cfg.OpenCenter.Identity.OIDC.Enabled; got != tt.expectIdentityOIDC {
  	t.Fatalf("identity OIDC enabled = %t, want %t", got, tt.expectIdentityOIDC)
  }

  if tt.expectEnabledServices != nil {
  	got := enabledServiceNames(t, cfg.OpenCenter.Services)
  	if !slices.Equal(got, tt.expectEnabledServices) {
  		t.Fatalf("enabled services = %v, want %v", got, tt.expectEnabledServices)
  	}
  }
  ```

  Retain the existing assertions for OpenStack Cinder CSI and VMware vSphere CSI; they are regression coverage for the provider overrides that must remain intact.

- [ ] **Step 5: Run the focused test and confirm it fails for the intended reason**

  Run:

  ```bash
  go test ./internal/config/v2 -run '^TestNewV2DefaultProviders$' -count=1
  ```

  Expected: FAIL for the OpenStack row because identity OIDC is currently enabled and the enabled service list still includes non-minimal services. Kind, bare-metal, and VMware OIDC assertions should already pass.

- [ ] **Step 6: Commit the failing regression test**

  ```bash
  git add internal/config/v2/defaults_test.go
  git commit -m "test(config): specify OpenStack minimal defaults"
  ```

---

## Task 2: Apply the minimal profile only to OpenStack defaults

**Files:**

- Modify: `internal/config/v2/defaults.go`
- Modify: `internal/config/v2/default_service_map_types_test.go`
- Test: `internal/config/v2/defaults_test.go`
- Test: `internal/config/v2/default_service_map_types_test.go`

- [ ] **Step 1: Pass the canonical provider into service-map construction**

  In `NewV2Default`, change the service map call from:

  ```go
  Services: defaultServiceMap(clusterFQDN),
  ```

  to:

  ```go
  Services: defaultServiceMap(clusterFQDN, selectedProvider),
  ```

  The provider has already been canonicalized before this point, so aliases resolve consistently without duplicating normalization logic.

- [ ] **Step 2: Make identity OIDC OpenStack-aware**

  Define the provider decision once after `selectedProvider` is canonicalized:

  ```go
  isOpenStack := selectedProvider == "openstack"
  ```

  Then change only identity OIDC construction:

  ```go
  Identity: IdentityConfig{
  	OIDC: IdentityOIDCConfig{
  		Enabled:  !isOpenStack,
  		Source:   OIDCSourceInternal,
  		Provider: OIDCProviderKeycloak,
  	},
  },
  ```

  Do not change the separate Kubernetes OIDC block.

- [ ] **Step 3: Add an OpenStack-only extended-service flag to `defaultServiceMap`**

  Change the function signature and introduce a narrowly named flag:

  ```go
  func defaultServiceMap(clusterFQDN, provider string) ServiceMap {
  	extendedServicesEnabled := provider != "openstack"

  	return ServiceMap{
  		// existing definitions
  	}
  }
  ```

  Use the flag only for services that are currently enabled but are not part of the minimal OpenStack profile. Do not use it on the nine allowlisted services, and do not modify entries that are already disabled.

- [ ] **Step 4: Disable every currently enabled non-minimal service for OpenStack**

  Replace the literal `Enabled: true` with `Enabled: extendedServicesEnabled` for exactly these service entries:

  ```text
  etcd-backup
  external-snapshotter
  headlamp
  keycloak
  postgres-operator
  rbac-manager
  kube-prometheus-stack
  loki
  tempo
  velero
  olm
  ```

  Keep `Enabled: true` for:

  ```text
  calico
  cert-manager
  fluxcd
  gateway
  gateway-api
  kyverno
  openstack-ccm
  openstack-csi
  sources
  ```

  Leave all preexisting `Enabled: false` values unchanged. This makes every service outside the allowlist disabled for OpenStack while returning the exact prior map for every other provider.

- [ ] **Step 5: Update direct test callers for the new function signature**

  In `internal/config/v2/default_service_map_types_test.go`, pass a non-OpenStack provider such as `"kind"` to each direct `defaultServiceMap` call:

  ```go
  serviceMap := defaultServiceMap("cluster.example.com", "kind")
  ```

  These tests verify concrete service config types, not provider behavior. Using Kind preserves their current fixture behavior and must not change their expectations.

- [ ] **Step 6: Leave provider behavior overrides untouched**

  Do not edit `applyProviderBehaviorDefaults`. In particular, retain:

  - OpenStack enabling the Kubernetes Cinder CSI plugin.
  - VMware enabling the Kubernetes vSphere CSI plugin.
  - All existing Kind-specific OLM, PostgreSQL, OpenStack storage, and Velero behavior.

- [ ] **Step 7: Format the changed Go files**

  ```bash
  gofmt -w internal/config/v2/defaults.go internal/config/v2/defaults_test.go internal/config/v2/default_service_map_types_test.go
  ```

- [ ] **Step 8: Run focused regression tests**

  ```bash
  go test ./internal/config/v2 -run '^(TestNewV2DefaultProviders|TestDefaultServiceMap.*)$' -count=1
  ```

  Expected: PASS. Confirm the OpenStack subtest reports only the nine allowlisted services, OIDC is false only there, and the existing storage override assertions still pass.

- [ ] **Step 9: Run the complete v2 config package tests**

  ```bash
  go test ./internal/config/v2 -count=1
  ```

  Expected: PASS with no changes to Kind-specific expectations.

- [ ] **Step 10: Commit the implementation**

  ```bash
  git add internal/config/v2/defaults.go internal/config/v2/default_service_map_types_test.go
  git commit -m "feat(config): minimize OpenStack default services"
  ```

---

## Task 3: Document the provider-qualified defaults

**Files:**

- Modify: `docs/reference/default-values.md`
- Modify: `docs/operations/deployment-profiles.md`

- [ ] **Step 1: Correct the identity OIDC default in the reference table**

  In `docs/reference/default-values.md`, change the `opencenter.identity.oidc.enabled` default from an unconditional `true` to provider-qualified wording:

  ```markdown
  | `opencenter.identity.oidc.enabled` | `false` for OpenStack; `true` for other providers | Enables identity OIDC. |
  ```

  Keep the Kubernetes OIDC row unchanged.

- [ ] **Step 2: Replace the OpenStack enabled-services table with the exact minimal allowlist**

  Under the OpenStack platform service defaults, list only:

  ```text
  calico
  cert-manager
  fluxcd
  gateway
  gateway-api
  kyverno
  openstack-ccm
  openstack-csi
  sources
  ```

  Preserve the existing version/source/type descriptions for those entries where present.

- [ ] **Step 3: Document all services disabled by default on OpenStack**

  Make the disabled section explicitly OpenStack-specific and include both the newly disabled services and entries that were already disabled. At minimum, verify that the following newly disabled names appear there:

  ```text
  etcd-backup
  external-snapshotter
  headlamp
  keycloak
  postgres-operator
  rbac-manager
  kube-prometheus-stack
  loki
  tempo
  velero
  olm
  ```

  Also retain the existing disabled entries such as Harbor, MetalLB, Kafka, vSphere CSI, Weave GitOps, Longhorn, Mimir, OpenTelemetry, Sealed Secrets, and the managed alert proxy so the documented OpenStack list is exhaustive.

- [ ] **Step 4: Add a provider-scope note**

  Add a short note near the platform service tables:

  ```markdown
  These service and identity defaults apply to OpenStack. Kind, bare-metal, and VMware retain their existing provider-specific defaults.
  ```

  If the reference footer names the legacy defaults source, point it to `internal/config/v2/defaults.go` without adding fragile line numbers.

- [ ] **Step 5: Remove the stale enterprise-profile equivalence claim**

  In `docs/operations/deployment-profiles.md`, replace the statement that the enterprise profile matches the openCenter default service set with provider-qualified language such as:

  ```markdown
  For OpenStack, enterprise services are opt-in additions to the minimal default profile.
  ```

  Update the OpenStack provider note to state that generated defaults use the minimal profile. Do not edit the Kind provider note or imply a Kind behavior change. Leave the vSphere provider guidance intact.

- [ ] **Step 6: Review the documentation against the regression allowlist**

  Compare the nine enabled names in the test and both documentation pages character-for-character. Search for stale unqualified default claims:

  ```bash
  rg -n "default service set|Enabled by Default|oidc.enabled|OpenStack" docs/reference/default-values.md docs/operations/deployment-profiles.md
  ```

  Expected: all relevant claims clearly identify OpenStack, and no text says Kind changed.

- [ ] **Step 7: Commit the documentation**

  ```bash
  git add docs/reference/default-values.md docs/operations/deployment-profiles.md
  git commit -m "docs(config): document OpenStack minimal defaults"
  ```

---

## Task 4: Verify scope and repository health

**Files:**

- Verify: `internal/config/v2/defaults.go`
- Verify: `internal/config/v2/defaults_test.go`
- Verify: `internal/config/v2/default_service_map_types_test.go`
- Verify: `docs/reference/default-values.md`
- Verify: `docs/operations/deployment-profiles.md`

- [ ] **Step 1: Run the full test suite**

  ```bash
  go test ./... -count=1
  ```

  Expected: PASS.

- [ ] **Step 2: Check formatting and whitespace**

  ```bash
  git diff --check
  ```

  Expected: no output.

- [ ] **Step 3: Audit the provider boundary**

  Review the final diff and verify all of the following:

  - The only behavioral condition added is for canonical provider `openstack`.
  - The Kind branch in `applyProviderBehaviorDefaults` is byte-for-byte unchanged.
  - Bare-metal and VMware defaults still receive the prior service map and identity OIDC value.
  - OpenStack still enables Cinder CSI through its existing provider override.
  - VMware still enables vSphere CSI through its existing provider override.
  - OpenStack's enabled services equal the nine-name allowlist exactly; no tenth service is enabled.
  - Identity OIDC is false for OpenStack and true for all three other tested providers.

- [ ] **Step 4: Scan changed content for unfinished placeholders**

  ```bash
  git diff --check
  rg -n "TODO|FIXME|TBD|placeholder" internal/config/v2/defaults.go internal/config/v2/defaults_test.go internal/config/v2/default_service_map_types_test.go docs/reference/default-values.md docs/operations/deployment-profiles.md
  ```

  Expected: no newly introduced unfinished content. Existing unrelated matches, if any, are inspected and left untouched.

- [ ] **Step 5: Inspect final scope**

  ```bash
  git status --short
  git diff --stat
  git diff -- internal/config/v2/defaults.go internal/config/v2/defaults_test.go internal/config/v2/default_service_map_types_test.go docs/reference/default-values.md docs/operations/deployment-profiles.md
  ```

  Expected: changes are limited to OpenStack-aware default construction, its regression coverage, and the two documentation pages. No Kind implementation or documentation is modified.

- [ ] **Step 6: Create a final verification commit only if the verification pass required corrections**

  If review found and fixed a scoped issue, stage only the corrected files and use a narrowly descriptive commit. If no correction was needed, do not create an empty commit.
