#!/usr/bin/env bash
# Runs the search and ranking benchmarks in internal/app and prints a markdown table for a PR:
# ns/op, ns/name (ranking only), B/op and allocs/op per benchmark, with the CPU and Go version.
#
#   scripts/rig/bench-summary.sh [benchtime]           # default benchtime 1s
#   BENCH_RAW=/path/out.txt scripts/rig/bench-summary.sh   # also keep the raw go test output
#
# Go is not installed on maintainers' machines; run it in the shared test box:
#   scripts/boxgo.sh wp05 'bash scripts/rig/bench-summary.sh 1s'
set -euo pipefail

native_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$native_dir"
benchtime="${1:-1s}"

raw="$(go test -run '^$' -bench . -benchmem -benchtime="$benchtime" ./internal/app/)"
if [ -n "${BENCH_RAW:-}" ]; then
  printf '%s\n' "$raw" >"$BENCH_RAW"
fi

cpu="$(printf '%s\n' "$raw" | sed -n 's/^cpu: //p' | head -n 1)"
echo "Benchmarks: \`go test -bench . -benchmem -benchtime=$benchtime ./internal/app/\`"
echo
echo "Environment: ${cpu:-unknown CPU}, $(go env GOOS)/$(go env GOARCH), $(go version | awk '{print $3}'), $(nproc) CPUs"
echo
echo "| Benchmark | ns/op | ns/name | B/op | allocs/op |"
echo "| --- | ---: | ---: | ---: | ---: |"
printf '%s\n' "$raw" | awk '
  /^Benchmark/ {
    name = $1
    sub(/^Benchmark/, "", name)
    sub(/-[0-9]+$/, "", name)
    ns = ""; nsname = ""; bytes = ""; allocs = ""
    for (i = 3; i + 1 <= NF; i += 2) {
      if ($(i + 1) == "ns/op") ns = $i
      else if ($(i + 1) == "ns/name") nsname = $i
      else if ($(i + 1) == "B/op") bytes = $i
      else if ($(i + 1) == "allocs/op") allocs = $i
    }
    printf "| %s | %s | %s | %s | %s |\n", name, ns, nsname, bytes, allocs
  }'
echo
echo "Not measured here: rendering, the palette's view state, icons, the extension host and the"
echo "clipboard watcher. The root search runs Palette.search on the real root view with the"
echo "fixture stores; the application list is 500 fixtures and the files are 50k fixture paths."
