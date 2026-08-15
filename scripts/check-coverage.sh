#!/usr/bin/env sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
root=$(CDPATH= cd -- "$script_dir/.." && pwd)
threshold="${COVERAGE_THRESHOLD:-90}"

profile=$(mktemp)
trap 'rm -f "$profile"' EXIT

cd "$root"
go test -count=1 -coverpkg=./... -coverprofile="$profile" ./... >/dev/null
total=$(go tool cover -func="$profile" | awk '/^total:/ {gsub(/%/, "", $3); print $3}')

echo "coverage: ${total}% (threshold: ${threshold}%)"
awk -v total="$total" -v threshold="$threshold" \
  'BEGIN { if (total + 0 < threshold + 0) { printf "FAIL: coverage %.1f%% below threshold %.1f%%\n", total, threshold; exit 1 } }'
