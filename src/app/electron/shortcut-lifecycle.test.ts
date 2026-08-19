import assert from 'node:assert/strict';
import test from 'node:test';
import {
  shortcutActionLifecycle,
  shortcutActionRunsWithoutView,
  withInheritedShortcutLifecycle,
} from './shortcut-lifecycle';

test('recognizes every viewless shortcut lifecycle', () => {
  assert.equal(shortcutActionRunsWithoutView({ background: true }), true);
  assert.equal(shortcutActionRunsWithoutView({ mode: 'background' }), true);
  assert.equal(shortcutActionRunsWithoutView({ mode: 'noView' }), true);
  assert.equal(shortcutActionRunsWithoutView({ mode: 'view' }), false);
});

test('resolves lifecycle metadata through durable action wrappers', () => {
  assert.deepEqual(
    shortcutActionLifecycle({
      persistentAction: { background: true, mode: 'noView' },
    }),
    { background: true, mode: 'noView' },
  );
  assert.equal(
    shortcutActionRunsWithoutView({
      rootAction: {
        persistentAction: { mode: 'noView' },
      },
    }),
    true,
  );
});

test('handles cyclic action wrappers', () => {
  const action: { rootAction?: unknown } = {};
  action.rootAction = action;
  assert.equal(shortcutActionRunsWithoutView(action), false);
});

test('root items inherit durable action lifecycle without overriding their own', () => {
  assert.deepEqual(
    withInheritedShortcutLifecycle(
      { kind: 'extension-root-item' },
      { background: true, mode: 'noView' },
    ),
    {
      kind: 'extension-root-item',
      background: true,
      mode: 'noView',
    },
  );
  assert.deepEqual(
    withInheritedShortcutLifecycle(
      { kind: 'extension-root-item', mode: 'view' },
      { mode: 'noView' },
    ),
    { kind: 'extension-root-item', mode: 'view' },
  );
});
