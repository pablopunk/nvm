import assert from 'node:assert/strict';
import path from 'node:path';
import test from 'node:test';
import { resolveExtensionPaths } from './extension-paths';

test('uses the dot-config extension path on every platform', () => {
  assert.deepEqual(
    resolveExtensionPaths({
      homeDirectory: '/home/example',
      userDataDirectory: '/data/nvm',
    }),
    {
      extensionsDir: '/home/example/.config/nevermind/extensions',
      extensionDraftsDir: '/data/nvm/extension-drafts',
    },
  );

  assert.deepEqual(
    resolveExtensionPaths({
      homeDirectory: String.raw`C:\Users\example`,
      userDataDirectory: String.raw`C:\Users\example\AppData\Roaming\nvm`,
      joinPath: path.win32.join,
    }),
    {
      extensionsDir: path.win32.join(
        String.raw`C:\Users\example`,
        '.config',
        'nevermind',
        'extensions',
      ),
      extensionDraftsDir: path.win32.join(
        String.raw`C:\Users\example\AppData\Roaming\nvm`,
        'extension-drafts',
      ),
    },
  );
});

test('keeps extension paths inside test user data', () => {
  assert.deepEqual(
    resolveExtensionPaths({
      homeDirectory: '/home/example',
      userDataDirectory: '/tmp/nvm-test',
      useUserDataForExtensions: true,
    }),
    {
      extensionsDir: '/tmp/nvm-test/extensions',
      extensionDraftsDir: '/tmp/nvm-test/extension-drafts',
    },
  );
});
