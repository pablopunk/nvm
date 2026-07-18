import assert from 'node:assert/strict';
import test from 'node:test';
import { windowsShortcutIconSources } from './windows-app-icons';

test('prefers a shortcut icon resource and target before the generic link icon', async () => {
  const shortcutPath = String.raw`C:\Start Menu\Helium.lnk`;
  assert.deepEqual(
    await windowsShortcutIconSources(
      shortcutPath,
      () => ({
        icon: String.raw`C:\Apps\Helium.ico`,
        target: String.raw`C:\Apps\Helium.exe`,
      }),
      async () => '',
    ),
    [
      { path: String.raw`C:\Apps\Helium.ico`, resourceIndex: 0 },
      String.raw`C:\Apps\Helium.exe`,
      shortcutPath,
    ],
  );
});

test('prefers a confined Visual Elements logo over executable fallbacks', async () => {
  const shortcutPath = String.raw`C:\Apps\Helium.lnk`;
  const target = String.raw`C:\Apps\Helium\Application\chrome.exe`;
  assert.deepEqual(
    await windowsShortcutIconSources(
      shortcutPath,
      () => ({ target }),
      async () =>
        `<VisualElements Square44x44Logo="0.14.3.1\\VisualElements\\SmallLogo.png" />`,
    ),
    [
      String.raw`C:\Apps\Helium\Application\0.14.3.1\VisualElements\SmallLogo.png`,
      target,
      shortcutPath,
    ],
  );
});

test('ignores Visual Elements logos outside the application directory', async () => {
  const shortcutPath = String.raw`C:\Apps\Example.lnk`;
  const target = String.raw`C:\Apps\Example\app.exe`;
  assert.deepEqual(
    await windowsShortcutIconSources(
      shortcutPath,
      () => ({ target }),
      async () => `<VisualElements Square44x44Logo="..\\secret.png" />`,
    ),
    [target, shortcutPath],
  );
});

test('falls back to the shortcut when Windows cannot resolve it', async () => {
  const shortcutPath = String.raw`C:\Start Menu\Broken.lnk`;
  assert.deepEqual(
    await windowsShortcutIconSources(shortcutPath, () => {
      throw new Error('unreadable shortcut');
    }),
    [shortcutPath],
  );
});
