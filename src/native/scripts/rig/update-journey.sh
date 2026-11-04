#!/usr/bin/env bash
# Runs the update journeys (J24) on a rig desktop: builds the running app (2.0.0-next.0) and the update (2.0.1), both
# linked with the rig's fake update public key and a feed on loopback, then runs the journeys against them.
#
#   scripts/rig/entrypoint.sh bash scripts/rig/update-journey.sh
#
# NVM_RIG_UPDATE_DIR (default: the temp directory) holds the two builds, NVM_RIG_HOME (default: a fresh HOME per launch)
# is the app's HOME. NVM_RIG_UPDATE_PORT (default 18765) is the
# loopback port of the feed. The rig key is fake: its seed is in internal/rigtest/journey_update_test.go, and no
# release is signed with it.
set -euo pipefail

native_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$native_dir"
dir="${NVM_RIG_UPDATE_DIR:-${TMPDIR:-/tmp}/nvm-rig-update}"
port="${NVM_RIG_UPDATE_PORT:-18765}"
pub="${NVM_RIG_UPDATE_PUBKEY:?set NVM_RIG_UPDATE_PUBKEY to the base64 public key of the fake rig key}"
feed="http://127.0.0.1:${port}/update-linux-amd64.json"
mygo=github.com/egoist/mygo

rm -rf "$dir" && mkdir -p "$dir/old" "$dir/new"
if [ -n "${NVM_RIG_HOME:-}" ]; then rm -rf "$NVM_RIG_HOME" && mkdir -p "$NVM_RIG_HOME"; fi
build() {
  local version=$1 out=$2
  go build -tags automation -o "$out" -ldflags "-X $mygo.production=1 -X $mygo.packageName=Nevermind -X $mygo.packageVersion=$version -X $mygo.packageIdentifier=com.pablopunk.nvm.next -X $mygo.packageUpdateFeed=$feed -X $mygo.packageUpdateKey=$pub" ./cmd/nvm
}
build 2.0.0-next.0 "$dir/old/nevermind-next"
build 2.0.1 "$dir/new/nevermind-next"

export NVM_RIG_UPDATE_OLD_BIN="$dir/old/nevermind-next"
export NVM_RIG_UPDATE_BIN_NEW="$dir/new/nevermind-next"
export NVM_RIG_UPDATE_PORT="$port"
export NVM_RIG_HOME="${NVM_RIG_HOME:-}"
export NVM_RIG_ARTIFACTS="${NVM_RIG_ARTIFACTS:-${RIG_ARTIFACTS:-$dir}/tests}"
go test -count=1 -tags rig ./internal/rigtest/ -run 'TestJourneyUpdate' -v -timeout 15m
