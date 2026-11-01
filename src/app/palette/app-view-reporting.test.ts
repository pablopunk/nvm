import assert from 'node:assert/strict';
import test from 'node:test';
import { settledAppView, type ViewExposure } from './app-view-reporting';
import { extensionLoadingView } from './extension-loading-view';
import type { PaneFrame } from './extension-view/pane-navigation';
import type { CommandView } from './model';

const HIDDEN: ViewExposure = { mode: 'hidden' };
const PALETTE: ViewExposure = { mode: 'palette' };
const SHORTCUT: ViewExposure = { mode: 'shortcut' };

function view(overrides: Partial<CommandView> = {}): CommandView {
  return {
    type: 'list',
    id: 'view',
    title: 'View',
    items: [],
    ...overrides,
  } as CommandView;
}

function frame(
  views: CommandView[],
  options: { id?: string; entryIds?: string[] } = {},
): PaneFrame {
  return {
    id: options.id ?? 'frame',
    generation: 0,
    primary: true,
    history: views.map((item, index) => ({
      id: options.entryIds?.[index] ?? `entry-${index}`,
      view: item,
      query: '',
      selection: '',
      formValues: {},
    })),
  };
}

test('a hidden window never counts as a view', () => {
  assert.equal(settledAppView(HIDDEN, undefined), null);
  assert.equal(settledAppView(HIDDEN, frame([view()])), null);
});

test('the root palette counts only when the palette itself was shown', () => {
  assert.deepEqual(settledAppView(PALETTE, undefined), {
    view: 'palette',
    key: 'palette',
  });
  assert.equal(settledAppView(SHORTCUT, undefined), null);
});

test('a shortcut counts when its view resolves, not while it loads', () => {
  const loading = frame([extensionLoadingView()]);
  const resolved = frame([view({ id: 'real' })]);
  assert.equal(settledAppView(SHORTCUT, loading), null);
  assert.equal(settledAppView(SHORTCUT, resolved)?.view, 'extension');
});

test('a view retained from before a shortcut is not the shortcut view', () => {
  const retainedChat = frame([view({ id: 'chat', aiChat: true })], {
    entryIds: ['retained'],
  });
  const exposure: ViewExposure = { mode: 'shortcut', staleEntryId: 'retained' };
  assert.equal(settledAppView(exposure, retainedChat), null);
  assert.equal(
    settledAppView(exposure, frame([view({ id: 'shortcut-view' })]))?.view,
    'extension',
  );
});

test('refreshing or replacing a view with itself keeps its key', () => {
  const first = settledAppView(PALETTE, frame([view()], { entryIds: ['a'] }));
  const replaced = settledAppView(
    PALETTE,
    frame([view({ title: 'Renamed' })], { entryIds: ['b'] }),
  );
  assert.equal(first?.key, replaced?.key);
});

test('pushing, popping and switching views each get their own key', () => {
  const root = view({ id: 'root' });
  const child = view({ id: 'child' });
  const keys = [
    frame([root]),
    frame([root, child]),
    frame([root]),
    frame([view({ id: 'other' })]),
  ].map((item) => settledAppView(PALETTE, item)?.key);
  assert.equal(new Set(keys).size, 3);
});

test('pushing a view with the same id still counts as a navigation', () => {
  const same = view({ id: 'same' });
  const shallow = settledAppView(PALETTE, frame([same]));
  const deep = settledAppView(PALETTE, frame([same, same]));
  assert.notEqual(shallow?.key, deep?.key);
});

test('AI chat is classified from the active view only', () => {
  assert.equal(
    settledAppView(PALETTE, frame([view({ aiChat: true })]))?.view,
    'ai-chat',
  );
  assert.equal(settledAppView(PALETTE, frame([view()]))?.view, 'extension');
});
