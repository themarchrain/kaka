package memory

import (
	"hash/fnv"
	"testing"
)

// Standard FNV-1a 64 test vectors (from the Wikipedia FNV page).
func TestFNV1a64_FixedVectors(t *testing.T) {
	cases := []struct {
		in   string
		want uint64
	}{
		{"", 0xcbf29ce484222325},       // offset basis
		{"a", 0xaf63dc4c8601ec8c},      // FNV-1a 64 "a"
		{"foobar", 0x85944171f73967e8}, // FNV-1a 64 "foobar"
	}
	for _, tc := range cases {
		if got := fnv1a64(tc.in); got != tc.want {
			t.Fatalf("fnv1a64(%q) = %#x, want %#x", tc.in, got, tc.want)
		}
	}
}

// Matches the stdlib hash/fnv byte-for-byte on arbitrary input.
func TestFNV1a64_MatchesStdlib(t *testing.T) {
	inputs := []string{"user:1", "user:12345", "a/b/c-d_e.f", "中文key:你好", ""}
	for _, in := range inputs {
		h := fnv.New64a()
		h.Write([]byte(in))
		if got := fnv1a64(in); got != h.Sum64() {
			t.Fatalf("fnv1a64(%q) = %#x, stdlib = %#x", in, got, h.Sum64())
		}
	}
}

// Determinism: the same input always yields the same output (stateless).
func TestFNV1a64_Deterministic(t *testing.T) {
	if fnv1a64("user:42") != fnv1a64("user:42") {
		t.Fatal("fnv1a64 must be deterministic")
	}
}
