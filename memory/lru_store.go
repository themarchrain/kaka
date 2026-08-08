package memory

import (
	"container/list"
	"time"
)

type lruStore[T any] struct {
	items       map[string]*list.Element
	order       *list.List // 表头=最新，表尾=最旧
	opts        options
	lastCleanup time.Time
	create      func(time.Time) T
}

type lruEntry[T any] struct {
	key      string
	value    T
	lastSeen time.Time
}

func newLRUStore[T any](opts options, create func(time.Time) T) *lruStore[T] {
	return &lruStore[T]{
		items:  make(map[string]*list.Element),
		order:  list.New(),
		opts:   opts,
		create: create,
	}
}

func (s *lruStore[T]) getOrCreate(key string, now time.Time) (T, error) {
	if elem, ok := s.items[key]; ok {
		entry := elem.Value.(*lruEntry[T])
		entry.lastSeen = now
		s.order.MoveToFront(elem)
		s.cleanup(now)
		return entry.value, nil
	}

	s.cleanup(now)
	if s.opts.maxKeys > 0 && s.order.Len() >= s.opts.maxKeys {
		s.evict()
	}

	value := s.create(now)
	s.items[key] = s.order.PushFront(&lruEntry[T]{key: key, value: value, lastSeen: now})
	return value, nil
}

// evict 淘汰链表尾部（最久未使用）的 key
func (s *lruStore[T]) evict() {
	if elem := s.order.Back(); elem != nil {
		entry := elem.Value.(*lruEntry[T])
		delete(s.items, entry.key)
		s.order.Remove(elem)
	}
}

func (s *lruStore[T]) len() int { return s.order.Len() }

// cleanup 与 mapStore 同语义：keyTTL + cleanupInterval 分批扫描
func (s *lruStore[T]) cleanup(now time.Time) {
	if s.opts.keyTTL <= 0 || s.opts.cleanupInterval <= 0 {
		return
	}
	if now.Sub(s.lastCleanup) < s.opts.cleanupInterval {
		return
	}
	s.lastCleanup = now

	expired := make([]string, 0, cleanupBatchSize)
	scanned := 0
	for elem := s.order.Front(); elem != nil && scanned < cleanupBatchSize; elem = elem.Next() {
		scanned++
		entry := elem.Value.(*lruEntry[T])
		if now.Sub(entry.lastSeen) >= s.opts.keyTTL {
			expired = append(expired, entry.key)
		}
	}
	for _, key := range expired {
		if elem, ok := s.items[key]; ok {
			s.order.Remove(elem)
			delete(s.items, key)
		}
	}
}
