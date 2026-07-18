import assert from 'node:assert/strict';
import test, { mock } from 'node:test';

mock.module('electron', {
  namedExports: {
    app: { isPackaged: false },
    shell: {},
  },
});

const { createOsAdapter } = await import('./os');

const capabilities = [
  'windows.always-on-top',
  'windows.all-spaces',
  'windows.frame-restore',
  'windows.display-recovery',
] as const;

test('drives every window capability from the injected OS and Linux session', () => {
  const cases = [
    {
      platform: 'darwin',
      sessionType: 'x11',
      expected: [true, true, true, true],
    },
    {
      platform: 'win32',
      sessionType: 'x11',
      expected: [true, false, true, true],
    },
    {
      platform: 'linux',
      sessionType: 'x11',
      expected: [true, true, true, true],
    },
    {
      platform: 'linux',
      sessionType: 'wayland',
      expected: [true, true, false, false],
    },
  ] as const;

  for (const { platform, sessionType, expected } of cases) {
    const adapter = createOsAdapter({
      processPlatform: platform,
      sessionType,
    });
    assert.deepEqual(
      capabilities.map((capability) => adapter.hasCapability(capability)),
      expected,
      `${platform}/${sessionType}`,
    );
  }
});

test('detects Wayland from the injectable session environment', () => {
  const adapter = createOsAdapter({
    environment: { XDG_SESSION_TYPE: 'wayland' },
    processPlatform: 'linux',
  });

  assert.equal(adapter.hasCapability('windows.frame-restore'), false);
  assert.equal(adapter.hasCapability('windows.display-recovery'), false);
  assert.equal(adapter.hasCapability('windows.all-spaces'), true);
});
