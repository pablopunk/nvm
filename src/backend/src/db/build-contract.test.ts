import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import test from 'node:test';

test('backend build does not run database migrations', function () {
  const packageJson = JSON.parse(
    fs.readFileSync(path.resolve(import.meta.dirname, '../../package.json'), 'utf8'),
  ) as { scripts?: { build?: string } };
  assert.equal(packageJson.scripts?.build, 'astro build');
});
