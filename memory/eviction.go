package memory

// EvictionPolicy 定义 maxKeys 达到上限时对新 key 的处理策略
type EvictionPolicy int

const (
	// EvictReject 拒绝新 key，返回 ErrMaxKeysExceeded（默认，与零值一致）
	EvictReject EvictionPolicy = iota
	// EvictLRU 淘汰最久未使用的 key，为新 key 腾位
	EvictLRU
)

// WithEvictionPolicy 设置 maxKeys 达到上限时的策略
// 默认 EvictReject：达到上限后新 key 返回 error
func WithEvictionPolicy(p EvictionPolicy) Option {
	return func(o *options) { o.eviction = p }
}
