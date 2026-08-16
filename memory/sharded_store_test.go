package memory

import (
	"errors"
	"sync"
	"testing"
	"time"
)

var _ stateStore[*bucket] = (*shardedStore[*bucket])(nil)

func newTestStore(opts options, shards int, created *int) *shardedStore[*bucket] {
	return newShardedStoreWithShards(opts, shards, func(now time.Time) *bucket {
		if created != nil {
			*created++
		}
		return &bucket{tokens: 1, lastRefilled: now}
	})
}

func TestShardedStore_GetOrCreate_HitDoesNotCallCreate(t *testing.T) {
	created := 0
	store := newTestStore(defaultOptions(), 1, &created)
	now := time.Now()

	first, err := store.getOrCreate("user:1", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	second, err := store.getOrCreate("user:1", now.Add(time.Second))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if second != first {
		t.Fatal("expected hit to return original value")
	}
	if created != 1 {
		t.Fatalf("expected create once, got %d", created)
	}
}

func TestShardedStore_GetOrCreate_RejectsNewKeyWhenFull(t *testing.T) {
	store := newTestStore(options{maxKeys: 1}, 1, nil)
	now := time.Now()

	if _, err := store.getOrCreate("user:1", now); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err := store.getOrCreate("user:2", now)
	if !errors.Is(err, ErrMaxKeysExceeded) {
		t.Fatalf("expected ErrMaxKeysExceeded, got %v", err)
	}
	if store.len() != 1 {
		t.Fatalf("expected len 1, got %d", store.len())
	}
}

func TestShardedStore_GetOrCreate_MaxKeysZeroIsUnlimited(t *testing.T) {
	store := newTestStore(defaultOptions(), 1, nil)
	now := time.Now()
	for i := 0; i < 100; i++ {
		if _, err := store.getOrCreate("user:"+string(rune('a'+i)), now); err != nil {
			t.Fatalf("unexpected error at %d: %v", i, err)
		}
	}
	if store.len() != 100 {
		t.Fatalf("expected len 100, got %d", store.len())
	}
}

func TestShardedStore_ConcurrentNewKeys_RespectsMaxKeysExactly(t *testing.T) {
	// Store-level contract test: 32 concurrent new keys, maxKeys=4 ->
	// exactly 4 succeed and 28 are rejected.
	store := newTestStore(options{maxKeys: 4}, 64, nil)
	var wg sync.WaitGroup
	results := make(chan error, 32)
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := store.getOrCreate("user:"+string(rune('a'+i)), time.Now())
			results <- err
		}(i)
	}
	wg.Wait()
	close(results)

	allowed, rejected := 0, 0
	for err := range results {
		if err == nil {
			allowed++
		} else if errors.Is(err, ErrMaxKeysExceeded) {
			rejected++
		} else {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if allowed != 4 || rejected != 28 {
		t.Fatalf("expected 4 allowed / 28 rejected, got %d / %d", allowed, rejected)
	}
	if store.len() != 4 {
		t.Fatalf("expected len 4, got %d", store.len())
	}
}
