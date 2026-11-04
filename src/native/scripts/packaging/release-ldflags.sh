#!/usr/bin/env bash
# Prints the -ldflags that mygo build reads from MYGO_LDFLAGS. Every packaged build links isPackagedBuild; only
# RELEASE=true also links releaseBuild, the one flag that gives the build an analytics endpoint.
set -euo pipefail

package=github.com/pablopunk/nvm/native/internal/app
flags="-X $package.isPackagedBuild=true"
if [ "${RELEASE:-false}" = true ]; then
  flags="$flags -X $package.releaseBuild=true"
fi
printf '%s\n' "$flags"
