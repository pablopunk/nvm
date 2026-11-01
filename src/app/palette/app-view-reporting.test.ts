import assert from 'node:assert/strict';
import test from 'node:test';
import { settledAppView } from './app-view-reporting';
import { extensionLoadingView } from './extension-loading-view';
import type { CommandView } from './model';

function view(overrides: Partial<CommandView> = {}): CommandView {
  return {
    type: 'list',
    id: 'view',
    title: 'View',
    items: [],
    ...overrides,
  } as CommandView;
}

test('a hidden window never counts as a view', () => {
  assert.equal(settledAppView('hidden', null), null);
  assert.equal(settledAppView('hidden', { id: 'a', view: view() }), null);
});

test('the root palette counts only when the palette itself was shown', () => {
  assert.deepEqual(settledAppView('palette', null), {
    view: 'palette',
    key: 'palette',
  });
  assert.equal(settledAppView('shortcut', null), null);
});

test('a shortcut counts when its view resolves, not while it loads', () => {
  const loading = { id: 'pane:view', view: extensionLoadingView() };
  const resolved = { id: 'pane:view', view: view({ id: 'real' }) };
  assert.equal(settledAppView('shortcut', loading), null);
  assert.deepEqual(settledAppView('shortcut', resolved), {
    view: 'extension',
    key: 'extension:pane:view',
  });
});

test('refreshing a view keeps its navigation key', () => {
  const first = settledAppView('palette', {
    id: 'pane',
    view: view({ items: [] }),
  });
  const refreshed = settledAppView('palette', {
    id: 'pane',
    view: view({ title: 'Renamed' }),
  });
  assert.equal(first?.key, refreshed?.key);
});

test('each visible navigation gets its own key', () => {
  const keys = ['a', 'b', 'a'].map(
    (id) => settledAppView('palette', { id, view: view() })?.key,
  );
  assert.deepEqual(keys, ['extension:a', 'extension:b', 'extension:a']);
});

test('AI chat is classified from the active view only', () => {
  assert.equal(
    settledAppView('palette', { id: 'chat', view: view({ aiChat: true }) })
      ?.view,
    'ai-chat',
  );
  assert.equal(
    settledAppView('palette', { id: 'x', view: view() })?.view,
    'extension',
  );
});
