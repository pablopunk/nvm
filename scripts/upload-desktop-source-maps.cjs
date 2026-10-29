const { execFileSync } = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');
const { desktopBuildIdentity } = require('./desktop-build-identity.cjs');

const POSTHOG_CLI_VERSION = '0.19.0';

function uploadDesktopSourceMaps() {
  const required = process.env.NEVERMIND_RELEASE_BUILD === '1';
  if (!required) return;
  for (const key of [
    'POSTHOG_CLI_API_KEY',
    'POSTHOG_CLI_PROJECT_ID',
    'VITE_POSTHOG_HOST',
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
  execFileSync(
    'pnpm',
    [
      'dlx',
      `@posthog/cli@${POSTHOG_CLI_VERSION}`,
      '--host',
      process.env.VITE_POSTHOG_HOST,
      'sourcemap',
      'process',
      '--directory',
      path.join(root, 'dist'),
      '--release-name',
      'nevermind',
      '--release-version',
      desktopBuildIdentity(root),
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
