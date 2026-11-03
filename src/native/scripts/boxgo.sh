#!/usr/bin/env bash
# Run Go commands for src/native inside the shared test box (Go is not installed locally).
# usage: scripts/boxgo.sh <slot> '<shell command>'
#   scripts/boxgo.sh wp01 'go build ./... && go vet ./... && go test -race ./...'
# <slot> isolates concurrent agents: use your WP id (wp01, wp02, ...).
# The current src/native tree is synced to the box, the command runs there with Go on PATH.
# To copy a file produced in the box back (e.g. after `gofmt -w` or `-update` goldens):
#   scripts/boxgo.sh --get <slot> <relative/path> [local/path]
set -euo pipefail
BOX=9101
root=$(git rev-parse --show-toplevel)
if [ "${1:-}" = "--get" ]; then
  slot=$2; rel=$3; dest=${4:-$root/src/native/$rel}
  mkdir -p "$(dirname "$dest")"
  box cp "$BOX:/root/wp/$slot/$rel" "$dest"
  exit 0
fi
slot=$1; shift
tmp=$(mktemp -d)
COPYFILE_DISABLE=1 tar --no-xattrs -czf "$tmp/src.tgz" -C "$root/src/native" --exclude=.git --exclude="._*" .
box exec "$BOX" -- "mkdir -p /root/wp" >/dev/null
box cp "$tmp/src.tgz" "$BOX:/root/wp/$slot.tgz" >/dev/null
box exec "$BOX" -- "rm -rf /root/wp/$slot && mkdir -p /root/wp/$slot && cd /root/wp/$slot && tar xzf /root/wp/$slot.tgz && export PATH=\$PATH:/usr/local/go/bin && $*"
rm -rf "$tmp"
