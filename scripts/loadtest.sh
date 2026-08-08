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

run_impl() {
    impl=$1
    echo "==> starting bench-server ($impl)"
    (cd "$root/benchmarks" && go run ./cmd/bench-server --impl "$impl" --addr "127.0.0.1:$port") &
    pid=$!
    trap 'kill $pid 2>/dev/null || true' EXIT
    sleep 3

    out="$out_dir/loadtest-$impl-$stamp.csv"
    echo "==> hey -n 100000 -c 100 ($impl)"
    hey -n 100000 -c 100 -m GET -o csv "http://127.0.0.1:$port/api/test" > "$out"
    echo "saved: $out"

    kill $pid 2>/dev/null || true
    wait $pid 2>/dev/null || true
    trap - EXIT
}

# 多 key 场景：Kaka many 模式（按连接哈希分入 10000 个 key 池）
run_impl_many() {
    impl=$1
    echo "==> starting bench-server ($impl, keymode=many)"
    (cd "$root/benchmarks" && go run ./cmd/bench-server --impl "$impl" --keymode many --addr "127.0.0.1:$port") &
    pid=$!
    trap 'kill $pid 2>/dev/null || true' EXIT
    sleep 3

    out="$out_dir/loadtest-$impl-many-$stamp.csv"
    echo "==> hey -n 100000 -c 100 (many-key, $impl)"
    hey -n 100000 -c 100 -m GET -o csv "http://127.0.0.1:$port/api/test" > "$out"
    echo "saved: $out"

    kill $pid 2>/dev/null || true
    wait $pid 2>/dev/null || true
    trap - EXIT
}

run_impl kaka
run_impl xtime
run_impl_many kaka
