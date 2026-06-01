package memory

import (
	"errors"
	"testing"
	"time"
)

func TestKeyStore_GetOrCreate_ReturnsExistingAndRefreshesLastSeen(t *testing.T) {
	store := newKeyStore[*bucket](options{
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

func TestKeyStore_GetOrCreate_CleansExpiredKeysBeforeMaxKeysCheck(t *testing.T) {
	store := newKeyStore[*bucket](options{
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

func TestKeyStore_GetOrCreate_RejectsNewKeyWhenMaxKeysReached(t *testing.T) {
	store := newKeyStore[*bucket](options{maxKeys: 1})
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
