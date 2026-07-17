import assert from 'node:assert/strict';
import nodeOs from 'node:os';
import test, { mock } from 'node:test';

mock.module('electron', {
  namedExports: {
    app: { isPackaged: false },
    shell: {},
  },
});

const os = await import('./os');
const linuxOnly = { skip: process.platform !== 'linux' };

test('keeps the Linux platform contract non-destructive', linuxOnly, () => {
  assert.equal(os.osLabel(), 'Linux');
  assert.equal(os.settingsTitle(), 'Open Settings');
  assert.equal(os.revealPathTitle(), 'Show in File Manager');
  assert.equal(os.quickLookTitle(), 'Preview File');
  assert.equal(os.hasCapability('quick-look'), false);
  assert.equal(os.hasCapability('launch-at-login'), false);
  assert.equal(os.getLaunchAtLoginEnabled(), false);
  assert.deepEqual(os.setLaunchAtLoginEnabled(true), {
    ok: false,
    message: 'Start at login is not available on Linux',
  });
  assert.deepEqual(os.paletteBrowserWindowOptions(), {});
  assert.equal(os.isReservedPaletteAccelerator('Command+Space'), false);
  assert.equal(os.reservedPaletteShortcutName(), 'the system');
  assert.deepEqual(os.appScanRoots(), [
    '/usr/share/applications',
    '/usr/local/share/applications',
    `${nodeOs.homedir()}/.local/share/applications`,
  ]);
});

test(
  'only enables Linux auto-updates for AppImages and restores APPIMAGE',
  linuxOnly,
  () => {
    const originalAppImage = process.env.APPIMAGE;
    try {
      delete process.env.APPIMAGE;
      assert.equal(os.supportsAutoUpdates(), false);
      process.env.APPIMAGE = '/tmp/Nevermind.AppImage';
      assert.equal(os.supportsAutoUpdates(), true);
    } finally {
      if (originalAppImage === undefined) delete process.env.APPIMAGE;
      else process.env.APPIMAGE = originalAppImage;
    }
  },
);

test(
  'parses visible Linux desktop entries without filesystem access',
  linuxOnly,
  () => {
    assert.deepEqual(
      os.parseLinuxDesktopEntry(`
[Desktop Entry]
Name=Example App
Exec=env FOO=bar /opt/example/bin/example --open %U %c
StartupWMClass=ExampleApp
`),
      {
        name: 'Example App',
        command: 'env FOO=bar /opt/example/bin/example --open',
        wmClass: 'ExampleApp',
      },
    );
    for (const body of [
      '[Desktop Entry]\nNoDisplay=true\nName=Hidden\nExec=hidden',
      '[Desktop Entry]\nHidden=true\nName=Hidden\nExec=hidden',
      '[Desktop Entry]\nExec=missing-name',
      '[Desktop Entry]\nName=Missing Exec',
    ])
      assert.equal(os.parseLinuxDesktopEntry(body), null);
  },
);

test(
  'matches Linux app commands and window classes against injected process names',
  linuxOnly,
  () => {
    assert.equal(
      os.linuxAppMatchesProcessNames(
        {
          command: 'env DEBUG=1 /opt/Example/bin/example',
          wmClass: 'ExampleApp',
        },
        ['other-process', 'EXAMPLE'],
      ),
      true,
    );
    assert.equal(
      os.linuxAppMatchesProcessNames(
        { command: 'example', wmClass: 'ExampleApp' },
        ['exampleapp'],
      ),
      true,
    );
    assert.equal(
      os.linuxAppMatchesProcessNames(
        { command: 'example', wmClass: 'ExampleApp' },
        ['unrelated'],
      ),
      false,
    );
  },
);
