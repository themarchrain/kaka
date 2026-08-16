package compare

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/themarchrain/kaka/memory"
)

// BenchmarkMultiKeyParallel measures limiter-level multi-key concurrent
// throughput. g = goroutines, k = distinct keys in the working set; the key
// distribution (w+i*g)%k spreads workers across all keys. Sub-benchmark
// names are g<g>k<k> (no dashes, so the viz parser's parallel-N split never
// misfires; go test appends the -<GOMAXPROCS> suffix).
func BenchmarkMultiKeyParallel(b *testing.B) {
	ctx := context.Background()
	for _, g := range []int{1, 8, 32, 64} {
		for _, k := range []int{8, 64, 1024} {
			b.Run(fmt.Sprintf("g%dk%d", g, k), func(b *testing.B) {
				// Capacity/rate are huge and maxKeys far exceeds the key set:
				// the subject under test is store concurrency, not rate
				// limiting itself. Keys are pre-generated so per-iteration
				// Sprintf allocation noise is excluded from the measurement.
				limiter := memory.NewTokenBucket(1e9, 1e9, memory.WithMaxKeys(k*10))
				keys := make([]string, k)
				for i := range keys {
					keys[i] = fmt.Sprintf("user:%d", i)
				}
				b.ReportAllocs()
				b.ResetTimer()

				per := b.N / g
				var wg sync.WaitGroup
				for w := 0; w < g; w++ {
					wg.Add(1)
					go func(w int) {
						defer wg.Done()
						for i := 0; i < per; i++ {
							if _, err := limiter.Allow(ctx, keys[(w+i*g)%k]); err != nil {
								b.Fatal(err)
							}
						}
					}(w)
				}
				wg.Wait()
			})
		}
	}
}
