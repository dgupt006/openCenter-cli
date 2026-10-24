package v2

import (
	"regexp"
	"testing"
)

// The regex is applied by SOPS per mapping key, so it must compile and match
// whole key names only.
func TestDefaultSOPSEncryptedRegexMatchesCredentialKeys(t *testing.T) {
	re, err := regexp.Compile(DefaultSOPSEncryptedRegex)
	if err != nil {
		t.Fatalf("DefaultSOPSEncryptedRegex does not compile: %v", err)
	}

	sensitive := []string{
		"data", "stringData", "credentials", "password", "passwordHash",
		"htpasswdString", "secret", "token",
		"clientSecret", "client_secret",
		"accessKeyId", "access_key_id", "access_key", "accesskey",
		"secretAccessKey", "secret_access_key", "secret_key", "secretkey",
		"application_credential_secret", "application-credential-secret",
		"tls.crt", "tls.key",
	}
	for _, key := range sensitive {
		if !re.MatchString(key) {
			t.Errorf("credential key %q must be encrypted", key)
		}
	}

	benign := []string{
		"ca_bundle", "ca", "cert", "crt", "tls", "keys", "key", "kubeconfig",
		"token_limit", "auth-url", "region", "endpoint", "enabled", "storageClass",
		"signatureVersion", "tenant-name", "domain-name", "datastoreurl",
	}
	for _, key := range benign {
		if re.MatchString(key) {
			t.Errorf("non-credential key %q must not be encrypted", key)
		}
	}
}
