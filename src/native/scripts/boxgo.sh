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
# box_retry reruns a box call that lost the race on the box CLI's own jump-host file, which concurrent agents rewrite
# (`mv: rename .../jump.sh.new`); any other failure is the command's own and is returned as is.
box_retry() {
  local out status attempt
  for attempt in 1 2 3 4 5; do
    set +e
    out=$(box "$@" 2>&1)
    status=$?
    set -e
    if [ $status -eq 0 ] || ! grep -q 'jump.sh.new' <<<"$out"; then
      printf '%s\n' "$out"
      return $status
    fi
    sleep $((attempt * 2))
  done
  printf '%s\n' "$out"
  return $status
}
if [ "${1:-}" = "--get" ]; then
  slot=$2; rel=$3; dest=${4:-$root/src/native/$rel}
  mkdir -p "$(dirname "$dest")"
  box_retry cp "$BOX:/root/wp/$slot/$rel" "$dest"
  exit 0
fi
slot=$1; shift
tmp=$(mktemp -d)
COPYFILE_DISABLE=1 tar --no-xattrs -czf "$tmp/src.tgz" -C "$root/src/native" --exclude=.git --exclude="._*" .
# Many agents share the box's Go build cache, which grows without bound: trim old entries when space runs low.
box_retry exec "$BOX" -- 'mkdir -p /root/wp; [ "$(df --output=avail -k / | tail -1)" -gt 3145728 ] || find /root/.cache/go-build -type f -mmin +30 -delete' >/dev/null
box_retry cp "$tmp/src.tgz" "$BOX:/root/wp/$slot.tgz" >/dev/null
box_retry exec "$BOX" -- "rm -rf /root/wp/$slot && mkdir -p /root/wp/$slot && cd /root/wp/$slot && tar xzf /root/wp/$slot.tgz && export PATH=\$PATH:/usr/local/go/bin GOCACHE=/root/.cache/go-build GOMODCACHE=/root/go/pkg/mod && $*"
rm -rf "$tmp"
