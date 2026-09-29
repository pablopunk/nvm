import assert from 'node:assert/strict';
import test from 'node:test';
import {
  isPaletteHotkeyReserved,
  registerPaletteHotkey,
} from './palette-shortcuts';

test('a failed Linux shortcut registers a recovery shortcut that reopens the palette after dismissal', () => {
  const registered = new Map<string, () => void>();
  let visible = true;
  const togglePalette = () => {
    visible = !visible;
  };
  const status = registerPaletteHotkey({
    accelerator: 'Alt+Space',
    platform: 'linux',
    register: (accelerator, listener) => {
      if (accelerator !== 'Control+Alt+K') return false;
      registered.set(accelerator, listener);
      return true;
    },
    listener: togglePalette,
  });

  assert.deepEqual(status, {
    accelerator: 'Alt+Space',
    registered: false,
    recoveryAccelerator: 'Control+Alt+K',
    platform: 'linux',
  });

  visible = false;
  registered.get('Control+Alt+K')?.();
  assert.equal(visible, true);
});

test('Linux recovery tries another shortcut when the first one is unavailable', () => {
  const attempts: string[] = [];
  const status = registerPaletteHotkey({
    accelerator: 'Alt+Space',
    platform: 'linux',
    register: (accelerator) => {
      attempts.push(accelerator);
      return accelerator === 'Control+Alt+Shift+K';
    },
    listener: () => {},
  });

  assert.deepEqual(attempts, [
    'Alt+Space',
    'Control+Alt+K',
    'Control+Alt+Shift+K',
  ]);
  assert.equal(status.recoveryAccelerator, 'Control+Alt+Shift+K');
});

test('a failed shortcut on other platforms does not register Linux recovery bindings', () => {
  const attempts: string[] = [];
  const status = registerPaletteHotkey({
    accelerator: 'Alt+Space',
    platform: 'darwin',
    register: (accelerator) => {
      attempts.push(accelerator);
      return false;
    },
    listener: () => {},
  });

  assert.deepEqual(attempts, ['Alt+Space']);
  assert.equal(status.registered, false);
  assert.equal(status.recoveryAccelerator, undefined);
});

test('reports when all Linux recovery shortcuts are unavailable', () => {
  const status = registerPaletteHotkey({
    accelerator: 'Alt+Space',
    platform: 'linux',
    register: () => false,
    listener: () => {},
  });

  assert.deepEqual(status, {
    accelerator: 'Alt+Space',
    registered: false,
    platform: 'linux',
  });
});

test('only the active Linux palette recovery shortcut is reserved from actions', () => {
  assert.equal(
    isPaletteHotkeyReserved(
      'Control+Alt+K',
      'Alt+Space',
      'Control+Alt+K',
      'linux',
    ),
    true,
  );
  assert.equal(
    isPaletteHotkeyReserved(
      'Control+Alt+K',
      'Alt+Space',
      'Control+Alt+K',
      'darwin',
    ),
    false,
  );
  assert.equal(
    isPaletteHotkeyReserved('Control+Alt+K', 'Alt+Space', undefined, 'linux'),
    false,
  );
});
