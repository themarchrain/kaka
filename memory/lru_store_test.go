package memory

import (
	"testing"
	"time"
)

var _ stateStore[*bucket] = (*lruStore[*bucket])(nil)

func TestLRUStore_GetOrCreate_EvictsLeastRecentlyUsed(t *testing.T) {
	store := newLRUStore[*bucket](options{maxKeys: 3}, func(now time.Time) *bucket {
		return &bucket{tokens: 1, lastRefilled: now}
	})
	start := time.Now()

	for _, k := range []string{"a", "b", "c"} {
		if _, err := store.getOrCreate(k, start); err != nil {
			t.Fatalf("unexpected error creating %s: %v", k, err)
		}
	}
	// 访问 a，让 a 变为最新，b 成为最旧
	if _, err := store.getOrCreate("a", start.Add(time.Millisecond)); err != nil {
		t.Fatalf("unexpected error accessing a: %v", err)
	}

	// 满时新 key d → 应淘汰 b（最久未用）
	if _, err := store.getOrCreate("d", start.Add(2*time.Millisecond)); err != nil {
		t.Fatalf("expected eviction to allow new key, got %v", err)
	}
	if store.len() != 3 {
		t.Fatalf("expected len 3 after eviction, got %d", store.len())
	}
	if _, ok := store.items["b"]; ok {
		t.Fatal("expected b to be evicted")
	}
	if _, ok := store.items["a"]; !ok {
		t.Fatal("expected a to survive")
	}
}

func TestLRUStore_GetOrCreate_RefreshMovesToFront(t *testing.T) {
	store := newLRUStore[*bucket](options{maxKeys: 3}, func(now time.Time) *bucket {
		return &bucket{tokens: 1, lastRefilled: now}
	})
	start := time.Now()
	for _, k := range []string{"a", "b", "c"} {
		_, _ = store.getOrCreate(k, start)
	}
	// 访问 b（最旧→最新）
	_, _ = store.getOrCreate("b", start.Add(time.Millisecond))
	// 新 key d → 应淘汰 a（此时 a 最旧）
	_, _ = store.getOrCreate("d", start.Add(2*time.Millisecond))
	if _, ok := store.items["a"]; ok {
		t.Fatal("expected a to be evicted after b refresh")
	}
	if _, ok := store.items["b"]; !ok {
		t.Fatal("expected b to survive after refresh")
	}
}

func TestLRUStore_GetOrCreate_TTLCleanupCoexists(t *testing.T) {
	store := newLRUStore[*bucket](options{
		maxKeys:         3,
		keyTTL:          100 * time.Millisecond,
		cleanupInterval: 10 * time.Millisecond,
	}, func(now time.Time) *bucket {
		return &bucket{tokens: 1, lastRefilled: now}
	})
	start := time.Now()
	_, _ = store.getOrCreate("expired", start)
	_, _ = store.getOrCreate("active", start)

	_, err := store.getOrCreate("active", start.Add(150*time.Millisecond))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.len() != 1 {
		t.Fatalf("expected expired key cleaned by TTL, got len=%d", store.len())
	}
}

func TestLRUStore_GetOrCreate_FullAndAllActiveAllowsNewKey(t *testing.T) {
	store := newLRUStore[*bucket](options{maxKeys: 2}, func(now time.Time) *bucket {
		return &bucket{tokens: 1, lastRefilled: now}
	})
	start := time.Now()
	_, _ = store.getOrCreate("a", start)
	_, _ = store.getOrCreate("b", start)

	if _, err := store.getOrCreate("c", start.Add(time.Millisecond)); err != nil {
		t.Fatalf("expected LRU to always make room, got %v", err)
	}
	if store.len() != 2 {
		t.Fatalf("expected len 2, got %d", store.len())
	}
}
