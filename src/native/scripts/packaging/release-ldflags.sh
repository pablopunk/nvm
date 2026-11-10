#!/usr/bin/env bash
# Prints the -ldflags that mygo build reads from MYGO_LDFLAGS. Every packaged build links isPackagedBuild; only
# RELEASE=true also links the legacy release marker. Analytics consent is enforced by the app.
set -euo pipefail

package=github.com/pablopunk/nvm/native/internal/app
flags="-X $package.isPackagedBuild=true"
config="$(cd "$(dirname "$0")/../.." && pwd)/mygo.json"
version="$(jq -er '.version | strings | select(test("^[A-Za-z0-9][A-Za-z0-9.+_-]*$"))' "$config")"
flags="$flags -X $package.analyticsAppVersion=$version"
source_commit="${NEVERMIND_SOURCE_COMMIT:-${GITHUB_SHA:-}}"
if [ -z "$source_commit" ]; then
  source_commit="$(git -C "$(dirname "$config")" rev-parse HEAD 2>/dev/null || true)"
fi
if [ -n "$source_commit" ]; then
  if ! [[ "$source_commit" =~ ^[a-f0-9]{40}$ ]]; then
    echo "invalid source commit SHA: expected 40 lowercase hexadecimal characters" >&2
    exit 1
  fi
  flags="$flags -X $package.analyticsSourceCommit=$source_commit"
fi
if [ "${RELEASE:-false}" = true ]; then
  flags="$flags -X $package.releaseBuild=true"
fi
printf '%s\n' "$flags"
