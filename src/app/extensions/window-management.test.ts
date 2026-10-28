// biome-ignore-all lint/suspicious/noExplicitAny: built-in extension contexts are intentionally untyped at this bridge boundary.
import assert from 'node:assert/strict';
import test from 'node:test';
import { WINDOW_LAYOUT_IDS } from '../electron/os/window-layout-catalog';
import { createWindowManagementExtension } from './window-management';
import { WINDOW_LAYOUT_PRESENTATION } from './window-management-layout-presentation';
import { fakeWindowManagementContext } from './window-management-test-doubles';

const P1_PERSISTED_IDS = [
  'extension-action:window-management:layout.left-half',
  'extension-action:window-management:layout.right-half',
  'extension-action:window-management:layout.top-half',
  'extension-action:window-management:layout.bottom-half',
  'extension-action:window-management:layout.maximize',
  'extension-action:window-management:layout.center',
];

const EXACT_AND_DESKTOP_PERSISTED_IDS = [
  'extension-action:window-management:move-window',
  'extension-action:window-management:resize-window',
  'extension-action:window-management:next-desktop',
  'extension-action:window-management:previous-desktop',
];

const COMMAND_PERSISTED_IDS = [
  'extension:window-management:settings',
  'extension:window-management:custom-commands',
];

function contextFor(hasWindowManagement: boolean) {
  return fakeWindowManagementContext({
    withWindowManagementCapability: hasWindowManagement,
  }).ctx;
}

function actionsFor(hasWindowManagement: boolean) {
  return createWindowManagementExtension().actions(
    contextFor(hasWindowManagement) as any,
  ) as Array<{ id: string; title: string; icon: string }>;
}

function persistedIdOf(localId: string) {
  return `extension-action:window-management:${localId}`;
}

function layoutActionsFor(hasWindowManagement: boolean) {
  return actionsFor(hasWindowManagement).filter((action) =>
    action.id.startsWith('layout.'),
  );
}

test('persisted ids of the P1 layouts are unchanged', () => {
  const persistedIds = layoutActionsFor(true).map((action) =>
    persistedIdOf(action.id),
  );
  for (const expectedId of P1_PERSISTED_IDS) {
    assert.ok(persistedIds.includes(expectedId), expectedId);
  }
});

test('every catalog layout is contributed exactly once as a durable action', () => {
  assert.deepEqual(
    layoutActionsFor(true).map((action) => action.id),
    WINDOW_LAYOUT_IDS.map((id) => `layout.${id}`),
  );
});

test('each layout action carries its presentation title and icon', () => {
  assert.deepEqual(
    layoutActionsFor(true).map((action) => [action.title, action.icon]),
    WINDOW_LAYOUT_IDS.map((id) => [
      WINDOW_LAYOUT_PRESENTATION[id].title,
      WINDOW_LAYOUT_PRESENTATION[id].icon,
    ]),
  );
});

test('layout actions are omitted when window management is unsupported', () => {
  assert.deepEqual(actionsFor(false), []);
});

test('Move, Resize, and desktop actions get persisted ids that never change', () => {
  const persistedIds = actionsFor(true)
    .filter((action) => !action.id.startsWith('layout.'))
    .map((action) => persistedIdOf(action.id));
  assert.deepEqual(persistedIds, EXACT_AND_DESKTOP_PERSISTED_IDS);
});

test('root commands keep their persisted ids', () => {
  const commandIds = (createWindowManagementExtension().commands as any[]).map(
    (command) => `extension:window-management:${command.id}`,
  );
  assert.deepEqual(commandIds, COMMAND_PERSISTED_IDS);
});

test('Move, Resize, and desktop actions are hidden when the capability is absent', () => {
  const titles = actionsFor(false).map((action) => action.title);
  assert.equal(titles.includes('Move Window…'), false);
  assert.equal(titles.includes('Resize Window…'), false);
  assert.equal(titles.includes('Next Desktop'), false);
  assert.equal(titles.includes('Previous Desktop'), false);
});

test('declares window management and system capabilities for review', () => {
  assert.deepEqual(createWindowManagementExtension().capabilities, [
    'desktop.window-management',
    'system',
  ]);
});

test('contributes the Window Management Settings root command first', () => {
  const [settingsCommand] = createWindowManagementExtension().commands as any[];
  assert.equal(settingsCommand.id, 'settings');
  assert.equal(settingsCommand.title, 'Window Management Settings');
  assert.equal(typeof settingsCommand.run, 'function');
});

test('contributes the Custom Window Commands root command', () => {
  const customCommand = (
    createWindowManagementExtension().commands as any[]
  )[1];
  assert.equal(customCommand.id, 'custom-commands');
  assert.equal(customCommand.title, 'Custom Window Commands');
  assert.equal(typeof customCommand.run, 'function');
});

test('root search contributes no custom command rows for an empty query', async () => {
  const extension = createWindowManagementExtension();
  const items = await extension.searchItems(contextFor(true) as any, '');
  assert.deepEqual(items, []);
});
