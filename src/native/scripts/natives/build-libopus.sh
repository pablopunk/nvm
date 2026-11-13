#!/usr/bin/env bash
# Builds the pinned libopus shared library for this machine into resources/<os>-<arch>/lib, where mygo build ships it
# beside the app's other natives and mygo signs it. The source tarball is checked against its SHA-256 before it builds.
set -euo pipefail

OPUS_VERSION=1.5.2
OPUS_SHA256=65c1d2f78b9f2fb20082c38cbe47c951ad5839345876e46941612ee87f9a7ce1
OPUS_URL="https://downloads.xiph.org/releases/opus/opus-${OPUS_VERSION}.tar.gz"
OPUS_RELEASE_URL="https://github.com/xiph/opus/releases/download/v${OPUS_VERSION}/opus-${OPUS_VERSION}.tar.gz"
REQUIRED_SYMBOLS="opus_encoder_create opus_encode opus_encoder_ctl opus_encoder_destroy"

root="$(cd "$(dirname "$0")/../.." && pwd)"
goos="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$(uname -m)" in
  x86_64|amd64) goarch=amd64 ;;
  arm64|aarch64) goarch=arm64 ;;
  *) echo "unsupported machine: $(uname -m)" >&2; exit 1 ;;
esac
case "$goos" in
  darwin) library=libopus.dylib ;;
  linux) library=libopus.so.0 ;;
  *) echo "unsupported system: $goos" >&2; exit 1 ;;
esac

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

digest() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | cut -d' ' -f1
  else
    shasum -a 256 "$1" | cut -d' ' -f1
  fi
}

if ! curl -fsSL --retry 3 --retry-all-errors --connect-timeout 15 --max-time 120 "$OPUS_URL" -o "$work/opus.tar.gz"; then
  echo "libopus $OPUS_VERSION: upstream download failed; trying the upstream GitHub release asset" >&2
  rm -f "$work/opus.tar.gz"
  if ! curl -fsSL --retry 3 --retry-all-errors --connect-timeout 15 --max-time 120 "$OPUS_RELEASE_URL" -o "$work/opus.tar.gz"; then
    echo "libopus $OPUS_VERSION: both upstream downloads failed" >&2
    exit 1
  fi
fi
if [ "$(digest "$work/opus.tar.gz")" != "$OPUS_SHA256" ]; then
  echo "libopus $OPUS_VERSION: SHA-256 does not match the pinned digest" >&2
  exit 1
fi

tar -xzf "$work/opus.tar.gz" -C "$work"
cd "$work/opus-$OPUS_VERSION"
./configure --disable-static --disable-doc --disable-extra-programs >"$work/configure.log" 2>&1 || {
  cat "$work/configure.log" >&2
  exit 1
}
make -j"$(getconf _NPROCESSORS_ONLN 2>/dev/null || sysctl -n hw.ncpu)" >"$work/make.log" 2>&1 || {
  tail -n 80 "$work/make.log" >&2
  exit 1
}

built="$work/opus-$OPUS_VERSION/.libs/$library"
if [ ! -f "$built" ]; then
  echo "libopus build did not produce $library" >&2
  exit 1
fi

symbols() {
  if [ "$goos" = darwin ]; then nm -gU "$1"; else nm -D --defined-only "$1"; fi
}
for symbol in $REQUIRED_SYMBOLS; do
  if ! symbols "$built" | grep -q "$symbol\$"; then
    echo "libopus build lacks $symbol" >&2
    exit 1
  fi
done

out="$root/resources/$goos-$goarch/lib"
mkdir -p "$out"
cp -L "$built" "$out/$library"
echo "libopus $OPUS_VERSION ($goos-$goarch): $out/$library"
