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

if ! command -v hey >/dev/null 2>&1; then
    echo "hey not found. Install with: go install github.com/rakyll/hey@latest" >&2
    exit 1
fi

stamp=$(date +%Y%m%d-%H%M%S)
port=8081
duration=${1:-600}     # 默认 10 分钟
interval=10            # 每 10 秒采样一次

echo "==> building bench-server"
(cd "$root/benchmarks" && go build -o "$root/benchmarks/.bench-server-bin" ./cmd/bench-server)
echo "==> starting bench-server (kaka, keymode=many, maxkeys=100000, keyttl=60s)"
"$root/benchmarks/.bench-server-bin" --impl kaka --keymode many \
    --maxkeys 100000 --keyttl 60s --addr "127.0.0.1:$port" &
pid=$!
trap 'kill $pid 2>/dev/null || true; rm -f "$root/benchmarks/.bench-server-bin"' EXIT
sleep 1
if ! kill -0 "$pid" 2>/dev/null; then
    echo "error: bench-server exited immediately (port $port already in use?)" >&2
    exit 1
fi
i=0
while ! curl -sf "http://127.0.0.1:$port/metrics" >/dev/null 2>&1; do
    i=$((i + 1))
    if [ "$i" -ge 8 ]; then
        echo "error: bench-server failed to start within 8s (port $port busy?)" >&2
        exit 1
    fi
    sleep 1
done
echo "==> bench-server up"

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
