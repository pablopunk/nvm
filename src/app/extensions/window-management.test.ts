// biome-ignore-all lint/suspicious/noExplicitAny: built-in extension contexts are intentionally untyped at this bridge boundary.
import assert from 'node:assert/strict';
import test from 'node:test';
import { WINDOW_LAYOUT_IDS } from '../electron/os/window-layout-catalog';
import { createWindowManagementExtension } from './window-management';
import { WINDOW_LAYOUT_PRESENTATION } from './window-management-layout-presentation';

const P1_PERSISTED_IDS = [
  'extension-action:window-management:layout.left-half',
  'extension-action:window-management:layout.right-half',
  'extension-action:window-management:layout.top-half',
  'extension-action:window-management:layout.bottom-half',
  'extension-action:window-management:layout.maximize',
  'extension-action:window-management:layout.center',
];

function systemContext(hasWindowManagement: boolean) {
  return {
    system: {
      capabilities: {
        has: (capability: string) =>
          hasWindowManagement && capability === 'window-management',
      },
    },
  };
}

function actionsFor(hasWindowManagement: boolean) {
  return createWindowManagementExtension().actions(
    systemContext(hasWindowManagement) as any,
  ) as Array<{ id: string; title: string; icon: string }>;
}

function persistedIdOf(localId: string) {
  return `extension-action:window-management:${localId}`;
}

test('persisted ids of the P1 layouts are unchanged', () => {
  const persistedIds = actionsFor(true).map((action) =>
    persistedIdOf(action.id),
  );
  for (const expectedId of P1_PERSISTED_IDS) {
    assert.ok(persistedIds.includes(expectedId), expectedId);
  }
});

test('every catalog layout is contributed exactly once as a durable action', () => {
  assert.deepEqual(
    actionsFor(true).map((action) => action.id),
    WINDOW_LAYOUT_IDS.map((id) => `layout.${id}`),
  );
});

test('each layout action carries its presentation title and icon', () => {
  assert.deepEqual(
    actionsFor(true).map((action) => [action.title, action.icon]),
    WINDOW_LAYOUT_IDS.map((id) => [
      WINDOW_LAYOUT_PRESENTATION[id].title,
      WINDOW_LAYOUT_PRESENTATION[id].icon,
    ]),
  );
});

test('layout actions are omitted when window management is unsupported', () => {
  assert.deepEqual(actionsFor(false), []);
});

test('declares the desktop.window-management capability', () => {
  assert.deepEqual(createWindowManagementExtension().capabilities, [
    'desktop.window-management',
  ]);
});

test('contributes the Window Management Settings root command', () => {
  const [settingsCommand] = createWindowManagementExtension().commands as any[];
  assert.equal(settingsCommand.id, 'settings');
  assert.equal(settingsCommand.title, 'Window Management Settings');
  assert.equal(typeof settingsCommand.run, 'function');
});
