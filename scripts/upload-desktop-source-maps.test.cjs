const assert = require('node:assert/strict');
const { test } = require('node:test');
const { pnpmInvocation } = require('./upload-desktop-source-maps.cjs');

test('uses Node to execute the pnpm script on Windows, including paths with spaces', () => {
  assert.deepEqual(
    pnpmInvocation(
      'C:\\Program Files\\nodejs\\node.exe',
      'C:\\Tools\\pnpm\\bin\\pnpm.cjs',
    ),
    {
      executable: 'C:\\Program Files\\nodejs\\node.exe',
      prefix: ['C:\\Tools\\pnpm\\bin\\pnpm.cjs'],
    },
  );
  assert.throws(
    () => pnpmInvocation('/usr/bin/node', undefined),
    /through pnpm/,
  );
});
