#!/usr/bin/env bash
# Builds the pi host into resources/<os>-<arch>/pi, where mygo build ships it inside the app: the host script
# (sidecar/pi-host), its production dependencies from the lockfile, and the Node binary it runs on. The Node archive is
# checked against its pinned SHA-256 before it is used. Supported: macOS and Linux, amd64 and arm64.
set -euo pipefail

NODE_VERSION=v22.23.4

root="$(cd "$(dirname "$0")/../.." && pwd)"
goos="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$(uname -m)" in
  x86_64|amd64) goarch=amd64; nodearch=x64 ;;
  arm64|aarch64) goarch=arm64; nodearch=arm64 ;;
  *) echo "unsupported machine: $(uname -m)" >&2; exit 1 ;;
esac
case "$goos-$goarch" in
  darwin-arm64) archive="node-$NODE_VERSION-darwin-arm64.tar.gz"; sha=c9e1dbc748b33ed2d14f42445191ecaf14c56259c7a9a26d0042c49ec2b66428 ;;
  darwin-amd64) archive="node-$NODE_VERSION-darwin-x64.tar.gz"; sha=efcc2b0db6f94bf53eddb038fe2b370277a042bd29e4f19b8f8e23598795b819 ;;
  linux-arm64) archive="node-$NODE_VERSION-linux-arm64.tar.xz"; sha=6287aa46b6c375d816e63ae66d024fbbfd70404825aabdfdf898a970871d2f4b ;;
  linux-amd64) archive="node-$NODE_VERSION-linux-x64.tar.xz"; sha=1805bcd35e6815559bd2eb97db438d3fdb0eb3a5e6a977b7c1badca76337e9ce ;;
  *) echo "unsupported system: $goos-$goarch" >&2; exit 1 ;;
esac

digest() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | cut -d' ' -f1; else shasum -a 256 "$1" | cut -d' ' -f1; fi
}

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
out="$root/resources/$goos-$goarch/pi"
rm -rf "$out"
mkdir -p "$out/bin"

curl -fsSL --retry 3 --retry-all-errors --connect-timeout 15 --max-time 300 "https://nodejs.org/dist/$NODE_VERSION/$archive" -o "$work/$archive"
if [ "$(digest "$work/$archive")" != "$sha" ]; then
  echo "Node $NODE_VERSION: checksum mismatch for $archive" >&2
  exit 1
fi
tar -xf "$work/$archive" -C "$work"
cp "$work/${archive%.tar.*}/bin/node" "$out/bin/node"
chmod 755 "$out/bin/node"

cp "$root/sidecar/pi-host/host.mjs" "$root/sidecar/pi-host/package.json" "$root/sidecar/pi-host/package-lock.json" "$out/"
(cd "$out" && npm ci --omit=dev --ignore-scripts --no-audit --no-fund >/dev/null)
# Source maps and docs are not read at run time.
find "$out/node_modules" \( -name '*.map' -o -name '*.md' -o -name '*.markdown' \) -type f -delete
# mygo omits dot-prefixed names when it ships resources, and pi-ai imports one data file that way: ship it undotted.
models="$out/node_modules/@earendil-works/pi-ai/dist/providers"
mv "$models/data/.manifest.json" "$models/data/manifest.json"
sed -i.bak 's#\./data/\.manifest\.json#./data/manifest.json#' "$models/all.js"
rm -f "$models/all.js.bak"
if grep -q '\.manifest\.json' "$models/all.js"; then
  echo "pi host: pi-ai still imports a dotfile" >&2
  exit 1
fi
rm -f "$out/package.json" "$out/package-lock.json"
echo "pi host: $out ($(du -sh "$out" | cut -f1))"
