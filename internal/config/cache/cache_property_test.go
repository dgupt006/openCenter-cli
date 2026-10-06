package cache

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

func TestProperty_NamedCache_SetGetRoundTrip(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)
	properties.Property("named cache returns the value last set for a key", prop.ForAll(
		func(values []int) bool {
			if len(values) == 0 {
				return true
			}
			ctx := context.Background()
			cache := NewNamedCache[int]()
			for i, value := range values {
				cache.Set(ctx, fmt.Sprintf("key-%d", i), value)
			}
			for i, value := range values {
				got, ok := cache.Get(ctx, fmt.Sprintf("key-%d", i))
				if !ok || got != value {
					return false
				}
			}
			return cache.Size() == len(values)
		},
		gen.SliceOf(gen.Int()),
	))

	properties.TestingRun(t)
}

func TestProperty_NamedCache_ExpiredEntriesAlwaysMiss(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50

	properties := gopter.NewProperties(parameters)
	properties.Property("entries with a past ExpiresAt never hit", prop.ForAll(
		func(name string, value string, pastOffset int) bool {
			ctx := context.Background()
			cache := NewNamedCache[string]()
			expiresAt := time.Now().Add(-time.Duration(1+pastOffset) * time.Millisecond)
			cache.SetWithExpiration(ctx, name, value, expiresAt)

			_, ok := cache.Get(ctx, name)
			return !ok
		},
		gen.AlphaString(),
		gen.AlphaString(),
		gen.IntRange(0, 10000),
	))

	properties.TestingRun(t)
}

func TestProperty_NamedCache_FutureExpiryAlwaysHits(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50

	properties := gopter.NewProperties(parameters)
	properties.Property("entries with a future ExpiresAt always hit", prop.ForAll(
		func(name string, value string, futureOffset int) bool {
			ctx := context.Background()
			cache := NewNamedCache[string]()
			expiresAt := time.Now().Add(time.Duration(1+futureOffset) * time.Hour)
			cache.SetWithExpiration(ctx, name, value, expiresAt)

			got, ok := cache.Get(ctx, name)
			return ok && got == value
		},
		gen.AlphaString(),
		gen.AlphaString(),
		gen.IntRange(0, 1000),
	))

	properties.TestingRun(t)
}

func TestProperty_DefaultsCache_ValueConsistency(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50

	properties := gopter.NewProperties(parameters)
	properties.Property("defaults cache is stable per name and rebuild-free under concurrency", prop.ForAll(
		func(names []string) bool {
			if len(names) == 0 {
				return true
			}
			builds := 0
			var mu sync.Mutex
			cache := NewDefaultsCache(func(name string) string {
				mu.Lock()
				builds++
				mu.Unlock()
				return "value-" + name
			})

			var wg sync.WaitGroup
			for _, name := range names {
				wg.Add(1)
				go func(name string) {
					defer wg.Done()
					if got := cache.GetDefaultConfig(name); got != "value-"+name {
						t.Errorf("unexpected value for %q: %q", name, got)
						return
					}
				}(name)
			}
			wg.Wait()

			unique := make(map[string]bool, len(names))
			for _, name := range names {
				unique[name] = true
			}
			if builds != len(unique) {
				t.Errorf("expected %d builds, got %d", len(unique), builds)
				return false
			}
			if stats := cache.Stats(); stats.DefaultConfigCount != len(unique) {
				t.Errorf("expected %d cached defaults, got %d", len(unique), stats.DefaultConfigCount)
				return false
			}
			return true
		},
		gen.SliceOf(gen.AlphaString()),
	))

	properties.TestingRun(t)
}
