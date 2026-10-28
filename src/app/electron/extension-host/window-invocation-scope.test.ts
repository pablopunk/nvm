import assert from 'node:assert/strict';
import test from 'node:test';
import type { WindowFocusTarget } from '../os/window-management-types';
import {
  currentWindowInvocationScope,
  runInWindowInvocationScope,
  windowInvocationFromScope,
} from './window-invocation-scope';

const SAFARI: WindowFocusTarget = { bundleId: 'com.apple.Safari', pid: 411 };
const NOTES: WindowFocusTarget = { bundleId: 'com.apple.Notes', pid: 512 };

function refusePaletteTarget(): WindowFocusTarget | null {
  throw new Error('palette target must not be read for shortcut scopes');
}

test('no scope resolves to no invocation', () => {
  assert.equal(
    windowInvocationFromScope(undefined, () => SAFARI),
    null,
  );
});

test('shortcut scope carries its own snapshot without reading the palette', () => {
  assert.deepEqual(
    windowInvocationFromScope(
      { source: 'shortcut', target: SAFARI },
      refusePaletteTarget,
    ),
    { source: 'shortcut', target: SAFARI },
  );
});

test('shortcut scope with no snapshot stays a shortcut invocation', () => {
  assert.deepEqual(
    windowInvocationFromScope(
      { source: 'shortcut', target: null },
      refusePaletteTarget,
    ),
    { source: 'shortcut', target: null },
  );
});

test('palette scope reads the palette target at resolution time', () => {
  let palette: WindowFocusTarget | null = SAFARI;
  const resolve = () =>
    windowInvocationFromScope({ source: 'palette' }, () => palette);
  palette = NOTES;
  assert.deepEqual(resolve(), { source: 'palette', target: NOTES });
});

test('shortcut fired while the palette is visible keeps the shortcut source', () => {
  const paletteVisibleTarget = () => SAFARI;
  const invocation = runInWindowInvocationScope(
    { source: 'shortcut', target: NOTES },
    () =>
      windowInvocationFromScope(
        currentWindowInvocationScope(),
        paletteVisibleTarget,
      ),
  );
  assert.deepEqual(invocation, { source: 'shortcut', target: NOTES });
});

test('scope survives asynchronous hops inside the same run', async () => {
  const observed = await runInWindowInvocationScope(
    { source: 'shortcut', target: SAFARI },
    async () => {
      await new Promise((resolve) => setImmediate(resolve));
      return currentWindowInvocationScope();
    },
  );
  assert.deepEqual(observed, { source: 'shortcut', target: SAFARI });
});

test('scope does not leak past its run', () => {
  runInWindowInvocationScope({ source: 'palette' }, () => undefined);
  assert.equal(currentWindowInvocationScope(), undefined);
});
