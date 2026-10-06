const assert = require('node:assert/strict');
const { execFile } = require('node:child_process');
const fs = require('node:fs/promises');
const os = require('node:os');
const path = require('node:path');
const { promisify } = require('node:util');
const zlib = require('node:zlib');
const test = require('node:test');

test('Linux blockmap generation uses the installed packager and preserves the artifact', async function externalBlockmap(t) {
  const temporary = await fs.mkdtemp(
    path.join(os.tmpdir(), 'nevermind-blockmap-'),
  );
  t.after(function removeFixture() {
    return fs.rm(temporary, { recursive: true, force: true });
  });
  const input = path.join(temporary, 'fixture.AppImage');
  const content = Buffer.alloc(100_000, 42);
  await fs.writeFile(input, content);
  await promisify(execFile)(process.execPath, [
    path.resolve(
      __dirname,
      '../../scripts/generate-linux-appimage-blockmap.cjs',
    ),
    input,
  ]);
  assert.deepEqual(await fs.readFile(input), content);
  const map = JSON.parse(
    zlib.gunzipSync(await fs.readFile(`${input}.blockmap`)),
  );
  assert.equal(map.version, '2');
  assert.equal(map.files.length, 1);
  assert.equal(
    map.files[0].sizes.reduce(function total(sum, size) {
      return sum + size;
    }, 0),
    content.length,
  );
  assert.equal(map.files[0].checksums.length, map.files[0].sizes.length);
});
