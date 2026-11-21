#!/usr/bin/env bash
# Compiles the window and selected-text helper (sidecar/macos-helper) into resources/darwin-<arch>/, where the app
# finds it beside its other resources. Builds for this Mac's architecture; pass arm64 or amd64 to cross-compile.
set -euo pipefail

root="$(cd "$(dirname "$0")/../.." && pwd)"
helper_dir="$root/sidecar/macos-helper"
sources=(
  main.swift menus.swift microphone.swift ocr.swift selected-text.swift thumbnail.swift window-manager.swift window-subcommands.swift window-accessibility.swift
  window-all.swift window-display.swift window-frame-writer.swift window-fullscreen.swift window-listing.swift
  window-raise.swift
)

case "${1:-$(uname -m)}" in
  arm64) swift_arch=arm64; goarch=arm64 ;;
  x86_64|amd64) swift_arch=x86_64; goarch=amd64 ;;
  *) echo "unsupported architecture: ${1:-$(uname -m)}" >&2; exit 1 ;;
esac

out="$root/resources/darwin-$goarch"
mkdir -p "$out"
paths=()
for source in "${sources[@]}"; do
  paths+=("$helper_dir/$source")
done
xcrun swiftc "${paths[@]}" -O -target "$swift_arch-apple-macos12.0" -o "$out/macos-selected-text"
echo "macos-selected-text ($goarch): $out/macos-selected-text"
