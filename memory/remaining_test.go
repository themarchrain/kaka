package memory

import "testing"

func TestRemainingFloorClampsSmallAndNegativeValues(t *testing.T) {
	cases := []struct {
		name  string
		value float64
		want  int64
	}{
		{name: "negative", value: -0.1, want: 0},
		{name: "zero", value: 0, want: 0},
		{name: "subunit", value: 0.999999, want: 0},
		{name: "unit", value: 1.0, want: 1},
		{name: "multi", value: 3.9, want: 3},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := remainingFloor(tc.value); got != tc.want {
				t.Fatalf("remainingFloor(%v) = %d, want %d", tc.value, got, tc.want)
			}
		})
	}
}
