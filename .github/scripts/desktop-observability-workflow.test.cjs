const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const root = path.resolve(__dirname, '../..');

test('release packaging uploads the final generated maps before packaging without a rebuild', () => {
  const packageJson = JSON.parse(
    fs.readFileSync(path.join(root, 'package.json'), 'utf8'),
  );
  assert.match(
    packageJson.scripts.build,
    /^electron-vite build && node scripts\/upload-desktop-source-maps.cjs$/,
  );
  for (const name of ['dist:mac:all', 'dist:win:x64'])
    assert.match(packageJson.scripts[name], /^pnpm build &&/);
  const workflow = fs.readFileSync(
    path.join(root, '.github/workflows/ci.yml'),
    'utf8',
  );
  for (const job of ['release-mac', 'release-windows']) {
    const definition = workflow.split(`  ${job}:`)[1].split(/\n  [a-z-]+:/)[0];
    assert.match(definition, /NEVERMIND_RELEASE_BUILD: '1'/);
    assert.match(definition, /SENTRY_PROJECT_DESKTOP:/);
  }
  assert.match(
    fs.readFileSync(path.join(root, 'electron-builder.yml'), 'utf8'),
    /!\*\*\/\*\.map/,
  );
});

test('maps are hidden for all processes and upload credentials are not build definitions', () => {
  const config = fs.readFileSync(
    path.join(root, 'electron.vite.config.ts'),
    'utf8',
  );
  assert.equal((config.match(/sourcemap: 'hidden'/g) || []).length, 3);
  assert.match(config, /__NEVERMIND_BUILD__/);
  assert.ok(!config.includes('SENTRY_AUTH_TOKEN'));
  const upload = fs.readFileSync(
    path.join(root, 'scripts/upload-desktop-source-maps.cjs'),
    'utf8',
  );
  assert.match(upload, /'--validate', '--strict', '--wait'/);
  assert.match(upload, /'app:\/\/\/dist'/);
});
