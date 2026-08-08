#!/usr/bin/env sh
set -eu

case "$0" in
    */*) script_dir_path=${0%/*} ;;
    *) script_dir_path=. ;;
esac

script_dir=$(CDPATH= cd -- "$script_dir_path" && pwd)
root=$(CDPATH= cd -- "$script_dir/.." && pwd)
: "${KAKA_RAW_DIR:=docs/benchmarks/raw}"
out_dir="$root/$KAKA_RAW_DIR"
mkdir -p "$out_dir"

stamp=$(date +%Y%m%d-%H%M%S)
out="$out_dir/benchmark-$stamp.txt"

echo "==> running comparison benchmarks (count=3)"
(cd "$root/benchmarks" && go test -run='^$' -bench=. -benchmem -count=3 ./compare/) | tee "$out"

echo "saved: $out"
