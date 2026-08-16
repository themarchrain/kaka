package memory

import (
	"strconv"
	"testing"
)

func TestEvictionPolicy_DefaultIsReject(t *testing.T) {
	opts := defaultOptions()
	if opts.eviction != EvictReject {
		t.Fatalf("expected default eviction policy EvictReject, got %v", opts.eviction)
	}
}

func TestWithEvictionPolicy_SetsPolicy(t *testing.T) {
	opts := defaultOptions()
	WithEvictionPolicy(EvictLRU)(&opts)
	if opts.eviction != EvictLRU {
		t.Fatalf("expected EvictLRU, got %v", opts.eviction)
	}
}

func TestOptions_ValidateRejectsInvalidPolicy(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for invalid eviction policy")
		}
	}()
	opts := options{eviction: EvictionPolicy(99), clock: realClock{}}
	opts.validate()
}

func TestWithShardCount_SetsValue(t *testing.T) {
	opts := defaultOptions()
	WithShardCount(8)(&opts)
	if opts.shards != 8 {
		t.Fatalf("expected shards=8, got %d", opts.shards)
	}
}

func TestWithShardCount_DefaultIs64(t *testing.T) {
	opts := defaultOptions()
	if opts.shards != defaultShardCount {
		t.Fatalf("expected default shards=%d, got %d", defaultShardCount, opts.shards)
	}
}

func TestOptions_ValidateAcceptsPowerOfTwoShardCounts(t *testing.T) {
	for _, n := range []int{1, 2, 64, 1024} {
		opts := defaultOptions()
		WithShardCount(n)(&opts)
		opts.validate() // must not panic
	}
}

func TestOptions_ValidateRejectsInvalidShardCounts(t *testing.T) {
	for _, n := range []int{0, -1, 3, 15, 1025, 2048} {
		t.Run(strconv.Itoa(n), func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Fatalf("expected panic for shardCount=%d", n)
				}
			}()
			opts := defaultOptions()
			WithShardCount(n)(&opts)
			opts.validate()
		})
	}
}
