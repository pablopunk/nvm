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
  validateSourceMapPairs(path.join(root, 'dist'));
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

function validateSourceMapPairs(directory) {
  for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
    const file = path.join(directory, entry.name);
    if (
      path.basename(directory) === 'assets' &&
      /^rolldown-runtime-[a-zA-Z0-9_-]+\.js$/.test(entry.name)
    )
      continue;
    if (entry.isDirectory()) validateSourceMapPairs(file);
    else if (
      /\.(?:js|cjs|mjs)$/.test(entry.name) &&
      !fs.existsSync(`${file}.map`)
    )
      throw new Error(`Missing source map for generated JavaScript: ${file}`);
  }
}

if (require.main === module) uploadDesktopSourceMaps();
module.exports = { uploadDesktopSourceMaps };
