#!/usr/bin/env bash
# Writes one packaging run's settings into mygo.json. CI runs it on a fresh checkout, so nothing secret is committed.
# Reads VERSION (blank keeps the committed version), NOTARY_PROFILE and NOTARY_KEYCHAIN (both set to notarize) and
# WINDOWS_CERTIFICATE_FILE (a .pfx path to sign Windows builds).
set -euo pipefail

cd "$(dirname "$0")/../.."
tmp="$(mktemp)"
jq \
  --arg version "${VERSION:-}" \
  --arg profile "${NOTARY_PROFILE:-}" \
  --arg keychain "${NOTARY_KEYCHAIN:-}" \
  --arg certificate "${WINDOWS_CERTIFICATE_FILE:-}" '
  (if $version != "" then .version = $version else . end)
  | (if $profile != "" and $keychain != "" then .macos.notarize = {keychainProfile: $profile, keychain: $keychain} else . end)
  | (if $certificate != "" then .windows.certificate = $certificate else . end)
' mygo.json >"$tmp"
mv "$tmp" mygo.json
jq -c '{version, macos: .macos.notarize, windows: .windows.certificate}' mygo.json
