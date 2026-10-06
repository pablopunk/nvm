const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const os = require('node:os');
const path = require('node:path');
const test = require('node:test');
const asar = require('@electron/asar');
const {
  verifyPackagedDependencies,
} = require('../../scripts/verify-packaged-dependencies.cjs');

async function packagedFixture(t, manifests) {
  const temporary = await fs.mkdtemp(
    path.join(os.tmpdir(), 'nevermind-packaged-dependencies-'),
  );
  t.after(function removeFixture() {
    return fs.rm(temporary, { recursive: true, force: true });
  });
  const source = path.join(temporary, 'source');
  for (const [directory, manifest] of Object.entries(manifests)) {
    const target = path.join(source, directory);
    await fs.mkdir(target, { recursive: true });
    await fs.writeFile(
      path.join(target, 'package.json'),
      JSON.stringify(manifest),
    );
  }
  const archive = path.join(temporary, 'app.asar');
  await asar.createPackage(source, archive);
  return archive;
}

test('packaging rejects the missing Sentry transitive dependency from the released app', async function missingDependency(t) {
  const archive = await packagedFixture(t, {
    '': { name: 'nvm', dependencies: { '@sentry/electron': '1' } },
    'node_modules/@sentry/electron': {
      name: '@sentry/electron',
      dependencies: { 'require-in-the-middle': '1' },
    },
    'node_modules/require-in-the-middle': {
      name: 'require-in-the-middle',
      dependencies: { 'module-details-from-path': '1' },
    },
  });
  assert.throws(function verifyMissingPackage() {
    verifyPackagedDependencies(archive);
  }, /require-in-the-middle requires module-details-from-path/);
});

test('packaging verifies the full required graph, nested versions and cycles', async function fullGraph(t) {
  const archive = await packagedFixture(t, {
    '': { name: 'nvm', dependencies: { a: '1', b: '1' } },
    'node_modules/a': { name: 'a', dependencies: { shared: '1', b: '1' } },
    'node_modules/a/node_modules/shared': { name: 'shared', version: '1' },
    'node_modules/b': { name: 'b', dependencies: { shared: '2', a: '1' } },
    'node_modules/shared': { name: 'shared', version: '2' },
  });
  assert.equal(verifyPackagedDependencies(archive).packagesChecked, 4);
});

test('packaging permits absent optional platform dependencies but not required ones', async function optionalGraph(t) {
  const archive = await packagedFixture(t, {
    '': { name: 'nvm', dependencies: { native: '1' } },
    'node_modules/native': {
      name: 'native',
      dependencies: { fsevents: '1' },
      optionalDependencies: { fsevents: '1', windows: '1' },
    },
  });
  assert.equal(verifyPackagedDependencies(archive).packagesChecked, 1);
});

test('present optional packages must still contain their required dependencies', async function presentOptional(t) {
  const archive = await packagedFixture(t, {
    '': { name: 'nvm', optionalDependencies: { native: '1' } },
    'node_modules/native': { name: 'native', dependencies: { missing: '1' } },
  });
  assert.throws(function rejectBrokenOptional() {
    verifyPackagedDependencies(archive);
  }, /native requires missing/);
});
