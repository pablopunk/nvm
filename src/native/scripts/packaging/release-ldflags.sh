#!/usr/bin/env bash
# Prints the -ldflags that mygo build reads from MYGO_LDFLAGS. Every packaged build links isPackagedBuild; only
# RELEASE=true also links the legacy release marker. Analytics consent is enforced by the app.
set -euo pipefail

package=github.com/pablopunk/nvm/native/internal/app
flags="-X $package.isPackagedBuild=true"
config="$(cd "$(dirname "$0")/../.." && pwd)/mygo.json"
version="$(jq -er '.version | strings | select(test("^[A-Za-z0-9][A-Za-z0-9.+_-]*$"))' "$config")"
flags="$flags -X $package.analyticsAppVersion=$version"
if [ "${RELEASE:-false}" = true ]; then
  flags="$flags -X $package.releaseBuild=true"
fi
printf '%s\n' "$flags"
