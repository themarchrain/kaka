package memory

import (
	"container/list"
	"sync"
	"sync/atomic"
	"time"

	"github.com/themarchrain/kaka"
)

// shard is one stripe of the sharded store: its own mutex plus its own
// storage. EvictReject mode uses items; EvictLRU mode uses orderItems + order
// (the same memory shapes as the former mapStore/lruStore, so LRU mode does
// not pay for an extra index map).
type shard[T any] struct {
	mu          sync.Mutex
	items       map[string]*keyEntry[T]  // EvictReject mode
	orderItems  map[string]*list.Element // EvictLRU mode: key -> list element
	order       *list.List               // EvictLRU only, non-nil; front=newest, back=oldest
	lastCleanup time.Time
}

// lruEntry is the value stored in an EvictLRU shard's order list.
type lruEntry[T any] struct {
	key      string
	value    T
	lastSeen time.Time
}

// shardedStore is a stateStore implementation sharded by key hash.
// Concurrency safety comes from the per-shard mutexes; the live atomic
// counter keeps maxKeys exact across shards.
type shardedStore[T any] struct {
	shards    []*shard[T]
	opts      options
	live      atomic.Int64
	create    func(time.Time) T
	sweepMu   sync.Mutex // guards lastSweep; acquired before any shard lock
	lastSweep time.Time  // global rate limit for sweepExpired
	evicts    atomic.Int64 // pending evictions, drained outside shard locks
	onEvict   func()       // optional; fired per drained eviction, never under a shard lock
}

const (
	defaultShardCount = 64
	maxShardCount     = 1024
)

func newShardedStore[T any](opts options, create func(time.Time) T, onEvict func()) *shardedStore[T] {
	return newShardedStoreWithShards[T](opts, opts.shards, create, onEvict)
}

func newShardedStoreWithShards[T any](opts options, shardCount int, create func(time.Time) T, onEvict func()) *shardedStore[T] {
	shards := make([]*shard[T], shardCount)
	for i := range shards {
		s := &shard[T]{}
		if opts.eviction == EvictLRU {
			s.orderItems = make(map[string]*list.Element)
			s.order = list.New()
		} else {
			s.items = make(map[string]*keyEntry[T])
		}
		shards[i] = s
	}
	return &shardedStore[T]{shards: shards, opts: opts, create: create, onEvict: onEvict}
}

func (s *shardedStore[T]) shardFor(key string) *shard[T] {
	return s.shards[fnv1a64(key)&uint64(len(s.shards)-1)]
}

// withState runs fn with the key's state while holding the shard lock, so
// the per-key mutation (refill math, log append) is serialized with other
// requests on the same key. Returns ErrMaxKeysExceeded without calling fn
// when the key is new and the store is full.
func (s *shardedStore[T]) withState(key string, now time.Time, fn func(T, time.Time) (kaka.Result, error)) (kaka.Result, error) {
	sh := s.shardFor(key)
	sh.mu.Lock()
	defer sh.mu.Unlock()

	var v T
	var ok bool
	if s.opts.eviction == EvictLRU {
		v, ok = s.getOrCreateLRU(sh, key, now)
	} else {
		v, ok = s.getOrCreateMap(sh, key, now)
	}
	if !ok {
		return kaka.Result{}, ErrMaxKeysExceeded
	}
	return fn(v, now)
}

// getOrCreate returns the key's state, creating it if absent. Test-only: it
// returns a raw pointer with no lock held, so production code must use
// withState instead (mutating the returned state outside the lock would
// reintroduce the same-key data race). Not part of the stateStore interface.
func (s *shardedStore[T]) getOrCreate(key string, now time.Time) (T, error) {
	var out T
	_, err := s.withState(key, now, func(v T, _ time.Time) (kaka.Result, error) {
		out = v
		return kaka.Result{}, nil
	})
	return out, err
}

// getOrCreateMap returns the key's state in an EvictReject shard, creating it
// if absent; ok=false means ErrMaxKeysExceeded. The caller holds sh.mu and
// must re-check the key after any path that temporarily releases the lock.
func (s *shardedStore[T]) getOrCreateMap(sh *shard[T], key string, now time.Time) (T, bool) {
	if entry, ok := sh.items[key]; ok {
		entry.lastSeen = now
		s.cleanupMap(sh, now)
		return entry.value, true
	}

	s.cleanupMap(sh, now)
	if !s.reserveSlot(sh, now) {
		var zero T
		return zero, false
	}

	// reserveSlot may temporarily release this shard's lock on the
	// "global full but local shard empty" fallback path; after re-acquiring,
	// the key may have been created concurrently: release the reservation
	// and return the existing value (keeps the counter exact).
	if entry, ok := sh.items[key]; ok {
		s.live.Add(-1)
		entry.lastSeen = now
		return entry.value, true
	}

	value := s.create(now)
	sh.items[key] = &keyEntry[T]{value: value, lastSeen: now}
	return value, true
}

// getOrCreateLRU returns the key's state in an EvictLRU shard, creating it if
// absent; ok=false means the store could not make room (defensive; LRU never
// rejects). The caller holds sh.mu and must re-check the key after any path
// that temporarily releases the lock.
func (s *shardedStore[T]) getOrCreateLRU(sh *shard[T], key string, now time.Time) (T, bool) {
	if elem, ok := sh.orderItems[key]; ok {
		entry := elem.Value.(*lruEntry[T])
		entry.lastSeen = now
		sh.order.MoveToFront(elem)
		s.cleanupLRU(sh, now)
		return entry.value, true
	}

	s.cleanupLRU(sh, now)
	if !s.reserveSlot(sh, now) {
		var zero T
		return zero, false
	}

	if elem, ok := sh.orderItems[key]; ok {
		s.live.Add(-1)
		entry := elem.Value.(*lruEntry[T])
		entry.lastSeen = now
		sh.order.MoveToFront(elem)
		return entry.value, true
	}

	value := s.create(now)
	elem := sh.order.PushFront(&lruEntry[T]{key: key, value: value, lastSeen: now})
	sh.orderItems[key] = elem
	return value, true
}

// reserveSlot reserves a maxKeys slot for a new key (CAS loop, exact
// globally). The caller holds sh.mu and must re-check the key after any
// path that temporarily releases the lock (see getOrCreateMap/LRU).
//
// When the store is full it first runs a global expired-key sweep (releasing
// sh.mu first: cross-shard locking while holding a shard lock could deadlock),
// then retries. EvictReject denies when still full; EvictLRU evicts locally
// (or from another shard when this shard is empty) so it never rejects.
//
// maxKeys=0 (unlimited) still counts via live.Add so len() reflects the
// real key count; counting only happens on the new-key path, never on hits.
func (s *shardedStore[T]) reserveSlot(sh *shard[T], now time.Time) bool {
	if s.opts.maxKeys <= 0 {
		s.live.Add(1)
		return true
	}
	swept := false
	for {
		n := s.live.Load()
		if n < int64(s.opts.maxKeys) {
			if s.live.CompareAndSwap(n, n+1) {
				return true
			}
			continue
		}
		// Full.
		if s.opts.eviction == EvictLRU {
			if s.evictFrom(sh) { // local eviction, we hold the lock
				continue
			}
			// Local shard empty: release the lock, sweep expired keys
			// globally, then evict from another shard if still full.
			// Single lock held at any time, no nesting, no deadlock.
			sh.mu.Unlock()
			if !swept {
				swept = true
				s.sweepExpired(now)
			}
			if s.live.Load() >= int64(s.opts.maxKeys) {
				s.evictAnyOther(sh)
			}
			sh.mu.Lock()
			continue
		}
		// EvictReject: sweep expired keys once (the expired keys may live in
		// other shards; the old single-store cleanup ran before the maxKeys
		// check, and the sweep preserves that contract), then re-check.
		if !swept {
			swept = true
			sh.mu.Unlock()
			s.sweepExpired(now)
			sh.mu.Lock()
			continue
		}
		return false
	}
}

// sweepExpired scans every shard and removes expired keys (one shard lock at
// a time). It is rate-limited by a global lastSweep + cleanupInterval so the
// full path stays O(1) between sweeps. The caller must not hold any shard lock.
func (s *shardedStore[T]) sweepExpired(now time.Time) {
	if s.opts.keyTTL <= 0 || s.opts.cleanupInterval <= 0 {
		return
	}
	s.sweepMu.Lock()
	defer s.sweepMu.Unlock()
	if now.Sub(s.lastSweep) < s.opts.cleanupInterval {
		return
	}
	s.lastSweep = now

	for _, sh := range s.shards {
		sh.mu.Lock()
		if s.opts.eviction == EvictLRU {
			s.cleanupLRU(sh, now)
		} else {
			s.cleanupMap(sh, now)
		}
		sh.mu.Unlock()
	}
}

// evictAnyOther evicts the LRU tail of any non-empty shard (one shard lock
// at a time; the caller must not hold any shard lock).
func (s *shardedStore[T]) evictAnyOther(self *shard[T]) {
	for _, other := range s.shards {
		if other == self {
			continue
		}
		other.mu.Lock()
		evicted := s.evictFrom(other)
		other.mu.Unlock()
		if evicted {
			return
		}
	}
}

// evictFrom evicts the LRU tail of the given shard and releases its slot
// (the caller must hold that shard's lock).
func (s *shardedStore[T]) evictFrom(sh *shard[T]) bool {
	if sh.order == nil || sh.order.Len() == 0 {
		return false
	}
	elem := sh.order.Back()
	entry := elem.Value.(*lruEntry[T])
	delete(sh.orderItems, entry.key)
	sh.order.Remove(elem)
	s.live.Add(-1)
	s.evicts.Add(1)
	return true
}

// cleanupMap lazily scans items in batches (keyTTL + cleanupInterval),
// EvictReject mode. The caller must hold sh's lock.
func (s *shardedStore[T]) cleanupMap(sh *shard[T], now time.Time) {
	if s.opts.keyTTL <= 0 || s.opts.cleanupInterval <= 0 {
		return
	}
	if now.Sub(sh.lastCleanup) < s.opts.cleanupInterval {
		return
	}
	sh.lastCleanup = now

	expired := make([]string, 0, cleanupBatchSize)
	scanned := 0
	for key, entry := range sh.items {
		if scanned >= cleanupBatchSize {
			break
		}
		scanned++
		if now.Sub(entry.lastSeen) >= s.opts.keyTTL {
			expired = append(expired, key)
		}
	}
	for _, key := range expired {
		if _, ok := sh.items[key]; ok {
			delete(sh.items, key)
			s.live.Add(-1)
		}
	}
}

// cleanupLRU lazily scans the list from the front in batches and removes
// expired keys together with their list elements, EvictLRU mode. The caller
// must hold sh's lock.
func (s *shardedStore[T]) cleanupLRU(sh *shard[T], now time.Time) {
	if s.opts.keyTTL <= 0 || s.opts.cleanupInterval <= 0 {
		return
	}
	if now.Sub(sh.lastCleanup) < s.opts.cleanupInterval {
		return
	}
	sh.lastCleanup = now

	expired := make([]*list.Element, 0, cleanupBatchSize)
	scanned := 0
	for elem := sh.order.Front(); elem != nil && scanned < cleanupBatchSize; elem = elem.Next() {
		scanned++
		entry := elem.Value.(*lruEntry[T])
		if now.Sub(entry.lastSeen) >= s.opts.keyTTL {
			expired = append(expired, elem)
		}
	}
	for _, elem := range expired {
		entry := elem.Value.(*lruEntry[T])
		if _, ok := sh.orderItems[entry.key]; ok {
			delete(sh.orderItems, entry.key)
			sh.order.Remove(elem)
			s.live.Add(-1)
		}
	}
}

func (s *shardedStore[T]) len() int {
	return int(s.live.Load())
}

// drainEvictions returns the pending eviction count and fires onEvict once
// per eviction. The caller must not hold any shard lock: sink callbacks run
// here, and a callback that re-enters the limiter must be able to acquire
// the shard locks.
func (s *shardedStore[T]) drainEvictions() int {
	n := int(s.evicts.Swap(0))
	for i := 0; i < n && s.onEvict != nil; i++ {
		s.onEvict()
	}
	return n
}
