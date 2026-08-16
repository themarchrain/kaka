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

echo "==> memory footprint per key (kaka vs ulule)"
(cd "$root/benchmarks" && go test -run 'TestMemoryFootprintPerKey' -v -count=1 ./compare/) \
    | tee "$out_dir/memory-perkey-$stamp.txt"

echo "==> memory footprint LRU (mapStore vs lruStore)"
(cd "$root/benchmarks" && go test -run 'TestMemoryFootprintLRU' -v -count=1 ./compare/) \
    | tee "$out_dir/memory-lru-$stamp.txt"

echo "==> correctness verification (differential + invariants, deterministic)"
(cd "$root/benchmarks" && go test -run 'Correctness|PerKeyIsolation' -v -count=1 ./compare/) \
    > "$out_dir/correctness-$stamp.txt"
(cd "$root" && go test -count=1 ./memory/ -run 'Invariants' -v) \
    >> "$out_dir/correctness-$stamp.txt"

echo "saved: memory-perkey-$stamp.txt, memory-lru-$stamp.txt, correctness-$stamp.txt"
