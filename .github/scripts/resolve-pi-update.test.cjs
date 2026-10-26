const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const os = require('node:os');
const path = require('node:path');
const test = require('node:test');
const { resolveCandidate } = require('./resolve-pi-update.cjs');

test('registry resolution requests publication times and handles updates and current pins', async function registryContract(t) {
  const root = await fs.mkdtemp(
    path.join(os.tmpdir(), 'pi-registry-contract-'),
  );
  t.after(() => fs.rm(root, { recursive: true, force: true }));
  const packages = ['@earendil-works/pi-ai', '@earendil-works/pi-coding-agent'];
  const now = Date.parse('2026-10-26T12:00:00Z');
  const calls = [];
  t.mock.method(
    globalThis,
    'fetch',
    async function registryResponse(url, options) {
      calls.push({ url, accept: options.headers.accept });
      const name = decodeURIComponent(new URL(url).pathname.slice(1));
      assert.ok(packages.includes(name));
      const metadata = {
        name,
        'dist-tags': { latest: '1.5.0' },
        versions: Object.fromEntries(
          ['1.4.0', '1.5.0'].map((version) => [
            version,
            {
              name,
              version,
              dependencies: {},
              dist: {
                tarball: `https://registry.npmjs.org/${name}/-/fixture-${version}.tgz`,
              },
            },
          ]),
        ),
      };
      // npm's abbreviated install packument omits the time map.
      if (options.headers.accept === 'application/json')
        metadata.time = {
          '1.4.0': '2026-10-01T00:00:00Z',
          '1.5.0': '2026-10-20T00:00:00Z',
        };
      return Response.json(metadata);
    },
  );
  await fs.writeFile(
    path.join(root, 'pnpm-workspace.yaml'),
    'minimumReleaseAge: 1440\n',
  );
  async function setPins(version) {
    await fs.writeFile(
      path.join(root, 'package.json'),
      JSON.stringify({
        dependencies: Object.fromEntries(
          packages.map((name) => [name, version]),
        ),
      }),
    );
  }
  await setPins('1.4.0');
  const candidate = await resolveCandidate(root, now);
  assert.deepEqual(
    candidate.candidateVersions,
    Object.fromEntries(packages.map((name) => [name, '1.5.0'])),
  );
  await setPins('1.5.0');
  assert.equal(await resolveCandidate(root, now), null);
  assert.equal(calls.length, 4);
  assert.ok(calls.every((call) => call.accept === 'application/json'));
});
