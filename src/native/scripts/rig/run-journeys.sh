#!/usr/bin/env bash
# Builds the app with the automation plugin and runs the rig journeys against it. Runs inside a rig
# desktop (scripts/rig/entrypoint.sh) from any directory; arguments are passed to go test.
#
#   scripts/rig/entrypoint.sh bash scripts/rig/run-journeys.sh -run Journey -v
#
# NVM_RIG_BIN (default: the temp directory) is where the app is built; NVM_RIG_ARTIFACTS (default:
# $RIG_ARTIFACTS/tests, else the temp directory) receives the per-test screenshots and logs.
set -euo pipefail

native_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$native_dir"
export NVM_RIG_BIN="${NVM_RIG_BIN:-${TMPDIR:-/tmp}/nvm-rig}"
export NVM_RIG_ARTIFACTS="${NVM_RIG_ARTIFACTS:-${RIG_ARTIFACTS:-${TMPDIR:-/tmp}/nvm-rig-artifacts}/tests}"

tests=("$@")
if [ "${#tests[@]}" -eq 0 ]; then
  tests=(-run Journey -v)
fi

go build -tags automation -o "$NVM_RIG_BIN" ./cmd/nvm
go test -count=1 -tags rig ./internal/rigtest/ "${tests[@]}"
