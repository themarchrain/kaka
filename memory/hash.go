package memory

// fnv1a64 is an inline FNV-1a 64-bit hash (byte-wise loop; zero allocation,
// deterministic). The hash/fnv stdlib interface is avoided on purpose:
// interface dispatch and escape analysis would add cost on the hot path,
// where this is called once per request.
func fnv1a64(s string) uint64 {
	const (
		offset64 = 14695981039346656037
		prime64  = 1099511628211
	)
	h := uint64(offset64)
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= prime64
	}
	return h
}
