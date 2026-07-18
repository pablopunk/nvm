import assert from 'node:assert/strict';
import test from 'node:test';
import { createExtensionWindowActions } from './extension-window-actions';

const view = { id: 'view-id', title: 'View title', type: 'detail' };

test('preserves every released create window action form', () => {
  const { create } = createExtensionWindowActions();

  assert.deepEqual(create(view), {
    dismissAfterRun: 'auto',
    type: 'createWindow',
    title: 'View title',
    view,
    windowOptions: {},
    windowId: 'view-id',
  });
  assert.deepEqual(create(view, { id: 'option-id', title: 'Option title' }), {
    dismissAfterRun: 'auto',
    type: 'createWindow',
    title: 'Option title',
    view,
    windowOptions: { id: 'option-id', title: 'Option title' },
    windowId: 'option-id',
  });
  assert.equal(create({ type: 'detail' }).title, 'Open Window');
});

test('preserves every released show, hide, and close action form', () => {
  const actions = createExtensionWindowActions();
  const cases = [
    ['show', 'showWindow', 'Show Window'],
    ['hide', 'hideWindow', 'Hide Window'],
    ['close', 'closeWindow', 'Close Window'],
  ] as const;

  for (const [helper, type, defaultTitle] of cases) {
    assert.deepEqual(actions[helper]('window-id'), {
      dismissAfterRun: 'auto',
      type,
      title: defaultTitle,
      windowId: 'window-id',
    });
    assert.deepEqual(
      actions[helper]('window-id', 'Custom title', {
        shortcut: 'Command+1',
      }),
      {
        dismissAfterRun: 'auto',
        shortcut: 'Command+1',
        type,
        title: 'Custom title',
        windowId: 'window-id',
      },
    );
  }
});

test('preserves every released string toggle overload', () => {
  const { toggle } = createExtensionWindowActions();
  const cases = [
    {
      args: ['window-id'] as const,
      expected: {
        dismissAfterRun: 'auto',
        type: 'toggleWindow',
        title: 'Toggle Window',
        windowId: 'window-id',
        windowOptions: {},
      },
    },
    {
      args: ['window-id', 'Custom title'] as const,
      expected: {
        dismissAfterRun: 'auto',
        type: 'toggleWindow',
        title: 'Custom title',
        windowId: 'window-id',
        windowOptions: {},
      },
    },
    {
      args: ['window-id', { title: 'Option title', width: 640 }] as const,
      expected: {
        dismissAfterRun: 'auto',
        type: 'toggleWindow',
        title: 'Option title',
        windowId: 'window-id',
        windowOptions: { title: 'Option title', width: 640 },
      },
    },
    {
      args: [
        'window-id',
        'Custom title',
        { width: 640, shortcut: 'Command+1' },
      ] as const,
      expected: {
        dismissAfterRun: 'auto',
        width: 640,
        shortcut: 'Command+1',
        type: 'toggleWindow',
        title: 'Custom title',
        windowId: 'window-id',
        windowOptions: { width: 640, shortcut: 'Command+1' },
      },
    },
  ];

  for (const { args, expected } of cases)
    assert.deepEqual(
      toggle(...(args as unknown as Parameters<typeof toggle>)),
      expected,
    );
});

test('preserves every released view toggle overload', () => {
  const { toggle } = createExtensionWindowActions();
  const cases = [
    {
      args: [view] as const,
      expected: {
        dismissAfterRun: 'auto',
        type: 'toggleWindow',
        title: 'Toggle Window',
        view,
        windowOptions: {},
        windowId: 'view-id',
      },
    },
    {
      args: [view, 'Custom title'] as const,
      expected: {
        dismissAfterRun: 'auto',
        type: 'toggleWindow',
        title: 'Custom title',
        view,
        windowOptions: {},
        windowId: 'view-id',
      },
    },
    {
      args: [view, { id: 'option-id', title: 'Option title' }] as const,
      expected: {
        dismissAfterRun: 'auto',
        type: 'toggleWindow',
        title: 'Option title',
        view,
        windowOptions: { id: 'option-id', title: 'Option title' },
        windowId: 'option-id',
      },
    },
    {
      args: [view, 'Custom title', { id: 'option-id', width: 640 }] as const,
      expected: {
        dismissAfterRun: 'auto',
        type: 'toggleWindow',
        title: 'Custom title',
        view,
        windowOptions: { id: 'option-id', width: 640 },
        windowId: 'option-id',
      },
    },
  ];

  for (const { args, expected } of cases)
    assert.deepEqual(
      toggle(...(args as unknown as Parameters<typeof toggle>)),
      expected,
    );
});
