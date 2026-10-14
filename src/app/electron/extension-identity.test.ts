import assert from 'node:assert/strict';
import test from 'node:test';
import { sameExtensionSourceFile } from './extension-identity';

test('matches the same extension source path', () => {
  assert.equal(
    sameExtensionSourceFile(
      '/home/user/extensions/paperless.ts',
      '/home/user/extensions/paperless.ts',
    ),
    true,
  );
});

test('does not treat a same-basename fixture as the same extension owner', () => {
  assert.equal(
    sameExtensionSourceFile(
      '/app/src/app/fixtures/paperless.ts',
      '/home/user/extensions/paperless.ts',
    ),
    false,
  );
});

test('does not match an extension with no source file', () => {
  assert.equal(
    sameExtensionSourceFile(undefined, '/home/user/extensions/x.ts'),
    false,
  );
});
