package memory

import "time"

// Clock 是可注入的时间源。默认使用 time.Now，测试或差分验证可注入 fake clock。
type Clock interface {
	Now() time.Time
}

// clock 为内部别名：保持 memory 包内既有用法的兼容，且与导出 Clock 等价。
type clock = Clock

type realClock struct{}

func (realClock) Now() time.Time {
	return time.Now()
}

// WithClock 注入自定义时钟（默认使用真实时间 time.Now）。
// 供外部包（如 benchmarks 差分验证）注入确定性时间。
func WithClock(c Clock) Option {
	return withClock(c)
}

func withClock(c clock) Option {
	return func(o *options) { o.clock = c }
}
