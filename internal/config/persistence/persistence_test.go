package persistence

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

func TestNormalizeDir_EmptyErrors(t *testing.T) {
	for _, input := range []string{"", "   ", "\t\n"} {
		if _, err := NormalizeDir(input); err == nil {
			t.Fatalf("NormalizeDir(%q): expected error for empty/whitespace input", input)
		}
	}
}

func TestNormalizeDir_CleansAbsolutePath(t *testing.T) {
	got, err := NormalizeDir("/tmp/a/b/../c")
	if err != nil {
		t.Fatalf("NormalizeDir: %v", err)
	}
	if got != "/tmp/a/c" {
		t.Fatalf("expected /tmp/a/c, got %q", got)
	}
}

func TestNormalizeDir_ConvertsRelative(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}

	got, err := NormalizeDir("subdir")
	if err != nil {
		t.Fatalf("NormalizeDir: %v", err)
	}
	want := filepath.Join(cwd, "subdir")
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestResolveDir_CreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "new", "nested", "dir")
	got, err := ResolveDir(dir)
	if err != nil {
		t.Fatalf("ResolveDir: %v", err)
	}
	if got != dir {
		t.Fatalf("expected %q, got %q", dir, got)
	}
	info, err := os.Stat(got)
	if err != nil {
		t.Fatalf("expected directory to exist: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("expected %q to be a directory", got)
	}
}

func TestResolveDir_RejectsEmpty(t *testing.T) {
	if _, err := ResolveDir(""); err == nil {
		t.Fatal("expected error for empty dir")
	}
}

func TestResolveDir_FailsOnUncreatablePath(t *testing.T) {
	existingFile := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(existingFile, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// A path that must traverse through a regular file cannot be created.
	if _, err := ResolveDir(filepath.Join(existingFile, "subdir")); err == nil {
		t.Fatal("expected error when directory path traverses a regular file")
	}
}

func TestDefaultConfigDir_RespectsEnv(t *testing.T) {
	t.Setenv("OPENCENTER_CONFIG_DIR", filepath.Join(t.TempDir(), "cfg"))

	got := DefaultConfigDir()
	_, err := filepath.Abs(filepath.Join(t.TempDir(), "cfg"))
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}
	wantPrefix := "cfg"
	if !strings.HasSuffix(got, wantPrefix) {
		t.Fatalf("expected DefaultConfigDir to end with %q, got %q", wantPrefix, got)
	}
}

func TestDefaultStateDir_RespectsEnv(t *testing.T) {
	t.Setenv("OPENCENTER_STATE_DIR", filepath.Join(t.TempDir(), "state"))

	got := DefaultStateDir()
	if !strings.HasSuffix(got, "state") {
		t.Fatalf("expected DefaultStateDir to end with %q, got %q", "state", got)
	}
}

func TestResolveConfigDir_Creates(t *testing.T) {
	base := filepath.Join(t.TempDir(), "resolve-cfg")
	t.Setenv("OPENCENTER_CONFIG_DIR", base)

	got, err := ResolveConfigDir()
	if err != nil {
		t.Fatalf("ResolveConfigDir: %v", err)
	}
	if !strings.HasSuffix(got, "resolve-cfg") {
		t.Fatalf("unexpected resolved dir %q", got)
	}
	if info, err := os.Stat(got); err != nil || !info.IsDir() {
		t.Fatalf("expected %q to be created as a directory", got)
	}
}

func TestMarshalYAML_NilPointerErrors(t *testing.T) {
	if _, err := MarshalYAML((*struct{})(nil)); err == nil {
		t.Fatal("expected error marshalling nil pointer")
	}
}

func TestUnmarshalYAML_EmptyErrors(t *testing.T) {
	if _, err := UnmarshalYAML[struct{}](nil); err == nil {
		t.Fatal("expected error unmarshalling empty data")
	}
}

type testYAML struct {
	Name  string `yaml:"name"`
	Count int    `yaml:"count"`
}

func TestYAMLRoundTrip(t *testing.T) {
	original := &testYAML{Name: "cluster-a", Count: 3}

	data, err := MarshalYAML(original)
	if err != nil {
		t.Fatalf("MarshalYAML: %v", err)
	}
	decoded, err := UnmarshalYAML[testYAML](data)
	if err != nil {
		t.Fatalf("UnmarshalYAML: %v", err)
	}
	if decoded.Name != original.Name || decoded.Count != original.Count {
		t.Fatalf("round trip mismatch: want %+v, got %+v", original, decoded)
	}
}

func TestUnmarshalYAML_InvalidYAMLErrors(t *testing.T) {
	if _, err := UnmarshalYAML[testYAML]([]byte("not: [valid")); err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestParseClusterIdentifier_Delelegates(t *testing.T) {
	noop := func(string) error { return nil }

	org, cluster, err := ParseClusterIdentifier("org/cluster", noop)
	if err != nil {
		t.Fatalf("ParseClusterIdentifier: %v", err)
	}
	if org != "org" || cluster != "cluster" {
		t.Fatalf("expected (org, cluster), got (%q, %q)", org, cluster)
	}

	org, cluster, err = ParseClusterIdentifier("cluster", noop)
	if err != nil {
		t.Fatalf("ParseClusterIdentifier: %v", err)
	}
	if org != "opencenter" || cluster != "cluster" {
		t.Fatalf("expected (opencenter, cluster), got (%q, %q)", org, cluster)
	}

	if _, _, err = ParseClusterIdentifier("", noop); err == nil {
		t.Fatal("expected error for empty identifier")
	}

	validationErr := errors.New("cluster name invalid")
	if _, _, err = ParseClusterIdentifier("bad", func(string) error { return validationErr }); !errors.Is(err, validationErr) {
		t.Fatalf("expected validation error to propagate, got %v", err)
	}
}

func TestProperty_NormalizeDir_ProducesAbsoluteCleanPath(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)
	properties.Property("NormalizeDir always returns a clean absolute path", prop.ForAll(
		func(dir string) bool {
			if strings.TrimSpace(dir) == "" {
				return true
			}
			got, err := NormalizeDir(dir)
			if err != nil {
				return false
			}
			return filepath.IsAbs(got) && filepath.Clean(got) == got
		},
		gen.AlphaString(),
	))

	properties.TestingRun(t)
}

func TestProperty_YAMLRoundTrip(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)
	properties.Property("MarshalYAML/UnmarshalYAML round-trips any value", prop.ForAll(
		func(name string, count int) bool {
			original := &testYAML{Name: name, Count: count}
			data, err := MarshalYAML(original)
			if err != nil {
				return false
			}
			decoded, err := UnmarshalYAML[testYAML](data)
			if err != nil {
				return false
			}
			return decoded.Name == name && decoded.Count == count
		},
		gen.AlphaString(),
		gen.Int(),
	))

	properties.TestingRun(t)
}
