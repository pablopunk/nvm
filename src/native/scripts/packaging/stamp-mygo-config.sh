#!/usr/bin/env bash
# Writes one packaging run's settings into mygo.json. CI runs it on a fresh checkout, so nothing secret is committed.
# Reads VERSION (blank keeps the committed version), NOTARY_PROFILE and NOTARY_KEYCHAIN (both set to notarize) and
# WINDOWS_CERTIFICATE_FILE (a .pfx path to sign Windows builds). A release run (RELEASE=true) with UPDATE_PUBLIC_KEY
# and UPDATE_REPOSITORY also writes the updates feed: the repository's native-v releases, signed with the private key
# that MYGO_UPDATER_PRIVATE_KEY gives mygo build.
set -euo pipefail

cd "$(dirname "$0")/../.."
update_tag_prefix=native-v
update_key=""
if [ "${RELEASE:-false}" = true ]; then
  update_key="${UPDATE_PUBLIC_KEY:-}"
fi
tmp="$(mktemp)"
jq \
  --arg version "${VERSION:-}" \
  --arg profile "${NOTARY_PROFILE:-}" \
  --arg keychain "${NOTARY_KEYCHAIN:-}" \
  --arg certificate "${WINDOWS_CERTIFICATE_FILE:-}" \
  --arg updateKey "$update_key" \
  --arg updateRepository "${UPDATE_REPOSITORY:-}" \
  --arg tagPrefix "$update_tag_prefix" '
  (if $version != "" then .version = $version else . end)
  | (if $profile != "" and $keychain != "" then .macos.notarize = {keychainProfile: $profile, keychain: $keychain} else . end)
  | (if $certificate != "" then .windows.certificate = $certificate else . end)
  | (if $updateKey != "" and $updateRepository != "" then .updates = {publicKey: $updateKey, github: $updateRepository, tagPrefix: $tagPrefix} else . end)
' mygo.json >"$tmp"
mv "$tmp" mygo.json
jq -c '{version, macos: .macos.notarize, windows: .windows.certificate, updates: .updates.github}' mygo.json
