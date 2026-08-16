package memory

import (
	"container/list"
	"sync"
	"sync/atomic"
	"time"
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

// shardedStore is a stateStore implementation sharded by key hash.
// Concurrency safety comes from the per-shard mutexes; the live atomic
// counter keeps maxKeys exact across shards.
type shardedStore[T any] struct {
	shards []*shard[T]
	opts   options
	live   atomic.Int64
	create func(time.Time) T
}

const (
	defaultShardCount = 64
	maxShardCount     = 1024
)

func newShardedStore[T any](opts options, create func(time.Time) T) *shardedStore[T] {
	return newShardedStoreWithShards[T](opts, opts.shards, create)
}

func newShardedStoreWithShards[T any](opts options, shardCount int, create func(time.Time) T) *shardedStore[T] {
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
	return &shardedStore[T]{shards: shards, opts: opts, create: create}
}

func (s *shardedStore[T]) shardFor(key string) *shard[T] {
	return s.shards[fnv1a64(key)&uint64(len(s.shards)-1)]
}

func (s *shardedStore[T]) getOrCreate(key string, now time.Time) (T, error) {
	sh := s.shardFor(key)
	sh.mu.Lock()
	defer sh.mu.Unlock()

	if s.opts.eviction == EvictLRU {
		return s.getOrCreateLRU(sh, key, now)
	}
	return s.getOrCreateMap(sh, key, now)
}

func (s *shardedStore[T]) getOrCreateMap(sh *shard[T], key string, now time.Time) (T, error) {
	if entry, ok := sh.items[key]; ok {
		entry.lastSeen = now
		s.cleanupMap(sh, now)
		return entry.value, nil
	}

	s.cleanupMap(sh, now)
	if !s.reserveSlot(sh) {
		var zero T
		return zero, ErrMaxKeysExceeded
	}

	// reserveSlot may temporarily release this shard's lock on the
	// "global full but local shard empty" fallback path; after re-acquiring,
	// the key may have been created concurrently: release the reservation
	// and return the existing value (keeps the counter exact).
	if entry, ok := sh.items[key]; ok {
		s.live.Add(-1)
		entry.lastSeen = now
		return entry.value, nil
	}

	value := s.create(now)
	sh.items[key] = &keyEntry[T]{value: value, lastSeen: now}
	return value, nil
}

func (s *shardedStore[T]) getOrCreateLRU(sh *shard[T], key string, now time.Time) (T, error) {
	if elem, ok := sh.orderItems[key]; ok {
		entry := elem.Value.(*lruEntry[T])
		entry.lastSeen = now
		sh.order.MoveToFront(elem)
		s.cleanupLRU(sh, now)
		return entry.value, nil
	}

	s.cleanupLRU(sh, now)
	if !s.reserveSlot(sh) {
		var zero T
		return zero, ErrMaxKeysExceeded
	}

	if elem, ok := sh.orderItems[key]; ok {
		s.live.Add(-1)
		entry := elem.Value.(*lruEntry[T])
		entry.lastSeen = now
		sh.order.MoveToFront(elem)
		return entry.value, nil
	}

	value := s.create(now)
	elem := sh.order.PushFront(&lruEntry[T]{key: key, value: value, lastSeen: now})
	sh.orderItems[key] = elem
	return value, nil
}

// reserveSlot reserves a maxKeys slot for a new key (CAS loop, exact
// globally). The caller holds sh.mu; under EvictLRU a full store first
// evicts locally, and when the local shard is empty it temporarily releases
// the lock to evict from another shard (the caller re-holds the lock on
// return), then retries the CAS.
//
// maxKeys=0 (unlimited) still counts via live.Add so len() reflects the
// real key count; counting only happens on the new-key path, never on hits.
func (s *shardedStore[T]) reserveSlot(sh *shard[T]) bool {
	if s.opts.maxKeys <= 0 {
		s.live.Add(1)
		return true
	}
	for {
		n := s.live.Load()
		if n < int64(s.opts.maxKeys) {
			if s.live.CompareAndSwap(n, n+1) {
				return true
			}
			continue
		}
		// Full: EvictReject denies; EvictLRU evicts one and retries.
		if s.opts.eviction != EvictLRU {
			return false
		}
		if s.evictFrom(sh) {
			continue // local eviction freed one slot
		}
		// Local shard empty: release the local lock, evict from another
		// shard one at a time (single lock held, no nesting, no deadlock).
		sh.mu.Unlock()
		s.evictAnyOther(sh)
		sh.mu.Lock()
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
