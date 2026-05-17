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

echo "CI will build, sign, notarize, and publish macOS + Linux artifacts for $TAG."
echo "Watch: https://github.com/pablopunk/nvm/actions"
echo "Release: https://github.com/pablopunk/nvm/releases/tag/$TAG"
