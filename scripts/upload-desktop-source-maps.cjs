const { execFileSync } = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');
const { desktopBuildIdentity } = require('./desktop-build-identity.cjs');

function uploadDesktopSourceMaps() {
  const required = process.env.NEVERMIND_RELEASE_BUILD === '1';
  if (!required) return;
  for (const key of [
    'SENTRY_AUTH_TOKEN',
    'SENTRY_ORG',
    'SENTRY_PROJECT_DESKTOP',
  ]) {
    if (!process.env[key])
      throw new Error(`${key} is required for desktop release source maps`);
  }
  const root = path.resolve(__dirname, '..');
  for (const file of [
    'dist/main/main.js.map',
    'dist/preload/preload.cjs.map',
  ]) {
    if (!fs.existsSync(path.join(root, file)))
      throw new Error(`Missing source map: ${file}`);
  }
  const cli = require('@sentry/cli');
  execFileSync(
    cli.getPath(),
    [
      'sourcemaps',
      'upload',
      '--org',
      process.env.SENTRY_ORG,
      '--project',
      process.env.SENTRY_PROJECT_DESKTOP,
      '--release',
      desktopBuildIdentity(root),
      '--url-prefix',
      'app:///dist',
      '--validate',
      '--strict',
      '--wait',
      path.join(root, 'dist'),
    ],
    { cwd: root, stdio: 'inherit' },
  );
}

if (require.main === module) uploadDesktopSourceMaps();
module.exports = { uploadDesktopSourceMaps };
