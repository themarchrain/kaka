#!/usr/bin/env sh
set -eu

case "$0" in
    */*) script_dir_path=${0%/*} ;;
    *) script_dir_path=. ;;
esac

script_dir=$(CDPATH= cd -- "$script_dir_path" && pwd)
root=$(CDPATH= cd -- "$script_dir/.." && pwd)
: "${KAKA_RAW_DIR:=docs/benchmarks/raw}"
case "$KAKA_RAW_DIR" in
    /*) out_dir=$KAKA_RAW_DIR ;;
    *) out_dir="$root/$KAKA_RAW_DIR" ;;
esac
mkdir -p "$out_dir"

stamp=$(date +%Y%m%d-%H%M%S)
export REDIS_ADDR="${REDIS_ADDR:-127.0.0.1:6379}"

echo "==> redis baseline + probes (count=1, benchtime=3s)"
# NOTE: 'BenchmarkRedis' does not match BenchmarkUluleRedis*, so both
# patterns are needed; Layered* benchmarks are intentionally excluded here.
(cd "$root/benchmarks" && go test -run='^$' -bench 'BenchmarkRedis|BenchmarkUluleRedis' \
    -benchmem -count=1 -benchtime=3s ./compare/) | tee "$out_dir/benchmark-redis-$stamp.txt"

echo "==> redis concurrency sweep (no -benchmem; per-subbench QPS from ns/op)"
(cd "$root/benchmarks" && go test -run='^$' -bench 'BenchmarkRedisConcurrencySweep' \
    -count=1 -benchtime=3s ./compare/) | tee "$out_dir/benchmark-redis-sweep-$stamp.txt"

echo "saved: benchmark-redis-$stamp.txt, benchmark-redis-sweep-$stamp.txt"
