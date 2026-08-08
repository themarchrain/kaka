package memory

import "time"

// clockStore 近似 LRU（clock / second-chance）：ring（slice）+ map
// 命中只置访问位，热路径零移动；满时从 hand 环形扫描清位/淘汰
type clockStore[T any] struct {
	items       map[string]*clockEntry[T]
	order       []*clockEntry[T] // 逻辑环
	hand        int              // 下一淘汰扫描位置
	opts        options
	lastCleanup time.Time
	create      func(time.Time) T
}

type clockEntry[T any] struct {
	key        string
	value      T
	lastSeen   time.Time
	referenced bool // 访问位
	index      int  // 在 order 中的位置
}

func newClockStore[T any](opts options, create func(time.Time) T) *clockStore[T] {
	return &clockStore[T]{
		items:  make(map[string]*clockEntry[T]),
		opts:   opts,
		create: create,
	}
}

func (s *clockStore[T]) getOrCreate(key string, now time.Time) (T, error) {
	if entry, ok := s.items[key]; ok {
		entry.referenced = true
		entry.lastSeen = now
		s.cleanup(now)
		return entry.value, nil
	}

	s.cleanup(now)
	if s.opts.maxKeys > 0 && len(s.items) >= s.opts.maxKeys {
		s.evict()
	}

	value := s.create(now)
	entry := &clockEntry[T]{key: key, value: value, lastSeen: now, index: len(s.order)}
	s.items[key] = entry
	s.order = append(s.order, entry)
	return value, nil
}

// evict 环形扫描：referenced=true → 清位跳过（second chance）；false → 淘汰
func (s *clockStore[T]) evict() {
	n := len(s.order)
	for i := 0; i < n; i++ {
		idx := (s.hand + i) % n
		entry := s.order[idx]
		if entry.referenced {
			entry.referenced = false
			continue
		}
		s.removeAt(idx)
		return
	}
	// 兜底：全部活跃（本轮全被清位），淘汰 hand 处
	s.removeAt(s.hand % n)
}

// removeAt 交换删除并维护 index / hand
func (s *clockStore[T]) removeAt(idx int) {
	last := len(s.order) - 1
	entry := s.order[idx]
	delete(s.items, entry.key)
	if idx != last {
		moved := s.order[last]
		s.order[idx] = moved
		moved.index = idx
	}
	s.order = s.order[:last]
	if len(s.order) > 0 {
		s.hand = idx % len(s.order)
	} else {
		s.hand = 0
	}
}

func (s *clockStore[T]) len() int { return len(s.items) }

// cleanup 与 mapStore 同语义：keyTTL + cleanupInterval 分批扫描
func (s *clockStore[T]) cleanup(now time.Time) {
	if s.opts.keyTTL <= 0 || s.opts.cleanupInterval <= 0 {
		return
	}
	if now.Sub(s.lastCleanup) < s.opts.cleanupInterval {
		return
	}
	s.lastCleanup = now

	expired := make([]*clockEntry[T], 0, cleanupBatchSize)
	scanned := 0
	for i := 0; i < len(s.order) && scanned < cleanupBatchSize; i++ {
		entry := s.order[i]
		scanned++
		if now.Sub(entry.lastSeen) >= s.opts.keyTTL {
			expired = append(expired, entry)
		}
	}
	for _, entry := range expired {
		if entry.index < len(s.order) && s.order[entry.index] == entry {
			s.removeAt(entry.index)
		}
	}
}
