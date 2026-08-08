package memory

import "testing"

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
