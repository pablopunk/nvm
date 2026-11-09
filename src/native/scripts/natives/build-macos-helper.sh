#!/usr/bin/env bash
# Compiles the window and selected-text helper of the Electron app for this Mac's architecture into
# resources/darwin-<arch>/, where the app finds it beside its other resources. The sources are shared, not copied.
set -euo pipefail

root="$(cd "$(dirname "$0")/../.." && pwd)"
helper_dir="$root/../app/resources/macos-helper"
sources=(
  main.swift menus.swift microphone.swift ocr.swift selected-text.swift thumbnail.swift window-manager.swift window-subcommands.swift window-accessibility.swift
  window-all.swift window-display.swift window-frame-writer.swift window-fullscreen.swift window-listing.swift
  window-raise.swift
)

case "$(uname -m)" in
  arm64) swift_arch=arm64; goarch=arm64 ;;
  x86_64) swift_arch=x86_64; goarch=amd64 ;;
  *) echo "unsupported machine: $(uname -m)" >&2; exit 1 ;;
esac

out="$root/resources/darwin-$goarch"
mkdir -p "$out"
paths=()
for source in "${sources[@]}"; do
  paths+=("$helper_dir/$source")
done
xcrun swiftc "${paths[@]}" -O -target "$swift_arch-apple-macos12.0" -o "$out/macos-selected-text"
echo "macos-selected-text ($goarch): $out/macos-selected-text"
