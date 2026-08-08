#!/usr/bin/env sh
set -eu

case "$0" in
    */*) script_dir_path=${0%/*} ;;
    *) script_dir_path=. ;;
esac

script_dir=$(CDPATH= cd -- "$script_dir_path" && pwd)
root=$(CDPATH= cd -- "$script_dir/.." && pwd)
out_dir="$root/docs/superpowers/benchmarks/raw"
mkdir -p "$out_dir"

if ! command -v hey >/dev/null 2>&1; then
    echo "hey not found. Install with: go install github.com/rakyll/hey@latest" >&2
    exit 1
fi

stamp=$(date +%Y%m%d-%H%M%S)
port=8081
duration=${1:-600}     # 默认 10 分钟
interval=10            # 每 10 秒采样一次

echo "==> starting bench-server (kaka, keymode=many, maxkeys=100000, keyttl=60s)"
(cd "$root/benchmarks" && go run ./cmd/bench-server --impl kaka --keymode many \
    --maxkeys 100000 --keyttl 60s --addr "127.0.0.1:$port") &
pid=$!
trap 'kill $pid 2>/dev/null || true' EXIT
sleep 3

mem_out="$out_dir/longterm-mem-$stamp.csv"
hey_out="$out_dir/longterm-hey-$stamp.csv"
echo "ts,heapAlloc,heapObjects,numGC,goroutines" > "$mem_out"

echo "==> long-term load test: ${duration}s, sampling every ${interval}s"
(start=$SECONDS
 while [ $((SECONDS - start)) -lt "$duration" ]; do
     ts=$((SECONDS - start))
     line=$(curl -s "http://127.0.0.1:$port/metrics" | tr -d '\n')
     echo "ts=$ts $line" | sed 's/{//;s/}//;s/"//g;s/:/=/g;s/,/ /g' >> "$mem_out"
     sleep "$interval"
 done) &

sampler_pid=$!
hey -z "${duration}s" -c 100 -m GET -o csv "http://127.0.0.1:$port/api/test" > "$hey_out" || true
wait $sampler_pid 2>/dev/null || true

echo "saved: $mem_out"
echo "saved: $hey_out"

kill $pid 2>/dev/null || true
wait $pid 2>/dev/null || true
trap - EXIT
