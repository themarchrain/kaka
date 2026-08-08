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
bin="$root/benchmarks/.bench-server-bin"

# 构建一次，直接运行二进制（go run 的后台子进程 kill 不干净，会残留占用端口）
build_server() {
    (cd "$root/benchmarks" && go build -o "$bin" ./cmd/bench-server)
}

# 启动服务并等待就绪；进程立即退出（端口被占）或迟迟不就绪时中止，
# 避免 hey 打到错误的服务（如残留的旧进程）
start_server() {
    impl=$1
    shift
    "$bin" "$@" &
    pid=$!
    trap 'kill $pid 2>/dev/null || true; rm -f "$bin"' EXIT
    sleep 1
    if ! kill -0 "$pid" 2>/dev/null; then
        echo "error: bench-server ($impl) exited immediately (port $port already in use?)" >&2
        exit 1
    fi
    i=0
    while ! curl -sf "http://127.0.0.1:$port/metrics" >/dev/null 2>&1; do
        i=$((i + 1))
        if [ "$i" -ge 8 ]; then
            echo "error: bench-server ($impl) not ready within 8s (port $port?)" >&2
            exit 1
        fi
        sleep 1
    done
    echo "==> bench-server ($impl) up"
}

stop_server() {
    kill $pid 2>/dev/null || true
    wait $pid 2>/dev/null || true
    trap - EXIT
    rm -f "$bin"
}

run_impl() {
    impl=$1
    echo "==> building bench-server"
    build_server
    echo "==> starting bench-server ($impl)"
    start_server "$impl" --impl "$impl" --addr "127.0.0.1:$port"

    out="$out_dir/loadtest-$impl-$stamp.csv"
    echo "==> hey -n 100000 -c 100 ($impl)"
    hey -n 100000 -c 100 -m GET -o csv "http://127.0.0.1:$port/api/test" > "$out"
    echo "saved: $out"

    stop_server
}

# 多 key 场景：Kaka many 模式（按连接哈希分入 10000 个 key 池）
run_impl_many() {
    impl=$1
    echo "==> building bench-server"
    build_server
    echo "==> starting bench-server ($impl, keymode=many)"
    start_server "$impl" --impl "$impl" --keymode many --addr "127.0.0.1:$port"

    out="$out_dir/loadtest-$impl-many-$stamp.csv"
    echo "==> hey -n 100000 -c 100 (many-key, $impl)"
    hey -n 100000 -c 100 -m GET -o csv "http://127.0.0.1:$port/api/test" > "$out"
    echo "saved: $out"

    stop_server
}

run_impl kaka
run_impl xtime
run_impl_many kaka
