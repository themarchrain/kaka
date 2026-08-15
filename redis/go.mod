module github.com/themarchrain/kaka/redis

go 1.24

replace github.com/themarchrain/kaka => ../

require (
	github.com/redis/go-redis/v9 v9.22.0
	github.com/themarchrain/kaka v0.0.0-00010101000000-000000000000
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
)
