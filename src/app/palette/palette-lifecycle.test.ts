import assert from 'node:assert/strict';
import test from 'node:test';
import {
  resetTransientPaletteState,
  rootResultSelection,
} from './palette-lifecycle';

test('keeps untouched root selection on the first progressive result', () => {
  assert.equal(
    rootResultSelection({
      actionIds: ['new-first', 'old-first'],
      currentSelection: 'old-first',
      previousFirstActionId: 'old-first',
      queryChanged: false,
    }),
    'new-first',
  );
});

test('preserves a root selection moved away from the first result', () => {
  assert.equal(
    rootResultSelection({
      actionIds: ['new-first', 'selected'],
      currentSelection: 'selected',
      previousFirstActionId: 'old-first',
      queryChanged: false,
    }),
    'selected',
  );
});

test('resets root selection when the query changes', () => {
  assert.equal(
    rootResultSelection({
      actionIds: ['new-first', 'selected'],
      currentSelection: 'selected',
      previousFirstActionId: 'old-first',
      queryChanged: true,
    }),
    'new-first',
  );
});

test('resets every transient palette surface', () => {
  const resets: [string, unknown][] = [];
  const record = (name: string) => (value: unknown) => {
    resets.push([name, value]);
  };

  resetTransientPaletteState({
    setOptionsFor: record('options'),
    setExtensionItemOptionsFor: record('extension item options'),
    setConfirmRemoveFor: record('remove confirmation'),
    setPreviewFor: record('preview'),
    setChildQuery: record('child query'),
    setShortcutFor: record('shortcut editor'),
    setRecordedShortcut: record('recorded shortcut'),
    setShortcutManagerOpen: record('shortcut manager'),
    setShortcutOptionsFor: record('shortcut options'),
    setAliasFor: record('alias editor'),
    setConfirmViewActionFor: record('view action confirmation'),
    setActionSubmenuFor: record('action submenu'),
  });

  assert.deepEqual(resets, [
    ['options', null],
    ['extension item options', null],
    ['remove confirmation', null],
    ['preview', null],
    ['child query', ''],
    ['shortcut editor', null],
    ['recorded shortcut', ''],
    ['shortcut manager', false],
    ['shortcut options', null],
    ['alias editor', null],
    ['view action confirmation', null],
    ['action submenu', null],
  ]);
});

test('does not reset preserved AI chat state', () => {
  const aiChat = {
    messages: ['still here'],
    viewId: 'chat-view',
    backStack: ['previous-chat-view'],
  };
  const ignore = (_value: unknown) => undefined;

  resetTransientPaletteState({
    setOptionsFor: ignore,
    setExtensionItemOptionsFor: ignore,
    setConfirmRemoveFor: ignore,
    setPreviewFor: ignore,
    setChildQuery: ignore,
    setShortcutFor: ignore,
    setRecordedShortcut: ignore,
    setShortcutManagerOpen: ignore,
    setShortcutOptionsFor: ignore,
    setAliasFor: ignore,
    setConfirmViewActionFor: ignore,
    setActionSubmenuFor: ignore,
  });

  assert.deepEqual(aiChat, {
    messages: ['still here'],
    viewId: 'chat-view',
    backStack: ['previous-chat-view'],
  });
});
