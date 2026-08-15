package memory

import (
	"encoding/binary"
	"math"
	"strings"
	"testing"
	"time"
)

// fuzzInt64 converts up to 8 bytes of fuzz input into a bounded float so
// float-to-int64 conversions in the functions under test stay well-defined.
func fuzzInt64(data []byte, divisor float64) float64 {
	var buf [8]byte
	copy(buf[:], data)
	return float64(int64(binary.LittleEndian.Uint64(buf[:]))) / divisor
}

func FuzzRemainingFloor(f *testing.F) {
	f.Add([]byte{1, 0, 0, 0, 0, 0, 0, 0}) // 1 / 1e6
	f.Fuzz(func(t *testing.T, data []byte) {
		value := fuzzInt64(data, 1e6)
		got := remainingFloor(value)
		if got < 0 {
			t.Fatalf("remainingFloor(%v) = %d < 0", value, got)
		}
		if value <= 0 && got != 0 {
			t.Fatalf("remainingFloor(%v) = %d, want 0 for non-positive input", value, got)
		}
		if value > 0 && got != int64(value) {
			t.Fatalf("remainingFloor(%v) = %d, want %d", value, got, int64(value))
		}
	})
}

func FuzzDurationFromSecondsCeil(f *testing.F) {
	f.Add([]byte{1, 0, 0, 0, 0, 0, 0, 0, 0}) // +1e9 ns → 1s
	f.Fuzz(func(t *testing.T, data []byte) {
		seconds := fuzzInt64(data, 1e9)
		if len(data) >= 9 && data[8]&1 == 1 {
			seconds = -seconds
		}
		d := durationFromSecondsCeil(seconds)
		if d < 0 {
			t.Fatalf("durationFromSecondsCeil(%v) = %v < 0", seconds, d)
		}
		if seconds <= 0 && d != 0 {
			t.Fatalf("durationFromSecondsCeil(%v) = %v, want 0 for non-positive input", seconds, d)
		}
		if seconds > 0 {
			floorNanos := math.Floor(seconds * float64(time.Second))
			if d < time.Duration(floorNanos) {
				t.Fatalf("durationFromSecondsCeil(%v) = %v < floor(%v)", seconds, d, floorNanos)
			}
		}
	})
}

func FuzzValidateKey(f *testing.F) {
	f.Add("user:42")
	f.Add("")
	f.Add("   ")
	f.Fuzz(func(t *testing.T, key string) {
		err := validateKey(key)
		wantErr := strings.TrimSpace(key) == ""
		if (err == ErrInvalidKey) != wantErr {
			t.Fatalf("validateKey(%q) err=%v, want ErrInvalidKey=%v", key, err, wantErr)
		}
	})
}
