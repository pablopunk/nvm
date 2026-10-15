import assert from 'node:assert/strict';
import test from 'node:test';
import { createLinuxDesktopText } from './linux-desktop-text';

test('uses one X11 window identity for capture, focus, Copy, and paste', async () => {
  const calls: string[] = [];
  let activeWindow = '42';
  const desktop = createLinuxDesktopText({
    environment: { DISPLAY: ':99', XDG_SESSION_TYPE: 'x11' },
    findCommand: async (name) => `/usr/bin/${name}`,
    run: async (_command, args) => {
      calls.push(args.join(' '));
      if (args[0] === 'getactivewindow') return activeWindow;
      if (args[0] === 'getwindowpid') return '321\n';
      if (args[0] === 'windowactivate') activeWindow = '42';
      return '';
    },
  });
  const target = await desktop.target();
  assert.deepEqual(target, {
    bundleId: 'linux:x11:42:321',
    windowId: '42',
    pid: 321,
  });
  assert.equal(await desktop.read(target!), null);
  assert.equal(await desktop.copy(target!), true);
  await desktop.paste(target!.bundleId);
  activeWindow = '77';
  await assert.rejects(desktop.copy(target!), /lost focus/);
  assert.equal(await desktop.restore(target!), true);
  assert.deepEqual(
    calls.filter((call) => call.startsWith('key ')),
    ['key --clearmodifiers ctrl+c', 'key --clearmodifiers ctrl+v'],
  );
});

test('does not restore a recycled window owned by another process', async () => {
  const desktop = createLinuxDesktopText({
    environment: { DISPLAY: ':99', XDG_SESSION_TYPE: 'x11' },
    findCommand: () => '/usr/bin/xdotool',
    run: async (_command, args) => (args[0] === 'getwindowpid' ? '999' : '42'),
  });
  assert.equal(
    await desktop.restore({
      bundleId: 'linux:x11:42:321',
      windowId: '42',
      pid: 321,
    }),
    false,
  );
});

test('reports missing tools and unsupported Wayland explicitly', async () => {
  const missing = createLinuxDesktopText({
    environment: { DISPLAY: ':99' },
    findCommand: () => null,
  });
  assert.deepEqual(await missing.access(), {
    state: 'unavailable',
    message:
      'Install xdotool and xclip to enable selected-text control, then restart Nevermind.',
  });
  assert.equal(missing.available(), true);
  const unsupported = createLinuxDesktopText({
    environment: { XDG_SESSION_TYPE: 'wayland' },
  });
  assert.equal((await unsupported.access()).state, 'unsupported');
  await assert.rejects(unsupported.target(), /Wayland desktop/);
});

test('checks Hyprland and Sway window identities without depending on XWayland', async () => {
  const hyprland = createLinuxDesktopText({
    environment: {
      XDG_SESSION_TYPE: 'wayland',
      HYPRLAND_INSTANCE_SIGNATURE: 'test',
    },
    findCommand: () => '/usr/bin/hyprctl',
    run: async (_command, args) =>
      args[1] === 'activewindow'
        ? JSON.stringify({ address: '0xabc', pid: 123 })
        : '[]',
  });
  assert.equal((await hyprland.target())?.bundleId, 'linux:hyprland:0xabc:123');
  const sway = createLinuxDesktopText({
    environment: { XDG_SESSION_TYPE: 'wayland', SWAYSOCK: '/run/test' },
    findCommand: (name) => `/usr/bin/${name}`,
    run: async (_command, args) =>
      args.includes('get_tree')
        ? JSON.stringify({
            nodes: [{ id: 3, nodes: [{ id: 15, focused: true, pid: 345 }] }],
          })
        : '',
  });
  assert.equal((await sway.target())?.bundleId, 'linux:sway:15:345');
});
