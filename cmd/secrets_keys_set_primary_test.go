package cmd

import "testing"

func TestSecretsKeysSetPrimaryFlags(t *testing.T) {
	cmd := newSecretsKeysSetPrimaryCmd()
	for _, name := range []string{"cluster", "type", "fingerprint"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Fatalf("missing --%s flag", name)
		}
	}
}

func TestSecretsKeysSetPrimaryRejectsInvalidType(t *testing.T) {
	cmd := newSecretsKeysSetPrimaryCmd()
	cmd.SetArgs([]string{"--cluster", "cluster-a", "--type", "rsa", "--fingerprint", "fp"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected invalid key type error")
	}
}
