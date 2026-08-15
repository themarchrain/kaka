package layered_test

import (
	"context"
	"fmt"

	"github.com/themarchrain/kaka/layered"
	"github.com/themarchrain/kaka/memory"
)

// ExampleNew composes an in-memory local layer with a remote layer. In
// production the remote layer would be Redis-backed
// (github.com/themarchrain/kaka/redis) so the quota is shared across
// instances; any kaka.Limiter works in either position.
func ExampleNew() {
	local := memory.NewTokenBucket(100, 10)
	remote := memory.NewTokenBucket(100, 10)
	limiter := layered.New(local, remote)

	result, err := limiter.Allow(context.Background(), "user:42")
	if err != nil {
		panic(err)
	}
	fmt.Printf("allowed=%v remaining=%d\n", result.Allowed, result.Remaining)
	// Output:
	// allowed=true remaining=99
}
