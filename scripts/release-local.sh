#!/usr/bin/env bash
set -euo pipefail

VERSION="${1:-}"
if [[ -z "$VERSION" ]]; then
  echo "Usage: $0 x.y.z"
  exit 1
fi

TAG="v$VERSION"

if [[ -n "$(git status --porcelain)" ]]; then
  echo "Error: working tree is dirty. Commit or stash changes first." >&2
  exit 1
fi

if ! command -v gh >/dev/null 2>&1; then
  echo "Error: gh CLI is required. Install with: brew install gh" >&2
  exit 1
fi

if [[ -f .env.release ]]; then
  set -a
  # shellcheck disable=SC1091
  source .env.release
  set +a
fi

: "${APPLE_ID:?Missing APPLE_ID. Put it in .env.release or export it.}"
: "${APPLE_APP_SPECIFIC_PASSWORD:?Missing APPLE_APP_SPECIFIC_PASSWORD. Put it in .env.release or export it.}"
: "${APPLE_TEAM_ID:?Missing APPLE_TEAM_ID. Put it in .env.release or export it.}"

current_version="$(node -p "require('./package.json').version")"
if [[ "$current_version" != "$VERSION" ]]; then
  node -e "const fs=require('fs'); const p=require('./package.json'); p.version='$VERSION'; fs.writeFileSync('package.json', JSON.stringify(p, null, 2)+'\\n');"
  mise exec -- pnpm install --lockfile-only
  git add package.json pnpm-lock.yaml
  git commit -m "release: $TAG"
fi

if git rev-parse "$TAG" >/dev/null 2>&1; then
  echo "Tag $TAG already exists locally; reusing it."
else
  git tag "$TAG"
fi

echo "Pushing branch and tag..."
git push origin "$(git branch --show-current)"
git push origin "$TAG"
echo "Linux release build will run on GitHub Actions for $TAG."

echo "Building signed/notarized macOS arm64 release..."
mise exec -- pnpm run dist:mac:arm64

if ! ls release/Nevermind-"$VERSION"-arm64.zip release/latest-mac.yml >/dev/null 2>&1; then
  echo "Error: expected release artifacts for $VERSION were not produced." >&2
  exit 1
fi

if gh release view "$TAG" >/dev/null 2>&1; then
  echo "GitHub release $TAG already exists; uploading artifacts with --clobber"
else
  echo "Creating GitHub release $TAG"
  gh release create "$TAG" --title "$TAG" --generate-notes
fi

echo "Uploading artifacts..."
gh release upload "$TAG" \
  release/Nevermind-"$VERSION"-arm64.zip \
  release/latest-mac.yml \
  release/Nevermind-"$VERSION"-arm64.zip.blockmap \
  --clobber

echo "Done: https://github.com/pablopunk/nvm/releases/tag/$TAG"
