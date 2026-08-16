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
extra_args=""
for a in "$@"; do
    if [ "$a" = "--full" ]; then
        full=1
    else
        extra_args="$extra_args $a"
    fi
done

# kind -> (glob pattern, script that produces it)
need_run() {  # $1 = glob pattern, $2 = script
    if ! ls "$raw_dir"/$1 >/dev/null 2>&1; then
        echo "==> missing $1; running $2"
        sh "$root/scripts/$2"
    fi
}

# Globs mirror main.py's KINDS: timestamp-anchored so a sibling kind
# (benchmark-redis-*, benchmark-layered-*, loadtest-kaka-many-*) never
# satisfies a missing-data check for another kind.
need_run 'benchmark-[0-9]*.txt'       bench.sh
need_run 'memory-perkey-*.txt'        bench-memory.sh
need_run 'memory-lru-*.txt'           bench-memory.sh
need_run 'correctness-*.txt'          bench-memory.sh
need_run 'lru-*.txt'                  bench-lru.sh
need_run 'loadtest-kaka-[0-9]*.csv'   loadtest.sh
need_run 'loadtest-xtime-*.csv'       loadtest.sh
need_run 'loadtest-kaka-many-*.csv'   loadtest.sh
need_run 'benchmark-redis-[0-9]*.txt' bench-redis.sh
need_run 'benchmark-redis-sweep-*.txt' bench-redis.sh
need_run 'benchmark-layered-*.txt'    bench-layered.sh
need_run 'loadtest-redis-*.csv'       bench-layered.sh
need_run 'loadtest-layered-*.csv'     bench-layered.sh
need_run 'benchmark-multikey-[0-9]*.txt' bench-multikey.sh

if [ "$full" = "1" ]; then
    echo "==> --full: rerunning long-term stability (600s)"
    sh "$root/scripts/loadtest-long.sh" 600
fi

# Resolve a working Python interpreter AND verify dependencies in one pass.
# On Windows the `python3` shim from the Microsoft Store is unreliable (its
# behavior varies with the calling environment), so each candidate must prove
# itself by importing matplotlib+pandas; a broken candidate falls through.
PY="${PYTHON:-}"
found_interp=""
if [ -z "$PY" ]; then
    for cand in python3 python; do
        if command -v "$cand" >/dev/null 2>&1; then
            found_interp=1
            if "$cand" -c "import matplotlib, pandas" >/dev/null 2>&1; then
                PY="$cand"
                break
            fi
        fi
    done
fi

if [ -n "$PY" ]; then
    # Explicit PYTHON (or an env-provided interpreter) still needs its deps.
    if ! "$PY" -c "import matplotlib, pandas" >/dev/null 2>&1; then
        echo "error: Python dependencies missing for interpreter $PY (matplotlib, pandas)" >&2
        echo "  install into the current interpreter:  $PY -m pip install -r \"$root/scripts/visualize/requirements.txt\"" >&2
        echo "  or use an isolated venv to avoid touching global site-packages:" >&2
        echo "    python -m venv \"$root/scripts/visualize/.venv\" && \"$root/scripts/visualize/.venv/Scripts/python\" -m pip install -r \"$root/scripts/visualize/requirements.txt\"" >&2
        echo "  (then set PYTHON to the venv interpreter and rerun this script)" >&2
        exit 1
    fi
fi

if [ -z "$PY" ] && [ -n "$found_interp" ]; then
    echo "error: no usable Python found — every candidate failed the dependency check" >&2
    echo "  install into the current interpreter:  python -m pip install -r \"$root/scripts/visualize/requirements.txt\"" >&2
    echo "  or use an isolated venv to avoid touching global site-packages:" >&2
    echo "    python -m venv \"$root/scripts/visualize/.venv\" && \"$root/scripts/visualize/.venv/Scripts/python\" -m pip install -r \"$root/scripts/visualize/requirements.txt\"" >&2
    echo "  (then set PYTHON to the venv interpreter and rerun this script)" >&2
    exit 1
fi

if [ -z "$PY" ]; then
    echo "error: no Python interpreter found (set PYTHON to one)" >&2
    exit 1
fi

echo "==> rendering charts"
# shellcheck disable=SC2086 - extra_args is a space-separated flag list (e.g. --only fig14,fig15)
"$PY" "$root/scripts/visualize/main.py" --raw-dir "$raw_dir" --charts-dir "$charts_dir" $extra_args
