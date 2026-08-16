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
out="$out_dir/benchmark-multikey-$stamp.txt"

echo "==> running multi-key concurrency sweep (benchtime=3s, count=1)"
(cd "$root/benchmarks" && go test -run='^$' -bench '^BenchmarkMultiKeyParallel' -benchmem -benchtime=3s -count=1 ./compare/) | tee "$out"

echo "saved: $out"
