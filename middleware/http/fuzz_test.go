package httpmiddleware

import (
	"math"
	"testing"
	"time"
)

func FuzzRetryAfterSeconds(f *testing.F) {
	f.Add(int64(time.Second))
	f.Add(int64(0))
	f.Add(int64(-time.Second))
	f.Fuzz(func(t *testing.T, nanos int64) {
		d := time.Duration(nanos)
		got := retryAfterSeconds(d)
		if got < 1 {
			t.Fatalf("retryAfterSeconds(%v) = %d < 1", d, got)
		}
		want := int(math.Ceil(d.Seconds()))
		if want < 1 {
			want = 1
		}
		if got != want {
			t.Fatalf("retryAfterSeconds(%v) = %d, want %d", d, got, want)
		}
	})
}
