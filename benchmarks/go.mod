module github.com/themarchrain/kaka/benchmarks

go 1.25.1

replace github.com/themarchrain/kaka => ../

require (
	github.com/juju/ratelimit v1.0.2
	github.com/themarchrain/kaka v0.0.0-00010101000000-000000000000
	github.com/ulule/limiter/v3 v3.11.2
	go.uber.org/ratelimit v0.3.1
	golang.org/x/time v0.15.0
)

require (
	github.com/benbjohnson/clock v1.3.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
)
