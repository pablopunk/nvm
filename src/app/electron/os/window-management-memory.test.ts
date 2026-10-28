// biome-ignore-all lint/style/noMagicNumbers: fixture geometry reads best as literals
import assert from 'node:assert/strict';
import test from 'node:test';
import {
  createHarness,
  fail,
  LEFT_HALF,
  NO_OPTIONS,
  shortcutInvocation,
} from './window-management.harness';
import { createSimulatedWindow } from './window-management.simulated-window';
import {
  createWindowMemory,
  MAX_REMEMBERED_WINDOWS,
} from './window-management-memory';
import type { WindowInfo, WindowLayoutSpec } from './window-management-types';

const RESTORE: WindowLayoutSpec = { kind: 'restore' };
const ENTRY = { restoreFrame: { x: 1, y: 2, width: 3, height: 4 } };

test('caps remembered windows and forgets the oldest first', () => {
  const memory = createWindowMemory();
  for (let index = 0; index <= MAX_REMEMBERED_WINDOWS; index += 1) {
    memory.write(`1:${index}`, ENTRY);
  }
  assert.equal(memory.size(), MAX_REMEMBERED_WINDOWS);
  assert.equal(memory.read('1:0'), undefined);
  assert.deepEqual(memory.read(`1:${MAX_REMEMBERED_WINDOWS}`), ENTRY);
});

test('drops only the windows that belong to the given pid', () => {
  const memory = createWindowMemory();
  memory.write('1:5', ENTRY);
  memory.write('12:5', ENTRY);
  memory.dropPid(1);
  assert.equal(memory.read('1:5'), undefined);
  assert.deepEqual(memory.read('12:5'), ENTRY);
});

for (const code of ['target-lost', 'no-window'] as const) {
  test(`forgets the remembered frame after the focused window reports ${code}`, async () => {
    const simulation = createSimulatedWindow({});
    let lost = false;
    const { service } = createHarness({
      focusedRead: () =>
        lost
          ? Promise.resolve(fail<WindowInfo>(code, 'gone'))
          : simulation.focusedRead(),
    });
    await service.apply(LEFT_HALF, NO_OPTIONS, shortcutInvocation());

    lost = true;
    const lostResult = await service.apply(
      LEFT_HALF,
      NO_OPTIONS,
      shortcutInvocation(),
    );
    assert.equal(lostResult.ok === false && lostResult.reason, code);

    lost = false;
    const restore = await service.apply(
      RESTORE,
      NO_OPTIONS,
      shortcutInvocation(),
    );
    assert.equal(restore.ok === false && restore.reason, 'no-history');
  });
}
