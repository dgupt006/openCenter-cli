package orchestration

import (
	"context"
	"strings"
	"sync"
	"testing"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// fakeOrchestrator is a minimal ProviderOrchestrator for registry tests.
type fakeOrchestrator struct {
	name      string
	supported []string
}

func (f *fakeOrchestrator) Name() string { return f.name }

func (f *fakeOrchestrator) Supports(provider string) bool {
	for _, s := range f.supported {
		if s == provider {
			return true
		}
	}
	return false
}

func (f *fakeOrchestrator) Discover(_ context.Context, _ *v2.Config) (DiscoveryResult, error) {
	return DiscoveryResult{}, nil
}

func (f *fakeOrchestrator) Prompts(_ *v2.Config, _ DiscoveryResult) []PromptSpec { return nil }

func (f *fakeOrchestrator) ApplyAnswers(_ *v2.Config, _ PromptAnswers) (ChangeSet, error) {
	return ChangeSet{}, nil
}

func (f *fakeOrchestrator) CapabilityRequests(_ *v2.Config, _ DiscoveryResult) []CapabilityRequest {
	return nil
}

var _ ProviderOrchestrator = (*fakeOrchestrator)(nil)

// fakeCapability is a minimal CapabilityHandler for registry tests.
type fakeCapability struct {
	name string
}

func (f *fakeCapability) Name() string { return f.name }

func (f *fakeCapability) Applies(_ *v2.Config, _ ProviderContext) bool { return true }

func (f *fakeCapability) Discover(_ context.Context, _ *v2.Config, _ ProviderContext) (DiscoveryResult, error) {
	return DiscoveryResult{}, nil
}

func (f *fakeCapability) Prompts(_ *v2.Config, _ ProviderContext, _ DiscoveryResult) []PromptSpec {
	return nil
}

func (f *fakeCapability) ApplyAnswers(_ *v2.Config, _ PromptAnswers, _ ProviderContext) (ChangeSet, error) {
	return ChangeSet{}, nil
}

var _ CapabilityHandler = (*fakeCapability)(nil)

func TestProviderRegistry_Resolve(t *testing.T) {
	kind := &fakeOrchestrator{name: "kind", supported: []string{"kind"}}
	openstack := &fakeOrchestrator{name: "openstack", supported: []string{"openstack", "magnum"}}

	registry := NewProviderRegistry(openstack, kind)

	resolved, err := registry.Resolve("kind")
	if err != nil {
		t.Fatalf("Resolve(kind): %v", err)
	}
	if resolved.Name() != "kind" {
		t.Fatalf("expected kind, got %s", resolved.Name())
	}

	resolved, err = registry.Resolve("magnum")
	if err != nil {
		t.Fatalf("Resolve(magnum): %v", err)
	}
	if resolved.Name() != "openstack" {
		t.Fatalf("expected openstack (supports magnum), got %s", resolved.Name())
	}

	if _, err := registry.Resolve("vsphere"); err == nil {
		t.Fatal("expected error for unregistered provider")
	} else if !strings.Contains(err.Error(), "vsphere") {
		t.Fatalf("expected error to mention provider, got %v", err)
	}
}

func TestProviderRegistry_TrimsProviderName(t *testing.T) {
	kind := &fakeOrchestrator{name: "kind", supported: []string{"kind"}}
	registry := NewProviderRegistry(kind)

	resolved, err := registry.Resolve("  kind\t")
	if err != nil {
		t.Fatalf("Resolve(padded kind): %v", err)
	}
	if resolved.Name() != "kind" {
		t.Fatalf("expected kind, got %s", resolved.Name())
	}
}

func TestProviderRegistry_SortsByName(t *testing.T) {
	zeta := &fakeOrchestrator{name: "zeta"}
	alpha := &fakeOrchestrator{name: "alpha"}
	mid := &fakeOrchestrator{name: "mid"}

	registry := NewProviderRegistry(zeta, alpha, mid)

	if registry.items[0].Name() != "alpha" || registry.items[1].Name() != "mid" || registry.items[2].Name() != "zeta" {
		t.Fatalf("expected items sorted by name, got [%s %s %s]",
			registry.items[0].Name(), registry.items[1].Name(), registry.items[2].Name())
	}
}

func TestProviderRegistry_FirstMatchWins(t *testing.T) {
	// Both support "generic"; the alphabetically first name should win.
	first := &fakeOrchestrator{name: "aaa", supported: []string{"generic"}}
	second := &fakeOrchestrator{name: "bbb", supported: []string{"generic"}}

	registry := NewProviderRegistry(second, first)
	resolved, err := registry.Resolve("generic")
	if err != nil {
		t.Fatalf("Resolve(generic): %v", err)
	}
	if resolved.Name() != "aaa" {
		t.Fatalf("expected first-registered (sorted) match aaa, got %s", resolved.Name())
	}
}

func TestProviderRegistry_EmptyRegistry(t *testing.T) {
	registry := NewProviderRegistry()
	if _, err := registry.Resolve("anything"); err == nil {
		t.Fatal("expected error from empty registry")
	}
}

func TestCapabilityRegistry_Resolve(t *testing.T) {
	registry := NewCapabilityRegistry(&fakeCapability{name: "dns"}, &fakeCapability{name: "storage"})

	resolved, err := registry.Resolve("dns")
	if err != nil {
		t.Fatalf("Resolve(dns): %v", err)
	}
	if resolved.Name() != "dns" {
		t.Fatalf("expected dns, got %s", resolved.Name())
	}

	if _, err := registry.Resolve("storage"); err != nil {
		t.Fatalf("Resolve(storage): %v", err)
	}

	if _, err := registry.Resolve("missing"); err == nil {
		t.Fatal("expected error for missing capability")
	} else if !strings.Contains(err.Error(), "missing") {
		t.Fatalf("expected error to mention capability name, got %v", err)
	}
}

func TestCapabilityRegistry_TrimsName(t *testing.T) {
	registry := NewCapabilityRegistry(&fakeCapability{name: "dns"})

	resolved, err := registry.Resolve("  dns\t")
	if err != nil {
		t.Fatalf("Resolve(padded dns): %v", err)
	}
	if resolved.Name() != "dns" {
		t.Fatalf("expected dns, got %s", resolved.Name())
	}
}

func TestCapabilityRegistry_EmptyRegistry(t *testing.T) {
	registry := NewCapabilityRegistry()
	if _, err := registry.Resolve("anything"); err == nil {
		t.Fatal("expected error from empty capability registry")
	}
}

func TestPromptAnswers_MapSemantics(t *testing.T) {
	answers := PromptAnswers{"a": "1"}
	answers["b"] = "2"

	if answers["a"] != "1" || answers["b"] != "2" {
		t.Fatalf("unexpected answers: %v", answers)
	}
	if len(answers) != 2 {
		t.Fatalf("expected 2 answers, got %d", len(answers))
	}
}

func TestChangeSet_Defaults(t *testing.T) {
	cs := ChangeSet{}
	if cs.Patches != nil || cs.Files != nil || cs.Warnings != nil {
		t.Fatalf("expected nil slices in zero ChangeSet, got %+v", cs)
	}
}

func TestPromptKind_Values(t *testing.T) {
	kinds := map[PromptKind]string{
		PromptKindInput:   "input",
		PromptKindSelect:  "select",
		PromptKindConfirm: "confirm",
		PromptKindSecret:  "secret",
	}
	for kind, want := range kinds {
		if string(kind) != want {
			t.Fatalf("expected %q, got %q", want, string(kind))
		}
	}
}

func TestProperty_ProviderRegistry_Resolution(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50

	properties := gopter.NewProperties(parameters)
	properties.Property("registry resolves any provider one of its items supports", prop.ForAll(
		func(providers []string) bool {
			if len(providers) == 0 {
				return true
			}
			orchestrators := make([]ProviderOrchestrator, 0, len(providers))
			for _, p := range providers {
				orchestrators = append(orchestrators, &fakeOrchestrator{name: p, supported: []string{p}})
			}
			registry := NewProviderRegistry(orchestrators...)

			for _, p := range providers {
				resolved, err := registry.Resolve(p)
				if err != nil {
					return false
				}
				if !resolved.Supports(p) {
					return false
				}
			}
			return true
		},
		gen.SliceOf(gen.AlphaString()),
	))

	properties.TestingRun(t)
}

func TestProperty_ProviderRegistry_ConcurrentResolveIsSafe(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 20

	properties := gopter.NewProperties(parameters)
	properties.Property("concurrent Resolve never fails for a supported provider", prop.ForAll(
		func(providers []string) bool {
			if len(providers) == 0 {
				return true
			}
			orchestrators := make([]ProviderOrchestrator, 0, len(providers))
			for _, p := range providers {
				orchestrators = append(orchestrators, &fakeOrchestrator{name: p, supported: []string{p}})
			}
			registry := NewProviderRegistry(orchestrators...)

			var wg sync.WaitGroup
			errs := make(chan error, len(providers))
			for _, p := range providers {
				wg.Add(1)
				go func(p string) {
					defer wg.Done()
					if _, err := registry.Resolve(p); err != nil {
						errs <- err
					}
				}(p)
			}
			wg.Wait()
			close(errs)

			for range errs {
				return false
			}
			return true
		},
		gen.SliceOf(gen.AlphaString()),
	))

	properties.TestingRun(t)
}

var _ = paths.ClusterPaths{} // keep the paths import for future ProviderContext tests
