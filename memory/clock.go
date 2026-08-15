package memory

import "time"

// Clock is an injectable time source. It defaults to time.Now; tests and differential
// verification can inject a fake clock.
type Clock interface {
	Now() time.Time
}

// clock 为内部别名：保持 memory 包内既有用法的兼容，且与导出 Clock 等价。
type clock = Clock

type realClock struct{}

func (realClock) Now() time.Time {
	return time.Now()
}

// WithClock injects a custom clock (defaults to real time via time.Now).
// It lets external packages, such as the benchmark differential tests, inject deterministic time.
func WithClock(c Clock) Option {
	return withClock(c)
}

func withClock(c clock) Option {
	return func(o *options) { o.clock = c }
}
