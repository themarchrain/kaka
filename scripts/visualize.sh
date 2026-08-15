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
    /*) raw_dir=$KAKA_RAW_DIR ;;
    *) raw_dir="$root/$KAKA_RAW_DIR" ;;
esac
charts_dir="$root/docs/benchmarks/charts"
mkdir -p "$raw_dir" "$charts_dir"

full=0
[ "${1:-}" = "--full" ] && full=1

# kind -> (glob pattern, script that produces it)
need_run() {  # $1 = glob pattern, $2 = script
    if ! ls "$raw_dir"/$1 >/dev/null 2>&1; then
        echo "==> missing $1; running $2"
        sh "$root/scripts/$2"
    fi
}

need_run 'benchmark-*.txt'            bench.sh
need_run 'memory-perkey-*.txt'        bench-memory.sh
need_run 'memory-lru-*.txt'           bench-memory.sh
need_run 'correctness-*.txt'          bench-memory.sh
need_run 'lru-*.txt'                  bench-lru.sh
need_run 'loadtest-kaka-*.csv'        loadtest.sh
need_run 'loadtest-xtime-*.csv'       loadtest.sh
need_run 'loadtest-kaka-many-*.csv'   loadtest.sh
need_run 'benchmark-redis-*.txt'      bench-redis.sh
need_run 'benchmark-redis-sweep-*.txt' bench-redis.sh
need_run 'benchmark-layered-*.txt'    bench-layered.sh
need_run 'loadtest-redis-*.csv'       bench-layered.sh
need_run 'loadtest-layered-*.csv'     bench-layered.sh

if [ "$full" = "1" ]; then
    echo "==> --full: rerunning long-term stability (600s)"
    sh "$root/scripts/loadtest-long.sh" 600
fi

echo "==> rendering charts"
python3 "$root/scripts/visualize/main.py" --raw-dir "$raw_dir" --charts-dir "$charts_dir"
