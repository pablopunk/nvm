#!/usr/bin/env bash
# Builds the app with the automation plugin and runs the rig journeys against it. Runs inside a rig
# desktop (scripts/rig/entrypoint.sh) from any directory; arguments are passed to go test.
#
#   scripts/rig/entrypoint.sh bash scripts/rig/run-journeys.sh -run Journey -v
#
# NVM_RIG_BIN (default: a folder this run creates) is where the app is built; NVM_RIG_ARTIFACTS (default:
# $RIG_ARTIFACTS/tests, else a folder this run creates) receives the per-test screenshots and logs. Each run owns
# its folders, so two runs on one machine never overwrite each other's binary or logs.
set -euo pipefail

native_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$native_dir"
run_dir="$(mktemp -d "${TMPDIR:-/tmp}/nvm-rig-run.XXXXXX")"
export NVM_RIG_BIN="${NVM_RIG_BIN:-$run_dir/nvm-rig}"
export NVM_RIG_ARTIFACTS="${NVM_RIG_ARTIFACTS:-${RIG_ARTIFACTS:+$RIG_ARTIFACTS/tests}}"

# The container runs as root, so its files in the mounted artifacts folder belong to root on the host. The runner
# that uploads them cannot read a file that is not world-readable, so the upload fails. Every exit makes them readable,
# and removes the run's binary folder.
finish_run() {
  local dir
  for dir in "$NVM_RIG_ARTIFACTS" "${RIG_ARTIFACTS:-}"; do
    if [ -n "$dir" ] && [ -d "$dir" ]; then
      chmod -R a+rX "$dir" || true
    fi
  done
  rm -rf "$run_dir"
}
trap finish_run EXIT

tests=("$@")
if [ "${#tests[@]}" -eq 0 ]; then
  tests=(-run Journey -v)
fi

go build -tags automation -o "$NVM_RIG_BIN" ./cmd/nvm
go test -count=1 -tags rig ./internal/rigtest/ "${tests[@]}"
