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

echo "==> layered direct benchmarks (count=3)"
(cd "$root/benchmarks" && go test -run='^$' \
    -bench 'Layered|RedisTokenBucket|RedisRejectHeavy|KakaTokenBucketRejectPath' \
    -benchmem -count=3 ./compare/) | tee "$out_dir/benchmark-layered-$stamp.txt"

if ! command -v hey >/dev/null 2>&1; then
    echo "hey not found; skipping end-to-end HTTP comparison" >&2
    exit 0
fi

echo "==> end-to-end: bench-server redis (:8082) + layered (:8083), hey 10s x 64"
(cd "$root/benchmarks" && go build -o "$root/benchmarks/.bench-server-bin" ./cmd/bench-server)
bin="$root/benchmarks/.bench-server-bin"

"$bin" --impl redis --addr 127.0.0.1:8082 &
pid_redis=$!
"$bin" --impl layered --addr 127.0.0.1:8083 &
pid_layered=$!
trap 'kill $pid_redis $pid_layered 2>/dev/null || true; rm -f "$bin"' EXIT
sleep 2

hey -n 20000 -c 64 -z 10s -o csv "http://127.0.0.1:8082/api/test" > "$out_dir/loadtest-redis-$stamp.csv" &
hey1=$!
hey -n 20000 -c 64 -z 10s -o csv "http://127.0.0.1:8083/api/test" > "$out_dir/loadtest-layered-$stamp.csv" &
hey2=$!
wait $hey1 $hey2

kill $pid_redis $pid_layered 2>/dev/null || true
wait $pid_redis $pid_layered 2>/dev/null || true
trap - EXIT
rm -f "$bin"

echo "saved: benchmark-layered-$stamp.txt, loadtest-redis-$stamp.csv, loadtest-layered-$stamp.csv"
