// Copyright 2025.
// Licensed under the Apache License, Version 2.0.

package secrets

import (
	"path/filepath"

	"github.com/opencenter-cloud/opencenter-cli/internal/secretartifacts"
)

// serviceScopedSecretPath builds the overlay-relative path a generic per-service
// artifact materializes to, matching the planner's services/<service>/secret.yaml
// route. generateSecretManifest derives the Kubernetes Secret name partly from
// this path, so tests must pass the file they are actually pretending to write.
func serviceScopedSecretPath(service string) string {
	return filepath.Join("services", service, "secret.yaml")
}

// certManagerCredentialSecretPath builds the overlay-relative path of a
// cert-manager DNS-credential Secret through the planner's own builder, so a
// test cannot pin a filename the planner no longer produces.
func certManagerCredentialSecretPath(t testingTB, provider, name string) string {
	t.Helper()
	relative, err := secretartifacts.CertManagerCredentialArtifactPath(provider, name)
	if err != nil {
		t.Fatalf("cert-manager credential path for %s/%s: %v", provider, name, err)
	}
	return filepath.FromSlash(relative)
}

// testingTB is the subset of *testing.T these helpers need.
type testingTB interface {
	Helper()
	Fatalf(format string, args ...any)
}
