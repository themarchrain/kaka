#!/usr/bin/env sh
set -eu

race=0
if [ "$#" -gt 1 ]; then
    echo "usage: ./scripts/test.sh [--race]" >&2
    exit 2
elif [ "${1:-}" = "--race" ]; then
    race=1
elif [ "${1:-}" != "" ]; then
    echo "usage: ./scripts/test.sh [--race]" >&2
    exit 2
fi

case "$0" in
    */*) script_dir_path=${0%/*} ;;
    *) script_dir_path=. ;;
esac

script_dir=$(CDPATH= cd -- "$script_dir_path" && pwd)
root=$(CDPATH= cd -- "$script_dir/.." && pwd)

modules="
.
middleware/gin
middleware/http
examples/http-example
examples/gin-example
benchmarks
redis
"

for module in $modules; do
    echo "==> go test -count=1 ./... ($module)"
    (cd "$root/$module" && go test -count=1 ./...)
done

if [ "$race" -eq 1 ]; then
    echo "==> go test -race -count=1 ./... (.)"
    previous_cgo_enabled="${CGO_ENABLED-}"
    had_cgo_enabled=0
    if [ "${CGO_ENABLED+x}" = "x" ]; then
        had_cgo_enabled=1
    fi

    CGO_ENABLED=1
    export CGO_ENABLED

    set +e
    (cd "$root" && go test -race -count=1 ./...)
    status=$?
    set -e

    if [ "$had_cgo_enabled" -eq 1 ]; then
        CGO_ENABLED=$previous_cgo_enabled
        export CGO_ENABLED
    else
        unset CGO_ENABLED
    fi

    exit "$status"
fi
