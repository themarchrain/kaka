package memory

import (
	"math/rand"
	"testing"
	"time"
)

var _ stateStore[*bucket] = (*clockStore[*bucket])(nil)

func TestClockStore_GetOrCreate_EvictsWhenFull(t *testing.T) {
	store := newClockStore[*bucket](options{maxKeys: 3}, func(now time.Time) *bucket {
		return &bucket{tokens: 1, lastRefilled: now}
	})
	start := time.Now()
	for _, k := range []string{"a", "b", "c"} {
		_, _ = store.getOrCreate(k, start)
	}
	// 访问 a、b，令其 referenced=true
	_, _ = store.getOrCreate("a", start.Add(time.Millisecond))
	_, _ = store.getOrCreate("b", start.Add(time.Millisecond))

	// 满时新 key d → 必须进入，且长度保持 3
	if _, err := store.getOrCreate("d", start.Add(2*time.Millisecond)); err != nil {
		t.Fatalf("expected eviction to allow new key, got %v", err)
	}
	if store.len() != 3 {
		t.Fatalf("expected len 3, got %d", store.len())
	}
}

func TestClockStore_GetOrCreate_TTLCleanupCoexists(t *testing.T) {
	store := newClockStore[*bucket](options{
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
		t.Fatalf("expected TTL cleanup, got len=%d", store.len())
	}
}

// 淘汰准确性：与理想 LRU（测试内参考实现）对比淘汰集合重合率
func TestClockStore_EvictionMatchesIdealLRU(t *testing.T) {
	seq := buildDeterministicSequence(7, 200, 2000)
	ideal := runIdealLRUEvictions(seq, 50)
	clock := runClockStoreEvictions(seq, 50)

	match := intersectionRate(ideal, clock)
	if match < 0.8 {
		t.Fatalf("eviction match rate %.2f < 0.8", match)
	}
	t.Logf("eviction match rate vs ideal LRU: %.4f", match)
}

// --- 测试辅助：确定性序列 / 理想 LRU 参考 / 重合率 ---

// buildDeterministicSequence 生成 80/20 热点访问序列：
// 20% 的热点 key 占 80% 访问，其余冷 key 低频轮换
func buildDeterministicSequence(seed int64, numKeys, length int) []string {
	rng := rand.New(rand.NewSource(seed))
	hotN := numKeys / 5
	seq := make([]string, 0, length)
	for i := 0; i < length; i++ {
		if rng.Float64() < 0.8 {
			seq = append(seq, "hot:"+itoa(rng.Intn(hotN)))
		} else {
			seq = append(seq, "cold:"+itoa(hotN+rng.Intn(numKeys-hotN)))
		}
	}
	return seq
}

// idealLRU 测试专用理想 LRU（map + slice，不追求性能）
type idealLRU struct {
	maxKeys int
	keys    []string          // 表头=最旧，表尾=最新
	set     map[string]bool
}

func newIdealLRU(maxKeys int) *idealLRU {
	return &idealLRU{maxKeys: maxKeys, set: make(map[string]bool)}
}

func (l *idealLRU) touch(key string) (evicted string) {
	if l.set[key] {
		// 移到表尾（最新）
		for i, k := range l.keys {
			if k == key {
				l.keys = append(l.keys[:i], l.keys[i+1:]...)
				break
			}
		}
		l.keys = append(l.keys, key)
		return ""
	}
	if len(l.set) >= l.maxKeys {
		evicted = l.keys[0]
		delete(l.set, evicted)
		l.keys = l.keys[1:]
	}
	l.keys = append(l.keys, key)
	l.set[key] = true
	return evicted
}

// runIdealLRUEvictions 跑理想 LRU，返回被淘汰 key 集合
func runIdealLRUEvictions(seq []string, maxKeys int) map[string]int {
	l := newIdealLRU(maxKeys)
	evictions := make(map[string]int)
	for _, key := range seq {
		if e := l.touch(key); e != "" {
			evictions[e]++
		}
	}
	return evictions
}

// runClockStoreEvictions 用 clockStore 跑同一序列，返回被淘汰 key 集合
// （同包测试直接访问内部字段，通过调用前后 items 快照对比识别被淘汰 key）
func runClockStoreEvictions(seq []string, maxKeys int) map[string]int {
	store := newClockStore[*bucket](options{maxKeys: maxKeys}, func(now time.Time) *bucket {
		return &bucket{tokens: 1, lastRefilled: now}
	})
	now := time.Now()
	evictions := make(map[string]int)
	for _, key := range seq {
		before := make(map[string]bool, len(store.items))
		for k := range store.items {
			before[k] = true
		}
		_, _ = store.getOrCreate(key, now)
		for k := range before {
			if _, ok := store.items[k]; !ok {
				evictions[k]++
			}
		}
	}
	return evictions
}

// itoa 简易 int→string（避免 strconv 依赖污染测试可读性）
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	buf := make([]byte, 0, 8)
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	if neg {
		buf = append([]byte{'-'}, buf...)
	}
	return string(buf)
}

func intersectionRate(a, b map[string]int) float64 {
	if len(a) == 0 {
		return 1
	}
	inter := 0
	for k := range a {
		if _, ok := b[k]; ok {
			inter++
		}
	}
	return float64(inter) / float64(len(a))
}
