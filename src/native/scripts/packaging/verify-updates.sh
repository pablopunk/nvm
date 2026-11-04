#!/usr/bin/env bash
# Checks that a release build links the update public key that mygo.json's updates carries. MyGo stamps it with
# -ldflags, so it is a string in the executable; a build without it would never find or verify an update.
# usage: verify-updates.sh <executable> <public key>
set -euo pipefail

executable="$1"
key="$2"
if ! grep -aqF -- "$key" "$executable"; then
  echo "$executable: the update public key is not linked" >&2
  exit 1
fi
echo "$executable: links the update public key"
