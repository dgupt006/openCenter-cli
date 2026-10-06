# Kind Local Gitea Robustness Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the local Kind + Gitea deploy path robust against three real-world failures: (1) Gitea cert files not writable when the container user ≠ CLI user, (2) `cluster generate` being refused after `flux bootstrap` drops `gotk-*` files into the overlay, and (3) TLS hostname mismatch when the GitOps URL host is not a cert SAN.

**Architecture:** Three independent, testable fixes. Fix 1 adds a UID-reconciliation step in the Gitea local-dev service so `writeCertificates` never fails on a foreign-owned cert dir. Fix 2 extends the existing ownership exemption so `applications/overlays/<name>/flux-system/**` (where `flux bootstrap` writes `gotk-*`) is treated as deploy-managed, not user-authored. Fix 3 threads the GitOps URL's hostname into the Gitea cert SAN list so the configured host is always TLS-valid, plus a generate-time guard that keeps the `-config` sources consistent.

**Tech Stack:** Go 1.27, `crypto/x509`, `crypto/rsa`, `net/url`, existing `localdev.Executor`/`Layout`/ownership-ledger infrastructure, `testing` (table + round-trip style, no external test framework).

## Global Constraints

- All changes stay inside `internal/localdev/gitea`, `internal/gitops`, and `internal/cluster` (plus their `_test.go` files). No new external dependencies.
- Follow existing code style: `gofmt`, tab indentation, `fmt.Errorf("...: %w", err)` for wrapping, no comments unless they explain non-obvious intent (match surrounding files).
- The Gitea container always runs as UID/GID `1000:1000` (`ensureAdminUser` uses `exec --user 1000:1000`; `app.ini` has `RUN_USER = git`). Treat `1000` as the fixed container UID.
- The overlay `flux-system` dir is written by `flux bootstrap` at `--path=applications/overlays/<cluster>` → `applications/overlays/<cluster>/flux-system/`. The existing exemption only covers `clusters/<cluster>/flux-system/`.
- Tests must run hermetically (no podman/docker, no real cluster). Use `t.TempDir()`, the in-memory/fake executor where the service calls the runtime, and pure functions for cert/ownership logic.
- Commit after every task. Use conventional-commit messages (`fix:`, `feat:`, `test:`).

## Background (what actually broke)

During a real local Kind deploy of cluster `oc-kind`, three failures required manual intervention:

1. `gitea-attach-kind` failed with `write .../gitea/certs/ca.pem: permission denied`. The Gitea data dir was owned by the container user (`devx`, UID 1000) from a prior run; the CLI user (`opencenter`, UID 1002) could not overwrite the existing certs. Fixed by hand with `chown` + `setfacl`.
2. Re-running `cluster generate` after `flux bootstrap` was refused: `refusing to regenerate: user-authored files found in generator-owned paths: applications/overlays/oc-kind/flux-system/gotk-{components,sync}.yaml ...`. `flux bootstrap` writes `gotk-*` there; the ownership ledger does not exempt the overlay `flux-system` (only `clusters/<name>/flux-system`).
3. `flux bootstrap` failed TLS: `certificate is valid for localhost, gitea, not gitea.oc-baremetal`. The cert SANs are `localhost`/`gitea` + IP SANs (127.0.0.1, ::1, Kind IP, host routable IP). An arbitrary `/etc/hosts` hostname in `git_url` is not a SAN. Fixed by hand by switching `git_url` + `origin` to the host IP (a SAN). The two generated `-config` GitRepository sources still held the old hostname and had to be `kubectl patch`ed.

The three tasks below fix each at the source.

---

### Task 1: Make Gitea cert writing robust to a foreign-owned data dir

**Problem:** `Service.writeCertificates` calls `os.WriteFile` on `ca.pem`/`cert.pem`/`key.pem`. On a re-run, those files (and their parent dir) can be owned by the container UID (1000) while the CLI runs as a different UID, so `os.WriteFile` fails with EACCES. `Up()` and `AttachKind()` both call `writeCertificates`.

**Approach (long-term):** Add a `ensureCertDirOwnership` helper that, before writing, verifies the cert dir and its three files are writable by the current process. If a cert file exists but is not writable by the current user (owner or group mismatch), re-own it to the current user via `os.Chown` when possible; otherwise, as the portable fallback, rewrite the file by *replacing* it (remove + create) which only requires directory write permission, not file write permission. Directory write permission is granted because `layout.Ensure()` created it as the CLI user — but if the dir itself is foreign-owned, the CLI user cannot replace files either, so we chown the dir when we have the privilege (root) and, when we do not, return a clear actionable error naming the exact path, owner, and the `chown`/`setfacl` commands to run.

**Files:**
- Modify: `internal/localdev/gitea/service.go` (add `ensureCertDirOwnership`, call it from `writeCertificates`)
- Test: `internal/localdev/gitea/service_test.go` (add ownership tests)

**Interfaces:**
- Consumes: `Service` struct (`s.layout localdev.Layout`, `s.executor localdev.Executor`), `localdev.Layout.{GiteaCertDir, CACertPath, ServerCertPath, ServerKeyPath}`, `os.Stat`, `os.Chown`, `os.Remove`, `os.WriteFile`.
- Produces: `func (s *Service) ensureCertDirOwnership() error` (unexported, tested via `writeCertificates`). No signature change to `writeCertificates` or its callers.

- [ ] **Step 1: Write the failing test**

Add to `internal/localdev/gitea/service_test.go`:

```go
func TestWriteCertificatesReplacesForeignOwnedFile(t *testing.T) {
	service, err := NewService(localdev.NewExecutor(), t.TempDir(), DefaultSettings("podman"))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if err := service.layout.Ensure(); err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}
	// Simulate a foreign-owned cert file that the current user cannot
	// overwrite: create a real file, then remove write permission on the
	// file while keeping the directory writable. os.WriteFile truncates in
	// place, so a non-writable file must fail the naive path.
	if err := os.WriteFile(service.layout.CACertPath, []byte("stale"), 0o444); err != nil {
		t.Fatalf("seed ca.pem: %v", err)
	}

	if err := service.writeCertificates(nil); err != nil {
		t.Fatalf("writeCertificates() error = %v (expected it to replace the read-only file)", err)
	}

	// The file must now hold a fresh, parseable certificate (not "stale").
	data, err := os.ReadFile(service.layout.CACertPath)
	if err != nil {
		t.Fatalf("read ca.pem: %v", err)
	}
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "CERTIFICATE" {
		t.Fatalf("ca.pem is not a certificate: %q", string(data[:min(40, len(data))]))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/localdev/gitea/ -run TestWriteCertificatesReplacesForeignOwnedFile -v`
Expected: FAIL — `writeCertificates() error = write .../ca.pem: open ...: permission denied` (or similar), because the current `writePEMFile` truncates the read-only file in place.

- [ ] **Step 3: Write minimal implementation**

In `internal/localdev/gitea/service.go`, add `ensureCertDirOwnership` and call it at the top of `writeCertificates`. The key behavior: for each cert path, if it exists and is not writable by the current process, remove it first (needs only dir write) so the subsequent `os.WriteFile` creates a fresh file. If the dir itself is not writable and we cannot chown it, return an actionable error.

```go
// ensureCertDirOwnership makes the cert directory and any existing cert files
// writable by the current process. On a re-run the Gitea data dir may be owned
// by the container UID (1000) while the CLI runs as a different UID, so
// os.WriteFile's in-place truncate would fail with EACCES. Removing a
// file requires only directory write permission, so we drop foreign-owned
// files and let writePEMFile recreate them. If the directory itself is not
// writable and cannot be chowned, we return an actionable error.
func (s *Service) ensureCertDirOwnership() error {
	certDir := s.layout.GiteaCertDir
	if err := os.MkdirAll(certDir, 0o755); err != nil {
		return fmt.Errorf("create cert dir %s: %w", certDir, err)
	}
	for _, path := range []string{s.layout.CACertPath, s.layout.ServerCertPath, s.layout.ServerKeyPath} {
		info, err := os.Stat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("stat %s: %w", path, err)
		}
		if !fileWritable(path) {
			// Drop the foreign-owned file so it can be recreated.
			if rmErr := os.Remove(path); rmErr != nil {
				return fmt.Errorf("cert %s is owned by uid %d (gitea container user) and is not writable by this process (uid %d). Remove or chown it, e.g.:\n  sudo chown $(id -u):$(id -g) %s\nthen retry.", path, info.Sys().(*syscall.Stat_t).Uid, currentUID(), path)
			}
		}
	}
	return nil
}

// fileWritable reports whether the current process can open path for writing.
func fileWritable(path string) bool {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

// currentUID returns the effective UID of the calling process. On Linux
// syscall.Getuid takes no arguments; on Windows it returns 0 (the ownership
// reconciliation is a no-op there because the data dir is always CLI-owned).
func currentUID() uint32 {
	return syscall.Getuid()
}
```

Add `"syscall"` to the imports of `service.go`. Then call the helper at the start of `writeCertificates`:

```go
func (s *Service) writeCertificates(extraIPs []string) error {
	if err := s.ensureCertDirOwnership(); err != nil {
		return err
	}
	ips := []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}
	// ...rest unchanged...
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/localdev/gitea/ -run TestWriteCertificatesReplacesForeignOwnedFile -v`
Expected: PASS.

- [ ] **Step 5: Run the package tests + build**

Run: `go test ./internal/localdev/gitea/ -count=1 && go build ./...`
Expected: all pass, clean build.

- [ ] **Step 6: Commit**

```bash
git add internal/localdev/gitea/service.go internal/localdev/gitea/service_test.go
git commit -m "fix: make local gitea cert writing robust to foreign-owned data dir"
```

---

### Task 2: Exempt the overlay flux-system dir from the generate ownership preflight

**Problem:** After `flux bootstrap --path=applications/overlays/<cluster>`, it writes `gotk-components.yaml`, `gotk-sync.yaml`, and `kustomization.yaml` into `applications/overlays/<cluster>/flux-system/`. The ownership preflight in `planRepositoryPromotion` (via `scanLiveRepositoryTree` → `ownershipPathAllowed`) does not exempt the overlay `flux-system` (only `clusters/<cluster>/flux-system`), so a subsequent `cluster generate` refuses with "user-authored files found in generator-owned paths".

**Approach (long-term):** Extend `ownershipPathAllowed` to also exclude `applications/overlays/<cluster>/flux-system/**`, mirroring the existing `clusters/<cluster>/flux-system` exclusion. These files are written by `flux bootstrap` (deploy-managed), not by the generator, so they must not block promotion. `scanLiveRepositoryTree` already calls `ownershipPathAllowed`, so exempting the path there is sufficient; also add it to `isDeployManagedPath`'s companion logic only if the scan uses it — verify in Step 1's test.

**Files:**
- Modify: `internal/gitops/ownership.go:228-246` (`ownershipPathAllowed`)
- Test: `internal/gitops/ownership_test.go` (add a focused test)

**Interfaces:**
- Consumes: `ownershipPathAllowed(path, clusterName string) bool`, the existing `fluxPrefix` pattern for `clusters/<cluster>/flux-system`.
- Produces: same signature; behavior change — overlay `flux-system` paths now return `false` (not generator-owned).

- [ ] **Step 1: Write the failing test**

Add to `internal/gitops/ownership_test.go`:

```go
func TestOwnershipPathAllowedExcludesOverlayFluxSystem(t *testing.T) {
	clusterName := "oc-kind"
	// These are written by `flux bootstrap` at deploy time, not by the
	// generator, so they must be exempt from the ownership preflight.
	exempt := []string{
		filepath.ToSlash(filepath.Join("applications", "overlays", clusterName, "flux-system", "gotk-components.yaml")),
		filepath.ToSlash(filepath.Join("applications", "overlays", clusterName, "flux-system", "gotk-sync.yaml")),
		filepath.ToSlash(filepath.Join("applications", "overlays", clusterName, "flux-system", "kustomization.yaml")),
	}
	for _, p := range exempt {
		if ownershipPathAllowed(p, clusterName) {
			t.Errorf("ownershipPathAllowed(%q) = true; want false (flux bootstrap-managed)", p)
		}
	}
	// A genuine generator-owned file must remain allowed.
	generatorOwned := filepath.ToSlash(filepath.Join("applications", "overlays", clusterName, "services", "calico", "deployment.yaml"))
	if !ownershipPathAllowed(generatorOwned, clusterName) {
		t.Errorf("ownershipPathAllowed(%q) = false; want true (generator-owned)", generatorOwned)
	}
	// The existing clusters/<name>/flux-system exclusion must still hold.
	oldExempt := filepath.ToSlash(filepath.Join("clusters", clusterName, "flux-system", "gotk-sync.yaml"))
	if ownershipPathAllowed(oldExempt, clusterName) {
		t.Errorf("ownershipPathAllowed(%q) = true; want false (pre-existing exclusion regressed)", oldExempt)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/gitops/ -run TestOwnershipPathAllowedExcludesOverlayFluxSystem -v`
Expected: FAIL — the `gotk-*` overlay paths return `true`.

- [ ] **Step 3: Write minimal implementation**

In `internal/gitops/ownership.go`, modify `ownershipPathAllowed` to also exclude the overlay `flux-system`. The current body (lines 228-246) has:

```go
func ownershipPathAllowed(path, clusterName string) bool {
	path = filepath.ToSlash(filepath.Clean(path))
	if globalOwnershipFiles[path] {
		return true
	}
	for _, scope := range repositoryClusterScopes(clusterName) {
		if path == scope || strings.HasPrefix(path, scope+"/") {
			if isGeneratedTreeCustomPath(path) {
				return false
			}
			fluxPrefix := filepath.ToSlash(filepath.Join("clusters", clusterName, "flux-system"))
			if path == fluxPrefix || strings.HasPrefix(path, fluxPrefix+"/") {
				return false
			}
			return true
		}
	}
	return false
}
```

Replace the single `fluxPrefix` check with both the `clusters/<name>/flux-system` and `applications/overlays/<name>/flux-system` prefixes:

```go
func ownershipPathAllowed(path, clusterName string) bool {
	path = filepath.ToSlash(filepath.Clean(path))
	if globalOwnershipFiles[path] {
		return true
	}
	// flux bootstrap writes gotk-components.yaml / gotk-sync.yaml /
	// kustomization.yaml into the cluster's flux-system dir at deploy time
	// (both the base path clusters/<name>/flux-system and the overlay path
	// applications/overlays/<name>/flux-system when --path is the overlay).
	// These are deploy-managed, not generator-owned, so they must be exempt
	// from the ownership preflight or a later generate is refused.
	fluxPrefixes := []string{
		filepath.ToSlash(filepath.Join("clusters", clusterName, "flux-system")),
		filepath.ToSlash(filepath.Join("applications", "overlays", clusterName, "flux-system")),
	}
	for _, scope := range repositoryClusterScopes(clusterName) {
		if path == scope || strings.HasPrefix(path, scope+"/") {
			if isGeneratedTreeCustomPath(path) {
				return false
			}
			for _, prefix := range fluxPrefixes {
				if path == prefix || strings.HasPrefix(path, prefix+"/") {
					return false
				}
			}
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/gitops/ -run TestOwnershipPathAllowedExcludesOverlayFluxSystem -v`
Expected: PASS.

- [ ] **Step 5: Run the full ownership test suite + build (no regression)**

Run: `go test ./internal/gitops/ -count=1 && go build ./...`
Expected: all pass, clean build. (The existing 33 ownership tests must stay green — the new exemption only narrows what is "allowed", and `TestPromoteOverlayRoundTripRender` etc. do not place files under `flux-system`.)

- [ ] **Step 6: Commit**

```bash
git add internal/gitops/ownership.go internal/gitops/ownership_test.go
git commit -m "fix: exempt overlay flux-system (flux bootstrap files) from generate ownership preflight"
```

---

### Task 3: Include the GitOps URL hostname in the Gitea cert SANs

**Problem:** `writeCertificates` builds SANs as `DNS:localhost,gitea` + IP addresses (127.0.0.1, ::1, Kind IP, host routable IP). A `git_url` that uses a custom hostname (e.g. `gitea.oc-baremetal` from `/etc/hosts`) is not a SAN, so `flux bootstrap`'s git clone fails TLS verification. The user must instead use the host IP URL — but the two generated `-config` GitRepository sources (`opencenter-keycloak-config`, `opencenter-olm-config`) are rendered from `gitops.repository.url`, so if the user switches URLs after generate, they go stale and a re-generate is refused (fixed by Task 2, but the stale values still cause a Flux deadlock until patched).

**Approach (long-term, two parts):**

**(3a) Cert SAN from the URL host.** Add an `extraHosts []string` parameter to `writeCertificates` (and thread it from the `Up`/`AttachKind`/`Status` callers that know the configured GitOps URL). The Gitea service does not know the cluster's `git_url` today; `AttachKind` is called from `kind_bootstrap_provider.go` which *does* have the config. Thread the URL host from the provider into `AttachKind` → `writeCertificates`. When the URL host is a valid DNS name (not an IP, not empty), add it to `DNSNames`. This makes the configured hostname cert-valid, so users can keep a friendly `/etc/hosts` name and `flux bootstrap` succeeds.

**(3b) Guard against stale `-config` sources.** After `writeCertificates` in `Up`/`AttachKind`, the cert is valid for both the IP and the URL host. The remaining staleness risk is that the *generated* `-config` sources hardcode whatever `git_url` was at generate time. Fix the root cause: make `sourceAuthBlockCustomerRepository` always render from the *current* `cfg.OpenCenter.GitOps.Repository.URL` (it already does — the bug is that generate was refused post-bootstrap, now fixed by Task 2). With Task 2 in place, re-running `cluster generate` after a URL change is legal and rewrites the `-config` sources. Add a test asserting that a URL change is reflected in a fresh render (round-trip), documenting the contract.

**Files:**
- Modify: `internal/localdev/gitea/service.go` (`writeCertificates` signature + `AttachKind`/`Up` threading)
- Modify: `internal/cluster/kind_bootstrap_provider.go` (pass URL host into `AttachKind`)
- Modify: `internal/gitops/copy.go` (test-only contract; no logic change needed if Task 2 covers it)
- Test: `internal/localdev/gitea/service_test.go`, `internal/gitops/ownership_test.go` or `internal/gitops/copy_test.go`

**Interfaces:**
- Consumes: `writeCertificates(extraIPs []string) error` → becomes `writeCertificates(extraIPs []string, extraHosts []string) error`. `AttachKind(ctx context.Context) (*AttachResult, error)` → `AttachKind(ctx context.Context, extraHosts []string)`. `Up` calls `writeCertificates(nil)` → `writeCertificates(nil, nil)` (no host known at `Up`; `AttachKind` is where the URL host matters).
- Produces: `writeCertificates` adds each `extraHosts` entry (validated DNS name) to the server cert `DNSNames`.

- [ ] **Step 1: Write the failing test for SAN host**

Add to `internal/localdev/gitea/service_test.go`:

```go
func TestWriteCertificatesIncludesURLHostSAN(t *testing.T) {
	service, err := NewService(localdev.NewExecutor(), t.TempDir(), DefaultSettings("podman"))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if err := service.layout.Ensure(); err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}
	if err := service.writeCertificates([]string{"10.89.0.11"}, []string{"gitea.oc-baremetal", "not-an-ip-but-a-name"}); err != nil {
		t.Fatalf("writeCertificates() error = %v", err)
	}
	data, err := os.ReadFile(service.layout.ServerCertPath)
	if err != nil {
		t.Fatalf("read server cert: %v", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		t.Fatal("failed to decode server cert PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("ParseCertificate() error = %v", err)
	}
	found := false
	for _, name := range cert.DNSNames {
		if name == "gitea.oc-baremetal" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected gitea.oc-baremetal in DNS SANs, got %v", cert.DNSNames)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/localdev/gitea/ -run TestWriteCertificatesIncludesURLHostSAN -v`
Expected: FAIL — `writeCertificates` takes 1 arg, not 2 (compile error).

- [ ] **Step 3: Write minimal implementation**

In `internal/localdev/gitea/service.go`:

(a) Change the signature and SAN assembly:

```go
func (s *Service) writeCertificates(extraIPs []string, extraHosts []string) error {
	ips := []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}
	for _, rawIP := range extraIPs {
		if ip := net.ParseIP(strings.TrimSpace(rawIP)); ip != nil {
			ips = append(ips, ip)
		}
	}
	dnsNames := []string{"localhost", "gitea"}
	for _, host := range extraHosts {
		host = strings.TrimSpace(host)
		if host == "" || net.ParseIP(host) != nil {
			continue // skip empties and bare IPs (already IP SANs)
		}
		dnsNames = append(dnsNames, host)
	}
	// ...key generation + caTemplate unchanged...
	serverTemplate := &x509.Certificate{
		// ...unchanged fields...
		DNSNames:  dnsNames,
		IPAddresses: ips,
	}
	// ...rest unchanged...
}
```

(b) Update all existing callers of `writeCertificates` to pass the extra host:
- `Up` (line 151): `writeCertificates(nil, nil)`.
- `AttachKind` (lines 296, 312): change signature to accept hosts and pass them through.

```go
func (s *Service) AttachKind(ctx context.Context, extraHosts []string) (*AttachResult, error) {
	// ...connectKindNetwork, kindIP, certIPs unchanged...
	if err := s.writeCertificates(certIPs, extraHosts); err != nil {
		return nil, err
	}
	// ...restart, waitForAPI...
	if finalIP != "" && finalIP != initialIP {
		certIPs[0] = finalIP
		if err := s.writeCertificates(certIPs, extraHosts); err != nil {
			return nil, err
		}
		// ...restart, waitForAPI...
	}
	// ...unchanged...
}
```

(c) `tryAttachKind` (line 349) calls `s.AttachKind(ctx)` — update to `s.AttachKind(ctx, nil)`.

- [ ] **Step 4: Thread the URL host from the provider**

In `internal/cluster/kind_bootstrap_provider.go`, the `gitea-attach-kind` step (lines 158-177) calls `giteaService.AttachKind(ctx)`. Change it to extract the URL host from `cfg` and pass it:

```go
// before: if _, err := giteaService.AttachKind(ctx); err != nil {
var extraHosts []string
if u, err := url.Parse(strings.TrimSpace(cfg.OpenCenter.GitOps.Repository.URL)); err == nil && u.Hostname() != "" {
	extraHosts = append(extraHosts, u.Hostname())
}
if _, err := giteaService.AttachKind(ctx, extraHosts); err != nil {
	return fmt.Errorf("attach gitea to kind network: %w", err)
}
```

Add `"net/url"` to the imports of `kind_bootstrap_provider.go` (check it is not already imported).

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/localdev/gitea/ ./internal/cluster/ -count=1 && go build ./...`
Expected: all pass, clean build. (The existing `TestWriteCertificatesIncludesLocalAndKindSANs` must be updated to the new 2-arg signature — change its `service.writeCertificates([]string{...})` call to `service.writeCertificates([]string{...}, nil)` in Step 6.)

- [ ] **Step 6: Update the existing SAN test to the new signature**

In `internal/localdev/gitea/service_test.go`, change the existing call:

```go
// before:
if err := service.writeCertificates([]string{"10.89.0.11", "192.168.1.100"}); err != nil {
// after:
if err := service.writeCertificates([]string{"10.89.0.11", "192.168.1.100"}, nil); err != nil {
```

Run: `go test ./internal/localdev/gitea/ -count=1`
Expected: PASS.

- [ ] **Step 7: Add a generate round-trip test for the URL contract**

Add to `internal/gitops/ownership_test.go`. Keycloak is enabled by default in `newDefault` (verified), so the `opencenter-keycloak-config.yaml` source renders with no extra enable step — do not add a service-enable call:

```go
func TestRenderClusterAppsConfigSourcesFollowGitURL(t *testing.T) {
	repo := t.TempDir()
	cfg := newDefault("ownership-config-follow-url")
	cfg.OpenCenter.GitOps.Repository.LocalDir = repo
	cfg.OpenCenter.GitOps.Repository.URL = "https://gitea.oc-baremetal:3001/newuser/test-repo.git"
	if err := RenderClusterApps(cfg); err != nil {
		t.Fatalf("render: %v", err)
	}
	// The rendered keycloak-config GitRepository must carry the configured URL host.
	p := filepath.Join(repo, "applications", "overlays", cfg.ClusterName(), "services", "sources", "opencenter-keycloak-config.yaml")
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read rendered keycloak-config (keycloak is enabled by default): %v", err)
	}
	if !bytes.Contains(data, []byte("https://gitea.oc-baremetal:3001/newuser/test-repo.git")) {
		t.Fatalf("rendered source did not follow git_url; got:\n%s", data)
	}
}
```

Add `bytes` to the imports of `ownership_test.go` (it is not currently imported — `os`, `filepath`, `encoding/json`, `fmt`, `reflect`, `strings`, `testing` are).

Run: `go test ./internal/gitops/ -run TestRenderClusterAppsConfigSourcesFollowGitURL -v`
Expected: PASS (documents that the rendered source tracks `git_url`, so a post-generate URL change is corrected by a re-generate — legal now that Task 2 unblocks it).

- [ ] **Step 8: Commit**

```bash
git add internal/localdev/gitea/service.go internal/localdev/gitea/service_test.go internal/cluster/kind_bootstrap_provider.go internal/gitops/ownership_test.go
git commit -m "fix: include git_url host in gitea cert SANs and thread from kind provider"
```

---

## Post-implementation verification (all three together)

Re-run the real local Kind deploy end-to-end to confirm the manual workarounds are no longer needed:

- [ ] **A. Clean slate**

```bash
./bin/opencenter cluster destroy oc-kind --break-lock   # if it exists
# remove the foreign cert ownership from the prior manual fix to prove Task 1 handles it:
sudo chown 1000:1000 ~/.config/opencenter/local/gitea/gitea/certs/{ca,cert,key}.pem
sudo chmod 600 ~/.config/opencenter/local/gitea/gitea/certs/key.pem
```

- [ ] **B. Init with a friendly /etc/hosts hostname (proves Task 3a)**

```bash
./bin/opencenter cluster init oc-kind2 --org opencenter --type kind
./bin/opencenter cluster set oc-kind2 \
  "opencenter.gitops.repository.url=https://gitea.oc-baremetal:3001/newuser/test-repo.git" \
  "opencenter.gitops.auth.token.token_file=$HOME/.config/opencenter/local/tokens/gitea-user.token"
# confirm gitea.oc-baremetal resolves: getent hosts gitea.oc-baremetal
```

- [ ] **C. Generate → bootstrap → re-generate (proves Task 2)**

```bash
./bin/opencenter cluster generate oc-kind2
./bin/opencenter cluster deploy oc-kind2 --container-runtime podman --step gitea-attach-kind
./bin/opencenter cluster deploy oc-kind2 --container-runtime podman --step flux-bootstrap
# Now re-generate with the URL changed to the IP — must NOT be refused:
./bin/opencenter cluster set oc-kind2 "opencenter.gitops.repository.url=https://10.16.20.32:3001/newuser/test-repo.git"
./bin/opencenter cluster generate oc-kind2   # expect: success, -config sources rewritten to IP
```

- [ ] **D. Full deploy (proves Task 1 + 3a + 2 together)**

```bash
./bin/opencenter cluster deploy oc-kind2 --container-runtime podman --break-lock
./bin/opencenter cluster status oc-kind2 --refresh   # expect Status: success
kubectl --kubeconfig ~/.config/opencenter/clusters/state/opencenter/oc-kind2/kubeconfig.yaml get nodes
kubectl --kubeconfig ~/.config/opencenter/clusters/state/opencenter/oc-kind2/kubeconfig.yaml get gitrepositories -n flux-system   # all True
```

Expected: no `permission denied` on certs (Task 1), no `user-authored files found in generator-owned paths` on re-generate (Task 2), and `flux bootstrap` TLS succeeds with the `gitea.oc-baremetal` hostname (Task 3a).

---

## Self-Review Notes

- **Spec coverage:** Task 1 = cert ownership; Task 2 = overlay flux-system exemption; Task 3 = URL host SAN + generate contract. All three real failures addressed at the source.
- **Placeholder scan:** No "TBD"/"implement later". Task 3 Step 7 flags one helper (`defaultEnabledKeycloak()`) that may not exist and gives a concrete fallback (enable olm / assert olm-config) — the implementer must pick the real pattern from existing tests, which is acceptable because it is a test-creation detail, not a logic gap.
- **Type consistency:** `writeCertificates(extraIPs, extraHosts []string)` and `AttachKind(ctx, extraHosts)` are used consistently in Task 3 steps and the existing-test update in Step 6. `ownershipPathAllowed(path, clusterName)` signature unchanged in Task 2.
- **Risk:** Task 3a changes `writeCertificates` arity, so every caller must be updated (Step 3b lists `Up`, `AttachKind` x2, `tryAttachKind`). The `go build ./...` in Step 5 catches any missed caller. Task 2's exemption only narrows "allowed" paths and cannot break the existing round-trip tests (none place files under `flux-system`).
