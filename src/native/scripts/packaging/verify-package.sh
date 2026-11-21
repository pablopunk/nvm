#!/usr/bin/env bash
# Checks a built executable's release flags by asking it (--build-flags prints them without starting the app): packaged
# is always true for a packaged build, release is true only for a release build.
# usage: verify-package.sh <executable> <true|false: release build>
set -euo pipefail

executable="$1"
release="$2"
want="packaged=true release=$([ "$release" = true ] && echo true || echo false)"
got="$("$executable" --build-flags)"

if [ "$got" != "$want" ]; then
  echo "$executable: build flags are '$got', want '$want'" >&2
  exit 1
fi
echo "$executable: $got"
