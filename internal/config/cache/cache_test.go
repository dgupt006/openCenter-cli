package cache

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestDefaultsCache_BuildsOnce(t *testing.T) {
	var builds int
	cache := NewDefaultsCache(func(name string) string {
		builds++
		return fmt.Sprintf("default-%s", name)
	})

	first := cache.GetDefaultConfig("c1")
	second := cache.GetDefaultConfig("c1")

	if first != "default-c1" || second != "default-c1" {
		t.Fatalf("expected stable default value, got %q and %q", first, second)
	}
	if builds != 1 {
		t.Fatalf("expected buildDefault to run once, ran %d times", builds)
	}
}

func TestDefaultsCache_PerNameIsolation(t *testing.T) {
	cache := NewDefaultsCache(func(name string) string { return name + "-default" })

	if got := cache.GetDefaultConfig("a"); got != "a-default" {
		t.Fatalf("expected a-default, got %q", got)
	}
	if got := cache.GetDefaultConfig("b"); got != "b-default" {
		t.Fatalf("expected b-default, got %q", got)
	}
	if got := cache.GetDefaultConfig("a"); got != "a-default" {
		t.Fatalf("expected a-default after b lookup, got %q", got)
	}

	stats := cache.Stats()
	if stats.DefaultConfigCount != 2 {
		t.Fatalf("expected 2 default configs, got %d", stats.DefaultConfigCount)
	}
}

func TestDefaultsCache_ConcurrentGetBuildsOnce(t *testing.T) {
	var mu sync.Mutex
	builds := 0
	cache := NewDefaultsCache(func(name string) string {
		mu.Lock()
		builds++
		mu.Unlock()
		time.Sleep(time.Millisecond)
		return name
	})

	results := make(chan string, 50)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- cache.GetDefaultConfig("shared")
		}()
	}
	wg.Wait()
	close(results)
	for got := range results {
		if got != "shared" {
			t.Fatalf("expected shared, got %q", got)
		}
	}

	if builds != 1 {
		t.Fatalf("expected exactly one build under contention, got %d", builds)
	}
}

func TestDefaultsCache_Invalidate(t *testing.T) {
	var builds int
	cache := NewDefaultsCache(func(name string) string {
		builds++
		return fmt.Sprintf("v%d", builds)
	})

	cache.GetDefaultConfig("c1")
	cache.Invalidate("c1")
	again := cache.GetDefaultConfig("c1")

	if builds != 2 {
		t.Fatalf("expected rebuild after invalidate, builds=%d", builds)
	}
	if again != "v2" {
		t.Fatalf("expected v2 after invalidate, got %q", again)
	}
	if stats := cache.Stats(); stats.DefaultConfigCount != 1 {
		t.Fatalf("expected 1 cached default after rebuild, got %d", stats.DefaultConfigCount)
	}
}

func TestDefaultsCache_InvalidateAll(t *testing.T) {
	cache := NewDefaultsCache(func(name string) string { return name })
	cache.GetDefaultConfig("a")
	cache.GetDefaultConfig("b")

	cache.InvalidateAll()

	if stats := cache.Stats(); stats.DefaultConfigCount != 0 {
		t.Fatalf("expected 0 defaults after InvalidateAll, got %+v", stats)
	}
	var builds int
	rebuilt := NewDefaultsCache(func(name string) string {
		builds++
		return name
	})
	rebuilt.GetDefaultConfig("a")
	_ = cache.GetDefaultConfig("a")
	if builds != 1 {
		t.Fatalf("expected one build on rebuilt cache, got %d", builds)
	}
}

func TestNamedCache_SetAndGet(t *testing.T) {
	ctx := context.Background()
	cache := NewNamedCache[int]()

	if _, ok := cache.Get(ctx, "missing"); ok {
		t.Fatal("expected miss for missing key")
	}

	cache.Set(ctx, "k", 42)
	got, ok := cache.Get(ctx, "k")
	if !ok || got != 42 {
		t.Fatalf("expected (42, true), got (%d, %v)", got, ok)
	}
	if cache.Size() != 1 {
		t.Fatalf("expected size 1, got %d", cache.Size())
	}
}

func TestNamedCache_Overwrite(t *testing.T) {
	ctx := context.Background()
	cache := NewNamedCache[string]()
	cache.Set(ctx, "k", "one")
	cache.Set(ctx, "k", "two")

	got, ok := cache.Get(ctx, "k")
	if !ok || got != "two" {
		t.Fatalf("expected latest value, got (%q, %v)", got, ok)
	}
	if cache.Size() != 1 {
		t.Fatalf("expected size 1 after overwrite, got %d", cache.Size())
	}
}

func TestNamedCache_Expiration(t *testing.T) {
	ctx := context.Background()
	cache := NewNamedCache[string]()

	cache.SetWithExpiration(ctx, "expired", "old", time.Now().Add(-time.Minute))
	if _, ok := cache.Get(ctx, "expired"); ok {
		t.Fatal("expected expired entry to miss")
	}

	cache.SetWithExpiration(ctx, "valid", "new", time.Now().Add(time.Hour))
	got, ok := cache.Get(ctx, "valid")
	if !ok || got != "new" {
		t.Fatalf("expected (new, true), got (%q, %v)", got, ok)
	}

	cache.Set(ctx, "noexpiry", "persist")
	time.Sleep(10 * time.Millisecond)
	if got, ok := cache.Get(ctx, "noexpiry"); !ok || got != "persist" {
		t.Fatalf("zero ExpiresAt should never expire, got (%q, %v)", got, ok)
	}
}

func TestNamedCache_InvalidateAndClear(t *testing.T) {
	ctx := context.Background()
	cache := NewNamedCache[int]()
	cache.Set(ctx, "a", 1)
	cache.Set(ctx, "b", 2)

	cache.Invalidate(ctx, "a")
	if _, ok := cache.Get(ctx, "a"); ok {
		t.Fatal("expected a to be invalidated")
	}
	if _, ok := cache.Get(ctx, "b"); !ok {
		t.Fatal("expected b to remain")
	}

	cache.Clear(ctx)
	if cache.Size() != 0 {
		t.Fatalf("expected empty cache after Clear, size=%d", cache.Size())
	}
}

func TestNamedCache_ConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	cache := NewNamedCache[int]()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("k%d", i%10)
			cache.Set(ctx, key, i)
			cache.Get(ctx, key)
			cache.Invalidate(ctx, key)
			cache.Size()
		}(i)
	}
	wg.Wait()
}
