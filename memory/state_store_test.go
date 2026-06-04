package memory

import (
	"errors"
	"strconv"
	"testing"
	"time"
)

var _ stateStore[*bucket] = (*mapStore[*bucket])(nil)

func TestMapStore_GetOrCreate_ReturnsExistingAndRefreshesLastSeen(t *testing.T) {
	store := newMapStore[*bucket](options{
		maxKeys:         1,
		keyTTL:          100 * time.Millisecond,
		cleanupInterval: 10 * time.Millisecond,
	})
	start := time.Now()

	first, err := store.getOrCreate("user:1", start, func(now time.Time) *bucket {
		return &bucket{tokens: 1, lastRefilled: now}
	})
	if err != nil {
		t.Fatalf("unexpected error creating first key: %v", err)
	}

	second, err := store.getOrCreate("user:1", start.Add(90*time.Millisecond), func(now time.Time) *bucket {
		return &bucket{tokens: 2, lastRefilled: now}
	})
	if err != nil {
		t.Fatalf("unexpected error getting existing key: %v", err)
	}
	if second != first {
		t.Fatal("expected existing key to return the original value")
	}

	_, err = store.getOrCreate("user:2", start.Add(150*time.Millisecond), func(now time.Time) *bucket {
		return &bucket{tokens: 1, lastRefilled: now}
	})
	if !errors.Is(err, ErrMaxKeysExceeded) {
		t.Fatalf("expected refreshed existing key to block new key, got %v", err)
	}
}

func TestMapStore_GetOrCreate_CleansExpiredKeysWhenExistingKeyIsAccessed(t *testing.T) {
	store := newMapStore[*bucket](options{
		keyTTL:          100 * time.Millisecond,
		cleanupInterval: 10 * time.Millisecond,
	})
	start := time.Now()

	active, err := store.getOrCreate("active", start, func(now time.Time) *bucket {
		return &bucket{tokens: 1, lastRefilled: now}
	})
	if err != nil {
		t.Fatalf("unexpected error creating active key: %v", err)
	}
	_, err = store.getOrCreate("expired", start, func(now time.Time) *bucket {
		return &bucket{tokens: 1, lastRefilled: now}
	})
	if err != nil {
		t.Fatalf("unexpected error creating expired key: %v", err)
	}

	got, err := store.getOrCreate("active", start.Add(150*time.Millisecond), func(now time.Time) *bucket {
		return &bucket{tokens: 2, lastRefilled: now}
	})
	if err != nil {
		t.Fatalf("unexpected error getting active key: %v", err)
	}
	if got != active {
		t.Fatal("expected existing active key to be returned")
	}
	if store.len() != 1 {
		t.Fatalf("expected expired key to be cleaned during existing-key access, got len=%d", store.len())
	}
}

func TestMapStore_GetOrCreate_CleansExpiredKeysBeforeMaxKeysCheck(t *testing.T) {
	store := newMapStore[*bucket](options{
		maxKeys:         1,
		keyTTL:          100 * time.Millisecond,
		cleanupInterval: 10 * time.Millisecond,
	})
	start := time.Now()

	_, err := store.getOrCreate("user:1", start, func(now time.Time) *bucket {
		return &bucket{tokens: 1, lastRefilled: now}
	})
	if err != nil {
		t.Fatalf("unexpected error creating first key: %v", err)
	}

	second, err := store.getOrCreate("user:2", start.Add(150*time.Millisecond), func(now time.Time) *bucket {
		return &bucket{tokens: 1, lastRefilled: now}
	})
	if err != nil {
		t.Fatalf("expected expired key to be cleaned before maxKeys check, got %v", err)
	}
	if second == nil {
		t.Fatal("expected new key state to be created")
	}
	if store.len() != 1 {
		t.Fatalf("expected store to contain 1 key after cleanup, got %d", store.len())
	}
}

func TestMapStore_CleanupDeletesAtMostBatchSize(t *testing.T) {
	store := newMapStore[*bucket](options{
		keyTTL:          time.Millisecond,
		cleanupInterval: time.Millisecond,
	})
	start := time.Now()
	total := cleanupBatchSize + 10

	for i := 0; i < total; i++ {
		key := "user:" + strconv.Itoa(i)
		store.items[key] = &keyEntry[*bucket]{
			value:    &bucket{tokens: 1, lastRefilled: start},
			lastSeen: start,
		}
	}

	store.cleanup(start.Add(2 * time.Millisecond))

	if got, want := store.len(), total-cleanupBatchSize; got != want {
		t.Fatalf("expected cleanup to delete at most %d keys, got len=%d want %d", cleanupBatchSize, got, want)
	}
}

func TestMapStore_CleanupRunsAtExactIntervalBoundary(t *testing.T) {
	store := newMapStore[*bucket](options{
		keyTTL:          time.Millisecond,
		cleanupInterval: 10 * time.Millisecond,
	})
	start := time.Now()
	store.items["user:1"] = &keyEntry[*bucket]{
		value:    &bucket{tokens: 1, lastRefilled: start},
		lastSeen: start,
	}
	store.lastCleanup = start

	store.cleanup(start.Add(10 * time.Millisecond))

	if got := store.len(); got != 0 {
		t.Fatalf("expected cleanup to run at exact interval boundary, got len=%d", got)
	}
}

func TestMapStore_CleanupDoesNotRefreshLastCleanupBeforeInterval(t *testing.T) {
	store := newMapStore[*bucket](options{
		keyTTL:          time.Millisecond,
		cleanupInterval: 10 * time.Millisecond,
	})
	start := time.Now()
	store.lastCleanup = start

	store.cleanup(start.Add(9 * time.Millisecond))

	if !store.lastCleanup.Equal(start) {
		t.Fatalf("expected lastCleanup to remain %v before interval, got %v", start, store.lastCleanup)
	}
}

func TestMapStore_CleanupRefreshesLastCleanupWhenIntervalElapses(t *testing.T) {
	store := newMapStore[*bucket](options{
		keyTTL:          100 * time.Millisecond,
		cleanupInterval: 10 * time.Millisecond,
	})
	start := time.Now()
	now := start.Add(10 * time.Millisecond)
	store.lastCleanup = start
	store.items["user:1"] = &keyEntry[*bucket]{
		value:    &bucket{tokens: 1, lastRefilled: start},
		lastSeen: start,
	}

	store.cleanup(now)

	if !store.lastCleanup.Equal(now) {
		t.Fatalf("expected lastCleanup to refresh to %v, got %v", now, store.lastCleanup)
	}
	if store.len() != 1 {
		t.Fatalf("expected unexpired key to remain, got len=%d", store.len())
	}
}

func TestMapStore_GetOrCreate_RejectsNewKeyWhenMaxKeysReached(t *testing.T) {
	store := newMapStore[*bucket](options{maxKeys: 1})
	now := time.Now()

	_, err := store.getOrCreate("user:1", now, func(now time.Time) *bucket {
		return &bucket{tokens: 1, lastRefilled: now}
	})
	if err != nil {
		t.Fatalf("unexpected error creating first key: %v", err)
	}

	_, err = store.getOrCreate("user:2", now, func(now time.Time) *bucket {
		return &bucket{tokens: 1, lastRefilled: now}
	})
	if !errors.Is(err, ErrMaxKeysExceeded) {
		t.Fatalf("expected ErrMaxKeysExceeded, got %v", err)
	}
}
