import assert from 'node:assert/strict';
import test from 'node:test';
import { createDefaultPortableConfig } from '../state/portable-config-schema';
import {
  desiredShortcutBindings,
  planShortcutBindings,
} from './shortcut-binding-plan';
import { createShortcutBindings } from './shortcut-bindings';

const NOTHING_RESERVED = () => false;

function fakeGlobalShortcut(unavailable: string[] = []) {
  const registered: string[] = [];
  const unregistered: string[] = [];
  return {
    registered,
    unregistered,
    api: {
      register: (accelerator: string) => {
        if (unavailable.includes(accelerator)) return false;
        registered.push(accelerator);
        return true;
      },
      unregister: (accelerator: string) => {
        unregistered.push(accelerator);
      },
    },
  };
}

test('duplicate user accelerators bind the first action and mark the second inactive', () => {
  const plan = planShortcutBindings(
    [
      {
        actionId: 'extension-action:a:two',
        accelerator: 'Command+Shift+X',
        title: 'Two',
        source: 'user',
      },
      {
        actionId: 'extension-action:a:one',
        accelerator: 'Command+Shift+X',
        title: 'One',
        source: 'user',
      },
    ],
    NOTHING_RESERVED,
  );
  assert.deepEqual(plan.bindings, [
    { actionId: 'extension-action:a:one', accelerator: 'Command+Shift+X' },
  ]);
  assert.deepEqual(
    plan.inactive.map((item) => item.actionId),
    ['extension-action:a:two'],
  );
  assert.match(plan.inactive[0].message, /already used by One/);
});

test('user shortcuts win over declared ones and reserved accelerators stay inactive', () => {
  const plan = planShortcutBindings(
    [
      {
        actionId: 'a',
        accelerator: 'Command+K',
        title: 'Declared',
        source: 'declared',
      },
      {
        actionId: 'b',
        accelerator: 'Command+K',
        title: 'User',
        source: 'user',
      },
      {
        actionId: 'c',
        accelerator: 'Command+Alt+K',
        title: 'Reserved',
        source: 'user',
      },
    ],
    (accelerator) => accelerator === 'Command+Alt+K',
  );
  assert.deepEqual(
    plan.bindings.map((item) => item.actionId),
    ['b'],
  );
  assert.equal(
    plan.inactive.find((item) => item.actionId === 'a')?.message,
    'Command+K is already used by User',
  );
  assert.match(
    plan.inactive.find((item) => item.actionId === 'c')?.message ?? '',
    /reserved/,
  );
});

test('declared shortcuts are hidden by a user override or a disabled marker, and dormant IDs are skipped', () => {
  const view = {
    ...createDefaultPortableConfig(),
    shortcuts: {
      'extension-action:a:overridden': 'Command+Shift+O',
      'extension-action:a:disabled': null,
      'extension-action:gone:missing': 'Command+Shift+M',
    },
  };
  const desired = desiredShortcutBindings(
    view,
    (actionId) =>
      actionId.startsWith('extension-action:gone') ? null : { title: actionId },
    [
      {
        actionId: 'extension-action:a:overridden',
        accelerator: 'Command+Shift+Z',
        action: { title: 'x' },
      },
      {
        actionId: 'extension-action:a:disabled',
        accelerator: 'Command+Shift+D',
        action: { title: 'x' },
      },
      {
        actionId: 'extension-action:a:declared',
        accelerator: 'Command+Shift+Q',
        action: { title: 'Declared' },
      },
    ],
  );
  assert.deepEqual(
    desired.map((item) => `${item.source}:${item.actionId}`),
    [
      'user:extension-action:a:overridden',
      'declared:extension-action:a:declared',
    ],
  );
});

test('releasing one action never unregisters another action’s binding', () => {
  const shortcut = fakeGlobalShortcut();
  const bindings = createShortcutBindings({
    globalShortcut: shortcut.api,
    runAction: () => undefined,
  });
  bindings.apply(
    [
      {
        actionId: 'a',
        accelerator: 'Command+Shift+1',
        title: 'A',
        source: 'user',
      },
      {
        actionId: 'b',
        accelerator: 'Command+Shift+2',
        title: 'B',
        source: 'user',
      },
    ],
    NOTHING_RESERVED,
  );
  bindings.apply(
    [
      {
        actionId: 'b',
        accelerator: 'Command+Shift+2',
        title: 'B',
        source: 'user',
      },
    ],
    NOTHING_RESERVED,
  );
  assert.deepEqual(shortcut.unregistered, ['Command+Shift+1']);
  assert.deepEqual(shortcut.registered, ['Command+Shift+1', 'Command+Shift+2']);
});

test('a binding the OS refuses is reported as a failure and not tracked as active', () => {
  const shortcut = fakeGlobalShortcut(['Command+Shift+9']);
  const bindings = createShortcutBindings({
    globalShortcut: shortcut.api,
    runAction: () => undefined,
  });
  const failures = bindings.apply(
    [
      {
        actionId: 'a',
        accelerator: 'Command+Shift+9',
        title: 'A',
        source: 'user',
      },
    ],
    NOTHING_RESERVED,
  );
  assert.equal(failures.get('a'), 'Command+Shift+9 is in use by another app');
  assert.equal(bindings.ownedAccelerators().has('Command+Shift+9'), false);
});

test('a duplicate accelerator never reaches the OS registration as a second binding', () => {
  const shortcut = fakeGlobalShortcut();
  const bindings = createShortcutBindings({
    globalShortcut: shortcut.api,
    runAction: () => undefined,
  });
  const failures = bindings.apply(
    [
      {
        actionId: 'a',
        accelerator: 'Command+Shift+5',
        title: 'A',
        source: 'user',
      },
      {
        actionId: 'b',
        accelerator: 'Command+Shift+5',
        title: 'B',
        source: 'user',
      },
    ],
    NOTHING_RESERVED,
  );
  assert.deepEqual(shortcut.registered, ['Command+Shift+5']);
  assert.deepEqual(shortcut.unregistered, []);
  assert.match(failures.get('b') ?? '', /already used by A/);
});
