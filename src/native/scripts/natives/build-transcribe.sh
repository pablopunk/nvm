#!/usr/bin/env bash
# Builds transcribe.cpp, the engine Hex runs Parakeet on, into one shared library in resources/<os>-<arch>/lib, where
# mygo build ships it beside the app's other natives and mygo signs it. It is the commit Hex pins (its Cargo.toml),
# checked out by hash. The static libraries (transcribe and ggml, with Metal on Apple Silicon) are linked into one
# library so the app loads a single file. Needs git, cmake and a C++ compiler.
set -euo pipefail

TRANSCRIBE_REPO=https://github.com/anomalyco/transcribe.cpp
TRANSCRIBE_COMMIT=60bed89dab84f7c1fddc3a17ad7fa8a27ddfba19
REQUIRED_SYMBOLS="transcribe_open transcribe_run transcribe_full_text transcribe_close transcribe_status_string"

root="$(cd "$(dirname "$0")/../.." && pwd)"
goos="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$(uname -m)" in
  x86_64|amd64) goarch=amd64 ;;
  arm64|aarch64) goarch=arm64 ;;
  *) echo "unsupported machine: $(uname -m)" >&2; exit 1 ;;
esac
case "$goos" in
  darwin) library=libtranscribe.dylib ;;
  linux) library=libtranscribe.so ;;
  *) echo "unsupported system: $goos" >&2; exit 1 ;;
esac
jobs="$(getconf _NPROCESSORS_ONLN 2>/dev/null || sysctl -n hw.ncpu)"

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

git init -q "$work/src"
git -C "$work/src" fetch -q --depth 1 "$TRANSCRIBE_REPO" "$TRANSCRIBE_COMMIT"
git -C "$work/src" checkout -q FETCH_HEAD
if [ "$(git -C "$work/src" rev-parse HEAD)" != "$TRANSCRIBE_COMMIT" ]; then
  echo "transcribe.cpp: the checkout is not the pinned commit $TRANSCRIBE_COMMIT" >&2
  exit 1
fi

cmake -S "$work/src" -B "$work/build" -DCMAKE_BUILD_TYPE=Release -DCMAKE_POSITION_INDEPENDENT_CODE=ON \
  -DTRANSCRIBE_BUILD_SHARED=OFF -DTRANSCRIBE_BUILD_TESTS=OFF -DTRANSCRIBE_BUILD_EXAMPLES=OFF >"$work/configure.log" 2>&1 || {
  tail -n 80 "$work/configure.log" >&2
  exit 1
}
cmake --build "$work/build" -j"$jobs" >"$work/build.log" 2>&1 || {
  tail -n 80 "$work/build.log" >&2
  exit 1
}

archives=()
while IFS= read -r archive; do archives+=("$archive"); done < <(find "$work/build/src" "$work/build/ggml" -name '*.a' -not -path '*/CMakeFiles/*' | sort)
if [ "${#archives[@]}" -eq 0 ]; then
  echo "transcribe.cpp: the build produced no static libraries" >&2
  exit 1
fi

built="$work/$library"
if [ "$goos" = darwin ]; then
  c++ -dynamiclib -o "$built" -install_name "@rpath/$library" -Wl,-all_load "${archives[@]}" \
    -framework Foundation -framework Metal -framework MetalKit -framework Accelerate -lc++
else
  c++ -shared -o "$built" -Wl,--whole-archive "${archives[@]}" -Wl,--no-whole-archive -lpthread -lm -ldl
fi

symbols() {
  if [ "$goos" = darwin ]; then nm -gU "$1"; else nm -D --defined-only "$1"; fi
}
for symbol in $REQUIRED_SYMBOLS; do
  if ! symbols "$built" | grep -q "$symbol\$"; then
    echo "transcribe.cpp library lacks $symbol" >&2
    exit 1
  fi
done

out="$root/resources/$goos-$goarch/lib"
mkdir -p "$out" "$root/resources/$goos-$goarch/transcribe"
cp "$built" "$out/$library"
cp "$work/src/LICENSE" "$root/resources/$goos-$goarch/transcribe/TRANSCRIBE-CPP-LICENSE"
echo "transcribe.cpp $TRANSCRIBE_COMMIT ($goos-$goarch): $out/$library"
