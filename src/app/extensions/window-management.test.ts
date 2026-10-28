// biome-ignore-all lint/suspicious/noExplicitAny: built-in extension contexts are intentionally untyped at this bridge boundary.
import assert from 'node:assert/strict';
import test from 'node:test';
import { WINDOW_LAYOUT_IDS } from '../electron/os/window-layout-catalog';
import { createWindowManagementExtension } from './window-management';
import {
  CUSTOM_COMMANDS_NOTICE,
  CUSTOM_COMMANDS_TITLE,
} from './window-management-custom-commands';
import { WINDOW_LAYOUT_PRESENTATION } from './window-management-layout-presentation';
import { fakeWindowManagementContext } from './window-management-test-doubles';

const SETTINGS_ACTION_TITLE = 'Window Management Settings';

const DESKTOP_LOCAL_IDS = ['next-desktop', 'previous-desktop'];

const EXACT_ACTION_COUNT = 2;

const BUILT_IN_ACTION_COUNT =
  WINDOW_LAYOUT_IDS.length + EXACT_ACTION_COUNT + DESKTOP_LOCAL_IDS.length;

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

async function actionsFor(hasWindowManagement: boolean) {
  const actions = await createWindowManagementExtension().actions(
    contextFor(hasWindowManagement) as any,
  );
  return actions as Array<{
    id: string;
    title: string;
    icon: string;
    actions?: Array<{ title: string }>;
  }>;
}

function persistedIdOf(localId: string) {
  return `extension-action:window-management:${localId}`;
}

async function layoutActionsFor(hasWindowManagement: boolean) {
  const actions = await actionsFor(hasWindowManagement);
  return actions.filter((action) => action.id.startsWith('layout.'));
}

function rootCommands() {
  return createWindowManagementExtension().commands as any[];
}

test('persisted ids of the P1 layouts are unchanged', async () => {
  const persistedIds = (await layoutActionsFor(true)).map((action) =>
    persistedIdOf(action.id),
  );
  for (const expectedId of P1_PERSISTED_IDS) {
    assert.ok(persistedIds.includes(expectedId), expectedId);
  }
});

test('every catalog layout is contributed exactly once as a durable action', async () => {
  assert.deepEqual(
    (await layoutActionsFor(true)).map((action) => action.id),
    WINDOW_LAYOUT_IDS.map((id) => `layout.${id}`),
  );
});

test('each layout action carries its presentation title and icon', async () => {
  assert.deepEqual(
    (await layoutActionsFor(true)).map((action) => [action.title, action.icon]),
    WINDOW_LAYOUT_IDS.map((id) => [
      WINDOW_LAYOUT_PRESENTATION[id].title,
      WINDOW_LAYOUT_PRESENTATION[id].icon,
    ]),
  );
});

test('no durable actions are contributed when window management is unsupported', async () => {
  assert.deepEqual(await actionsFor(false), []);
});

test('built-in durable actions are exactly the layouts, exact commands, and desktop commands', async () => {
  assert.equal((await actionsFor(true)).length, BUILT_IN_ACTION_COUNT);
});

test('Move, Resize, and desktop actions get persisted ids that never change', async () => {
  const persistedIds = (await actionsFor(true))
    .filter((action) => !action.id.startsWith('layout.'))
    .map((action) => persistedIdOf(action.id));
  assert.deepEqual(persistedIds, EXACT_AND_DESKTOP_PERSISTED_IDS);
});

test('root commands keep their persisted ids', () => {
  const commandIds = rootCommands().map(
    (command) => `extension:window-management:${command.id}`,
  );
  assert.deepEqual(commandIds, COMMAND_PERSISTED_IDS);
});

test('Move, Resize, and desktop actions are hidden when the capability is absent', async () => {
  const titles = (await actionsFor(false)).map((action) => action.title);
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
  const [settingsCommand] = rootCommands();
  assert.equal(settingsCommand.id, 'settings');
  assert.equal(settingsCommand.title, SETTINGS_ACTION_TITLE);
  assert.equal(typeof settingsCommand.run, 'function');
});

test('contributes the Custom Window Commands root command as a static notice', () => {
  const customCommand = rootCommands()[1];
  assert.equal(customCommand.id, 'custom-commands');
  assert.equal(customCommand.title, CUSTOM_COMMANDS_TITLE);
  assert.equal(customCommand.subtitle, 'Window Management');
  assert.equal(customCommand.icon, 'layout-list');
});

test('running Custom Window Commands opens a read-only preview with the AI-builder copy', async () => {
  const view = (await rootCommands()[1].run(contextFor(true))) as any;
  assert.equal(view.type, 'preview');
  assert.equal(view.title, CUSTOM_COMMANDS_TITLE);
  assert.equal(view.content, CUSTOM_COMMANDS_NOTICE);
  assert.equal(view.actions, undefined);
});

test('the AI-builder copy points at the command palette and the Safari and Terminal example', () => {
  assert.ok(CUSTOM_COMMANDS_NOTICE.includes('Coming from Raycast'));
  assert.ok(CUSTOM_COMMANDS_NOTICE.includes('command palette'));
  assert.ok(
    CUSTOM_COMMANDS_NOTICE.includes(
      'open Safari on the left at 60% and Terminal on the right at 40%',
    ),
  );
});

test('every layout and exact action offers one Window Management Settings secondary action', async () => {
  const actions = await actionsFor(true);
  const durableActions = actions.filter(
    (action) => !DESKTOP_LOCAL_IDS.includes(action.id),
  );

  assert.equal(
    durableActions.length,
    WINDOW_LAYOUT_IDS.length + EXACT_ACTION_COUNT,
  );
  for (const action of durableActions) {
    assert.deepEqual(
      (action.actions ?? []).map((secondary) => secondary.title),
      [SETTINGS_ACTION_TITLE],
      action.id,
    );
  }
});

test('desktop actions carry no secondary actions', async () => {
  const actions = await actionsFor(true);
  const desktopActions = actions.filter((action) =>
    DESKTOP_LOCAL_IDS.includes(action.id),
  );

  assert.equal(desktopActions.length, 2);
  for (const action of desktopActions) {
    assert.equal(action.actions, undefined, action.id);
  }
});
