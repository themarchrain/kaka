module github.com/themarchrain/kaka/benchmarks

go 1.25.1

replace github.com/themarchrain/kaka => ../

require (
	github.com/juju/ratelimit v1.0.2
	github.com/themarchrain/kaka v0.0.0-00010101000000-000000000000
	golang.org/x/time v0.15.0
)

require gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
