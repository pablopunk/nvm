#!/usr/bin/env bash
# Measures the performance budgets (PLAN.md section 3.2, B1-B7) on the rig and compares them with
# testdata/budgets.json and the stored testdata/perf-baseline.json. Runs inside a rig desktop, like
# run-journeys.sh; the shared test box and the rig container both qualify.
#
#   RIG_DISPLAY=:96 RIG_ARTIFACTS=/root/wp/wp05-logs scripts/rig/entrypoint.sh bash scripts/rig/perf.sh
#
#   --soak=<duration>   length of the B7 soak loop (default 2m; PLAN.md's full run is 30m)
#   --warn-only         print violations as warnings and exit 0 (the report must still be written)
#   --write-baseline    store this run's report as testdata/perf-baseline.json
#
# Environment: NVM_PERF_LABEL names the machine in the report; the renderer is read from the app's log. Outputs go to
# $RIG_ARTIFACTS/perf (default: the temp directory): perf-report.json, perf-test.log, and the app
# logs of each launch under tests/.
set -euo pipefail

native_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$native_dir"

soak="2m"
warn_only=0
write_baseline=0
while [ "$#" -gt 0 ]; do
  case "$1" in
    --soak=*) soak="${1#--soak=}" ;;
    --soak) soak="${2:?--soak needs a duration}"; shift ;;
    --warn-only) warn_only=1 ;;
    --write-baseline) write_baseline=1 ;;
    -h|--help) sed -n '2,14p' "${BASH_SOURCE[0]}" >&2; exit 0 ;;
    *) echo "perf.sh: unknown argument $1" >&2; exit 2 ;;
  esac
  shift
done

if [ -z "${DISPLAY:-}" ]; then
  echo "perf.sh: DISPLAY is empty; run inside scripts/rig/entrypoint.sh" >&2
  exit 2
fi

artifacts="${RIG_ARTIFACTS:-${TMPDIR:-/tmp}/nvm-rig-artifacts}/perf"
mkdir -p "$artifacts"
tmp_bin="$(mktemp -d "${TMPDIR:-/tmp}/perf-bin.XXXXXX")"
trap 'rm -rf "$tmp_bin"' EXIT

# B6 is the default build, which is what ships. The rig binary carries the automation plugin and
# the counters endpoint, so it is what the interactive measurements run.
echo "perf: building the default binary (B6) and the rig binary (automation,perfdebug)"
go build -o "$tmp_bin/nvm" ./cmd/nvm
go build -tags automation,perfdebug -o "$tmp_bin/nvm-rig" ./cmd/nvm

report="$artifacts/perf-report.json"
rm -f "$report"

export NVM_RIG_BIN="$tmp_bin/nvm-rig"
export NVM_PERF_SHIPPED_BIN="$tmp_bin/nvm"
export NVM_PERF_REPORT="$report"
export NVM_RIG_ARTIFACTS="$artifacts/tests"
export NVM_PERF_LABEL="${NVM_PERF_LABEL:-box}"

echo "perf: measuring on $DISPLAY (soak $soak), log in $artifacts/perf-test.log"
test_status=0
NVM_RIG_PERF=1 go test -count=1 -tags rig -run '^TestPerfRig$' -timeout 60m -v \
  ./internal/rigtest/ -args -soak="$soak" 2>&1 | tee "$artifacts/perf-test.log" || test_status=$?

if [ ! -s "$report" ]; then
  echo "perf.sh: no report was written; the measurements did not complete" >&2
  exit 1
fi

compare_args=(-budgets testdata/budgets.json -baseline testdata/perf-baseline.json -report "$report")
if [ "$warn_only" -eq 1 ]; then
  compare_args+=(-warn-only)
fi
compare_status=0
go run ./cmd/perfcompare "${compare_args[@]}" || compare_status=$?

if [ "$write_baseline" -eq 1 ]; then
  cp "$report" testdata/perf-baseline.json
  echo "perf: baseline written to testdata/perf-baseline.json from this run"
fi

if [ "$warn_only" -eq 1 ]; then
  exit 0
fi
if [ "$test_status" -ne 0 ]; then
  echo "perf.sh: the measurements reported test failures (see $artifacts/perf-test.log)" >&2
  exit "$test_status"
fi
exit "$compare_status"
