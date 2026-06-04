package memory

import (
	"math"
	"testing"
	"time"
)

func TestIsFinite(t *testing.T) {
	cases := []struct {
		name  string
		value float64
		want  bool
	}{
		{name: "zero", value: 0, want: true},
		{name: "negative", value: -1, want: true},
		{name: "positive", value: 1, want: true},
		{name: "NaN", value: math.NaN(), want: false},
		{name: "+Inf", value: math.Inf(1), want: false},
		{name: "-Inf", value: math.Inf(-1), want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isFinite(tc.value); got != tc.want {
				t.Fatalf("isFinite(%v) = %t, want %t", tc.value, got, tc.want)
			}
		})
	}
}

func TestDurationFromSecondsCeil(t *testing.T) {
	cases := []struct {
		name    string
		seconds float64
		want    time.Duration
	}{
		{name: "negative", seconds: -0.1, want: 0},
		{name: "zero", seconds: 0, want: 0},
		{name: "sub nanosecond", seconds: 0.5e-9, want: time.Nanosecond},
		{name: "exact nanosecond", seconds: 1e-9, want: time.Nanosecond},
		{name: "fractional nanosecond", seconds: 1.1e-9, want: 2 * time.Nanosecond},
		{name: "exact millisecond", seconds: 0.001, want: time.Millisecond},
		{name: "huge", seconds: 1e300, want: maxDuration},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := durationFromSecondsCeil(tc.seconds); got != tc.want {
				t.Fatalf("durationFromSecondsCeil(%v) = %v, want %v", tc.seconds, got, tc.want)
			}
		})
	}
}
