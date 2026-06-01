package memory

import (
	"math"
	"time"
)

func durationFromSecondsCeil(seconds float64) time.Duration {
	if seconds <= 0 {
		return 0
	}

	nanos := seconds * float64(time.Second)
	if nanos >= float64(maxDuration) {
		return maxDuration
	}
	rounded := math.Round(nanos)
	if math.Abs(nanos-rounded) < floatEpsilonNanos {
		return time.Duration(rounded)
	}
	return time.Duration(math.Ceil(nanos))
}

const (
	maxDuration       = time.Duration(1<<63 - 1)
	floatEpsilonNanos = 1e-6
)
