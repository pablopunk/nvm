#!/usr/bin/env bash
set -euo pipefail

# Uploads the secrets needed by .github/workflows/ci.yml to sign and notarize
# the macOS release on GitHub Actions.
#
# Required env (export, or put in .env.release):
#   APPLE_ID, APPLE_APP_SPECIFIC_PASSWORD, APPLE_TEAM_ID
#   MACOS_CERT_P12_PATH   path to exported Developer ID Application .p12
#   MACOS_CERT_PASSWORD   password used when exporting the .p12

if [[ -f .env.release ]]; then
  set -a
  # shellcheck disable=SC1091
  source .env.release
  set +a
fi

: "${APPLE_ID:?Missing APPLE_ID}"
: "${APPLE_APP_SPECIFIC_PASSWORD:?Missing APPLE_APP_SPECIFIC_PASSWORD}"
: "${APPLE_TEAM_ID:?Missing APPLE_TEAM_ID}"
: "${MACOS_CERT_P12_PATH:?Missing MACOS_CERT_P12_PATH (path to .p12 exported from Keychain Access)}"
: "${MACOS_CERT_PASSWORD:?Missing MACOS_CERT_PASSWORD (.p12 export password)}"

if ! command -v gh >/dev/null 2>&1; then
  echo "gh CLI required. brew install gh" >&2
  exit 1
fi

if [[ ! -f "$MACOS_CERT_P12_PATH" ]]; then
  echo "Cert not found at $MACOS_CERT_P12_PATH" >&2
  exit 1
fi

echo "Setting GitHub Actions secrets..."
gh secret set APPLE_ID --body "$APPLE_ID"
gh secret set APPLE_APP_SPECIFIC_PASSWORD --body "$APPLE_APP_SPECIFIC_PASSWORD"
gh secret set APPLE_TEAM_ID --body "$APPLE_TEAM_ID"
gh secret set MACOS_CERT_PASSWORD --body "$MACOS_CERT_PASSWORD"
base64 < "$MACOS_CERT_P12_PATH" | gh secret set MACOS_CERT_P12_BASE64

echo "Done. Verify with: gh secret list"
