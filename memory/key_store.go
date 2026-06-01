package memory

import "time"

type keyStore[T any] struct {
	items       map[string]*keyEntry[T]
	opts        options
	lastCleanup time.Time
}

type keyEntry[T any] struct {
	value    T
	lastSeen time.Time
}

func newKeyStore[T any](opts options) *keyStore[T] {
	return &keyStore[T]{
		items: make(map[string]*keyEntry[T]),
		opts:  opts,
	}
}

func (s *keyStore[T]) getOrCreate(key string, now time.Time, create func(time.Time) T) (T, error) {
	if entry, ok := s.items[key]; ok {
		entry.lastSeen = now
		return entry.value, nil
	}

	s.cleanup(now)
	if s.opts.maxKeys > 0 && len(s.items) >= s.opts.maxKeys {
		var zero T
		return zero, ErrMaxKeysExceeded
	}

	value := create(now)
	s.items[key] = &keyEntry[T]{
		value:    value,
		lastSeen: now,
	}
	return value, nil
}

func (s *keyStore[T]) len() int {
	return len(s.items)
}

// cleanup 在 keyTTL 和 cleanupInterval 都启用时，按批次清理过期 key
// 调用方必须直接或间接持有 mutex
func (s *keyStore[T]) cleanup(now time.Time) {
	if s.opts.keyTTL <= 0 || s.opts.cleanupInterval <= 0 {
		return
	}
	if now.Sub(s.lastCleanup) < s.opts.cleanupInterval {
		return
	}
	s.lastCleanup = now

	expired := make([]string, 0, cleanupBatchSize)
	scanned := 0
	for key, entry := range s.items {
		if scanned >= cleanupBatchSize {
			break
		}
		scanned++
		if now.Sub(entry.lastSeen) >= s.opts.keyTTL {
			expired = append(expired, key)
		}
	}
	for _, key := range expired {
		delete(s.items, key)
	}
}
