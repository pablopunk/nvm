// biome-ignore-all lint/style/useNamingConvention: Test data mirrors Launch Services plist keys.
import assert from 'node:assert/strict';
import test from 'node:test';
import { compatibleOpenWithApps } from './open-with-apps';

const apps = [
  { id: 'text', name: 'Text App', path: '/Applications/Text.app' },
  { id: 'image', name: 'Image App', path: '/Applications/Image.APP' },
  { id: 'other', name: 'Other App', path: '/Applications/Other.app' },
  { id: 'linux', name: 'Desktop Entry', path: '/usr/share/app.desktop' },
];

test('compatibleOpenWithApps ranks apps that declare support for the file', async () => {
  const result = await compatibleOpenWithApps(
    apps,
    'png',
    new Set(['public.png', 'public.image']),
    (appPath) => {
      if (appPath.includes('Image')) {
        return Promise.resolve([{ LSItemContentTypes: ['public.image'] }]);
      }
      if (appPath.includes('Text')) {
        return Promise.resolve([{ CFBundleTypeExtensions: ['png'] }]);
      }
      return Promise.resolve([]);
    },
  );

  assert.deepEqual(
    result.map((app) => app.id),
    ['text', 'image'],
  );
});

test('compatibleOpenWithApps falls back when metadata is unavailable', async () => {
  const result = await compatibleOpenWithApps(apps, 'png', new Set(), () =>
    Promise.resolve([]),
  );

  assert.deepEqual(
    result.map((app) => app.id),
    ['image', 'other', 'text'],
  );
});
