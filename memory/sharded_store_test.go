package memory

import (
	"errors"
	"strconv"
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
	}, nil)
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

// ---- migrated from state_store_test.go (shardCount=1 == former mapStore semantics) ----

func TestShardedStore_GetOrCreate_ReturnsExistingAndRefreshesLastSeen(t *testing.T) {
	store := newTestStore(options{
		maxKeys:         1,
		keyTTL:          100 * time.Millisecond,
		cleanupInterval: 10 * time.Millisecond,
	}, 1, nil)
	start := time.Now()

	first, err := store.getOrCreate("user:1", start)
	if err != nil {
		t.Fatalf("unexpected error creating first key: %v", err)
	}
	second, err := store.getOrCreate("user:1", start.Add(90*time.Millisecond))
	if err != nil {
		t.Fatalf("unexpected error getting existing key: %v", err)
	}
	if second != first {
		t.Fatal("expected existing key to return the original value")
	}
	if store.len() != 1 {
		t.Fatalf("expected len 1, got %d", store.len())
	}

	_, err = store.getOrCreate("user:2", start.Add(150*time.Millisecond))
	if !errors.Is(err, ErrMaxKeysExceeded) {
		t.Fatalf("expected refreshed existing key to block new key, got %v", err)
	}
}

func TestShardedStore_GetOrCreate_CleansExpiredKeysBeforeMaxKeysCheck(t *testing.T) {
	store := newTestStore(options{
		maxKeys:         1,
		keyTTL:          100 * time.Millisecond,
		cleanupInterval: 10 * time.Millisecond,
	}, 1, nil)
	start := time.Now()

	if _, err := store.getOrCreate("user:1", start); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	second, err := store.getOrCreate("user:2", start.Add(150*time.Millisecond))
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

func TestShardedStore_CleanupDeletesAtMostBatchSize(t *testing.T) {
	store := newTestStore(options{
		keyTTL:          time.Millisecond,
		cleanupInterval: time.Millisecond,
	}, 1, nil)
	start := time.Now()
	sh := store.shards[0]
	total := cleanupBatchSize + 10

	for i := 0; i < total; i++ {
		key := "user:" + strconv.Itoa(i)
		sh.items[key] = &keyEntry[*bucket]{
			value:    &bucket{tokens: 1, lastRefilled: start},
			lastSeen: start,
		}
	}
	store.live.Store(int64(total))

	store.cleanupMap(sh, start.Add(2*time.Millisecond))

	if got, want := store.len(), total-cleanupBatchSize; got != want {
		t.Fatalf("expected cleanup to delete at most %d keys, got len=%d want %d", cleanupBatchSize, got, want)
	}
}

func TestShardedStore_CleanupRunsAtExactIntervalBoundary(t *testing.T) {
	store := newTestStore(options{
		keyTTL:          time.Millisecond,
		cleanupInterval: 10 * time.Millisecond,
	}, 1, nil)
	start := time.Now()
	sh := store.shards[0]
	sh.items["user:1"] = &keyEntry[*bucket]{
		value:    &bucket{tokens: 1, lastRefilled: start},
		lastSeen: start,
	}
	sh.lastCleanup = start
	store.live.Store(1)

	store.cleanupMap(sh, start.Add(10*time.Millisecond))

	if got := store.len(); got != 0 {
		t.Fatalf("expected cleanup to run at exact interval boundary, got len=%d", got)
	}
}

func TestShardedStore_CleanupDoesNotRefreshBeforeInterval(t *testing.T) {
	store := newTestStore(options{
		keyTTL:          time.Millisecond,
		cleanupInterval: 10 * time.Millisecond,
	}, 1, nil)
	start := time.Now()
	sh := store.shards[0]
	sh.lastCleanup = start

	store.cleanupMap(sh, start.Add(9*time.Millisecond))

	if !sh.lastCleanup.Equal(start) {
		t.Fatalf("expected lastCleanup to remain %v, got %v", start, sh.lastCleanup)
	}
}

// ---- migrated from lru_store_test.go (shardCount=1 == former lruStore semantics) ----

func TestShardedStore_EvictLRU_EvictsLeastRecentlyUsed(t *testing.T) {
	store := newTestStore(options{maxKeys: 3, eviction: EvictLRU}, 1, nil)
	start := time.Now()

	for _, k := range []string{"a", "b", "c"} {
		if _, err := store.getOrCreate(k, start); err != nil {
			t.Fatalf("unexpected error creating %s: %v", k, err)
		}
	}
	// Touch a -> a newest, b oldest.
	if _, err := store.getOrCreate("a", start.Add(time.Millisecond)); err != nil {
		t.Fatalf("unexpected error accessing a: %v", err)
	}

	// Full; new key d -> evicts b (least recently used).
	if _, err := store.getOrCreate("d", start.Add(2*time.Millisecond)); err != nil {
		t.Fatalf("expected eviction to allow new key, got %v", err)
	}
	if store.len() != 3 {
		t.Fatalf("expected len 3 after eviction, got %d", store.len())
	}
	if _, ok := store.shards[0].orderItems["b"]; ok {
		t.Fatal("expected b to be evicted")
	}
	if _, ok := store.shards[0].orderItems["a"]; !ok {
		t.Fatal("expected a to survive")
	}
	// Direct concrete call (also marks the method as used for staticcheck,
	// which does not count interface-only calls on generic receivers).
	if got := store.drainEvictions(); got != 1 {
		t.Fatalf("expected 1 drained eviction, got %d", got)
	}
}

func TestShardedStore_EvictLRU_RefreshMovesToFront(t *testing.T) {
	store := newTestStore(options{maxKeys: 3, eviction: EvictLRU}, 1, nil)
	start := time.Now()
	for _, k := range []string{"a", "b", "c"} {
		_, _ = store.getOrCreate(k, start)
	}
	// Touch b (oldest -> newest); new key d evicts a (now oldest).
	_, _ = store.getOrCreate("b", start.Add(time.Millisecond))
	_, _ = store.getOrCreate("d", start.Add(2*time.Millisecond))
	if _, ok := store.shards[0].orderItems["a"]; ok {
		t.Fatal("expected a to be evicted after b refresh")
	}
	if _, ok := store.shards[0].orderItems["b"]; !ok {
		t.Fatal("expected b to survive after refresh")
	}
}

func TestShardedStore_EvictLRU_TTLCleanupCoexists(t *testing.T) {
	store := newTestStore(options{
		maxKeys:         3,
		eviction:        EvictLRU,
		keyTTL:          100 * time.Millisecond,
		cleanupInterval: 10 * time.Millisecond,
	}, 1, nil)
	start := time.Now()
	_, _ = store.getOrCreate("expired", start)
	_, _ = store.getOrCreate("active", start)

	if _, err := store.getOrCreate("active", start.Add(150*time.Millisecond)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.len() != 1 {
		t.Fatalf("expected expired key cleaned by TTL, got len=%d", store.len())
	}
}

func TestShardedStore_EvictLRU_FullAllActiveStillAllowsNewKey(t *testing.T) {
	// Former lruStore semantics: when full with all keys active, LRU still
	// makes room (never rejects).
	store := newTestStore(options{maxKeys: 2, eviction: EvictLRU}, 1, nil)
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

func TestShardedStore_EvictLRU_EmptyTargetShard_EvictsOtherShard(t *testing.T) {
	// Cross-shard eviction fallback: global full, new key lands on an empty
	// shard -> must make room from another shard (approximate LRU).
	store := newTestStore(options{maxKeys: 2, eviction: EvictLRU}, 4, nil)
	var a, b, c string
	used := map[uint64]bool{}
	for i := 0; ; i++ {
		k := "user:" + strconv.Itoa(i)
		h := fnv1a64(k) & 3
		if used[h] {
			continue
		}
		used[h] = true
		if a == "" {
			a = k
		} else if b == "" {
			b = k
		} else {
			c = k
			break
		}
	}
	start := time.Now()
	if _, err := store.getOrCreate(a, start); err != nil {
		t.Fatal(err)
	}
	if _, err := store.getOrCreate(b, start); err != nil {
		t.Fatal(err)
	}
	// live=2=maxKeys, c lands on the third (empty) shard -> cross-shard eviction.
	if _, err := store.getOrCreate(c, start.Add(time.Millisecond)); err != nil {
		t.Fatalf("expected cross-shard eviction to make room, got %v", err)
	}
	if store.len() != 2 {
		t.Fatalf("expected len 2, got %d", store.len())
	}
}

func TestShardedStore_FullWithExpiredKeysInOtherShards_AllowsNewKey(t *testing.T) {
	// Regression: per-shard cleanup only scans the accessed shard, but the
	// maxKeys check is global. A full store whose expired keys live in other
	// shards must still admit a new key (global sweep on the full path).
	store := newTestStore(options{
		maxKeys:         2,
		keyTTL:          100 * time.Millisecond,
		cleanupInterval: 10 * time.Millisecond,
	}, 64, nil)
	start := time.Now()

	var a, b, c string
	used := map[uint64]bool{}
	for i := 0; ; i++ {
		k := "user:" + strconv.Itoa(i)
		h := fnv1a64(k) & 63
		if used[h] {
			continue
		}
		used[h] = true
		if a == "" {
			a = k
		} else if b == "" {
			b = k
		} else {
			c = k
			break
		}
	}
	if _, err := store.getOrCreate(a, start); err != nil {
		t.Fatal(err)
	}
	if _, err := store.getOrCreate(b, start); err != nil {
		t.Fatal(err)
	}
	// live=2=maxKeys, c lands on a third (empty) shard; a and b expired
	// (TTL 100ms) by now -> the global sweep must free slots before the
	// maxKeys check rejects c.
	if _, err := store.getOrCreate(c, start.Add(150*time.Millisecond)); err != nil {
		t.Fatalf("expected expired keys to be swept before maxKeys check, got %v", err)
	}
	if store.len() != 1 {
		t.Fatalf("expected len 1 (only c) after sweep, got %d", store.len())
	}
}

// ---- differential: identical behavior with 1 vs 64 shards ----

// Under the same single-threaded key sequence, 1-shard and 64-shard stores
// must agree on success/error outcomes (maxKeys bound and hit semantics;
// NOT which key is evicted - approximate LRU allows shard-level differences).
func TestShardedStore_Differential_OneVsManyShards(t *testing.T) {
	run := func(shards int) []string {
		store := newTestStore(options{maxKeys: 8, eviction: EvictLRU}, shards, nil)
		now := time.Now()
		seq := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}
		var out []string
		for i, k := range seq {
			_, err := store.getOrCreate(k, now.Add(time.Duration(i)*time.Millisecond))
			if err != nil {
				out = append(out, "ERR")
			} else {
				out = append(out, "OK")
			}
			// Every other step revisits an old key.
			if i%2 == 1 {
				_, _ = store.getOrCreate(seq[i/2], now.Add(time.Duration(i)*time.Millisecond))
			}
		}
		return out
	}
	one, many := run(1), run(64)
	for i := range one {
		if one[i] != many[i] {
			t.Fatalf("step %d: shard=1 %v, shard=64 %v", i, one[i], many[i])
		}
	}
}

// ---- hash distribution sanity ----

func TestShardedStore_ShardDistribution_IsBalanced(t *testing.T) {
	counts := make([]int, 64)
	for i := 0; i < 1024; i++ {
		counts[fnv1a64("user:"+strconv.Itoa(i))&63]++
	}
	mean := 1024 / 64
	for i, c := range counts {
		if c > mean*3 {
			t.Fatalf("shard %d got %d keys (mean %d): distribution too skewed", i, c, mean)
		}
	}
}
