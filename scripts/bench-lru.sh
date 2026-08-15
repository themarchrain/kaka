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
out="$out_dir/lru-$stamp.txt"

# 05 report methodology: each LRU benchmark must run in its own process;
# a shared-process full run inflates LRU numbers 3-4x (GC heap growth).
for name in 'MaxKeysReject$' 'LRUEviction$' 'LRUThrash$' 'LRUEvictionParallel$'; do
    echo "==> lru benchmark: $name (separate process, count=3)"
    (cd "$root/benchmarks" && go test -run='^$' -bench "$name" -benchmem -count=3 ./compare/) \
        | tee -a "$out"
done

echo "saved: $out"
