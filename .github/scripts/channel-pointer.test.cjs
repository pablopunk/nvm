'use strict';
const assert = require('node:assert/strict');
const { test } = require('node:test');
const { choosePointer, publishPointer } = require('./channel-pointer.cjs');

const old = {
  kind: 'nevermind-channel-pointer',
  channel: 'dev',
  tags: ['v2026.1029.9-dev'],
};
test('pointers advance numerically and reject malformed or cross-channel values', () => {
  assert.equal(
    choosePointer('dev', 'v2026.1029.10-dev', old).tags[0],
    'v2026.1029.10-dev',
  );
  assert.deepEqual(choosePointer('dev', 'v2026.1029.8-dev', old), old);
  assert.throws(() => choosePointer('dev', 'v2026.1029.10-nightly', old));
  assert.throws(() =>
    choosePointer('dev', 'v2026.1029.10-dev', { ...old, channel: 'nightly' }),
  );
});

test('published release pointer can be retried after a failed write without regressing', () => {
  const sha = 'a'.repeat(40);
  let pointer = old;
  let failWrite = true;
  let writes = 0;
  function run(args) {
    const route = args[1] ?? '';
    if (route.includes('/releases/tags/'))
      return JSON.stringify({
        tag_name: 'v2026.1029.10-dev',
        draft: false,
        prerelease: true,
      });
    if (route.includes('/commits/')) return JSON.stringify({ sha });
    if (route.includes('/git/ref/heads/')) return '{}';
    if (route.includes('/contents/') && route.includes('?ref='))
      return JSON.stringify({
        sha: 'previous',
        content: Buffer.from(JSON.stringify(pointer)).toString('base64'),
      });
    if (args[1] === '-X' && args[2] === 'PUT') {
      writes += 1;
      if (failWrite) {
        failWrite = false;
        throw new Error('temporary failure');
      }
      const field = args.find((value) => value.startsWith('content='));
      pointer = JSON.parse(
        Buffer.from(field.slice('content='.length), 'base64').toString(),
      );
      return '{}';
    }
    throw new Error(`Unexpected call ${args.join(' ')}`);
  }
  const input = {
    repository: 'pablopunk/nvm',
    channel: 'dev',
    tag: 'v2026.1029.10-dev',
    sourceSha: sha,
    run,
  };
  assert.throws(() => publishPointer(input), /temporary failure/);
  assert.equal(pointer.tags[0], old.tags[0]);
  assert.equal(publishPointer(input).tags[0], 'v2026.1029.10-dev');
  assert.equal(publishPointer(input).tags[0], 'v2026.1029.10-dev');
  assert.equal(writes, 2);
});
