import assert from 'node:assert/strict';
import test from 'node:test';
import type { CommandView } from './model';
import {
  activePane,
  dismissPane,
  hidePanes,
  navigatePane,
  openPane,
  ownerIsCurrent,
  paneOwner,
  paneView,
  popPane,
  resolvePane,
} from './pane-navigation';

const chat: CommandView = {
  type: 'chat',
  id: 'chat',
  title: 'Chat',
  aiChat: true,
  chatId: 'chat-session',
};
const clipboard: CommandView = {
  type: 'list',
  id: 'clipboard-history',
  title: 'Clipboard',
};
const loading: CommandView = {
  type: 'list',
  title: 'Loading',
  isLoading: true,
};

test('Clipboard History retains the exact chat view instance and restores it on dismissal', () => {
  const parent = openPane({ frames: [] }, chat, 'chat-pane');
  const original = paneView(activePane(parent));
  const nested = openPane(parent, clipboard, 'clipboard-pane');
  assert.equal(nested.frames[0], parent.frames[0]);
  const restored = dismissPane(nested, paneOwner(nested)!);
  assert.equal(paneView(activePane(restored)), original);
});

test('parent history, filter, selection, and form state survive nested tools', () => {
  let parent = openPane({ frames: [] }, clipboard, 'parent');
  parent = navigatePane(parent, chat, 'push', 'chat-instance');
  const original = paneView(activePane(parent))!;
  original.query = 'saved filter';
  original.selection = 'selected-row';
  original.formValues = { text: 'draft', enabled: true };
  const nested = openPane(
    openPane(parent, clipboard, 'tool-a'),
    loading,
    'tool-b',
  );
  const restored = popPane(popPane(nested));
  assert.equal(paneView(activePane(restored)), original);
  assert.equal(activePane(restored)?.history.length, 2);
  assert.equal(paneView(activePane(popPane(restored)))?.view, clipboard);
});

test('Escape before hydration makes late completion a no-op', () => {
  const parent = openPane({ frames: [] }, chat, 'chat');
  const pending = openPane(parent, loading, 'tool', { requestId: 'request' });
  const dismissed = popPane(pending);
  assert.equal(resolvePane(dismissed, 'request', clipboard), dismissed);
});

test('out-of-order completion hydrates retained panes without changing the active pane', () => {
  const first = openPane({ frames: [] }, loading, 'a', { requestId: 'a' });
  const second = openPane(first, loading, 'b', { requestId: 'b' });
  const resolvedB = resolvePane(second, 'b', clipboard);
  const resolvedA = resolvePane(resolvedB, 'a', chat);
  assert.equal(activePane(resolvedA)?.id, 'b');
  assert.equal(paneView(resolvedA.frames[0])?.view, chat);
  assert.equal(paneView(activePane(resolvedA))?.view, clipboard);
});

test('no-view resolution removes only its owning middle frame', () => {
  const parent = openPane({ frames: [] }, chat, 'parent');
  const pending = openPane(parent, loading, 'a', { requestId: 'a' });
  const current = openPane(pending, clipboard, 'b');
  const resolved = resolvePane(current, 'a', undefined);
  assert.deepEqual(
    resolved.frames.map((frame) => frame.id),
    ['parent', 'b'],
  );
  assert.equal(paneView(activePane(popPane(resolved)))?.view, chat);
});

test('completion cannot dismiss another pane or a changed navigation generation', () => {
  const initial = openPane({ frames: [] }, clipboard, 'clipboard');
  const owner = paneOwner(initial)!;
  const newer = openPane(initial, chat, 'newer');
  assert.equal(dismissPane(newer, owner), newer);
  const navigated = navigatePane(initial, loading, 'push', 'new-view');
  assert.equal(ownerIsCurrent(navigated, owner), false);
  assert.equal(ownerIsCurrent(popPane(navigated), owner), false);
});

test('passive content patches retain operation ownership', () => {
  const initial = openPane({ frames: [] }, clipboard, 'clipboard');
  const owner = paneOwner(initial)!;
  const frame = activePane(initial)!;
  const entry = paneView(frame)!;
  const refreshed = {
    frames: [
      {
        ...frame,
        history: [{ ...entry, view: { ...entry.view, title: 'Refreshed' } }],
      },
    ],
  };
  assert.equal(ownerIsCurrent(refreshed, owner), true);
});

test('hidden-window startup and hide/reopen discard previous request ownership', () => {
  const old = openPane({ frames: [] }, loading, 'old', { requestId: 'old' });
  const fresh = openPane(old, clipboard, 'fresh', {
    primary: true,
    reset: true,
  });
  assert.equal(fresh.frames.length, 1);
  assert.equal(activePane(fresh)?.primary, true);
  assert.equal(resolvePane(fresh, 'old', chat), fresh);
});

test('hide resumes the retained chat and cancels pending tool hydration and action ownership', () => {
  const parent = openPane({ frames: [] }, chat, 'chat');
  const owner = paneOwner(parent)!;
  const pending = openPane(parent, loading, 'tool', { requestId: 'tool' });
  const hidden = hidePanes(pending);
  assert.equal(paneView(activePane(hidden))?.view, chat);
  assert.equal(ownerIsCurrent(hidden, owner), false);
  assert.equal(resolvePane(hidden, 'tool', clipboard), hidden);
});

test('navigation supersedes pending hydration even when the frame remains retained', () => {
  const pending = openPane({ frames: [] }, loading, 'tool', {
    requestId: 'tool',
  });
  const navigated = navigatePane(pending, clipboard, 'replace', 'replacement');
  assert.equal(resolvePane(navigated, 'tool', chat), navigated);
});

test('a newer operation invalidates completion from an earlier invocation', () => {
  const state = openPane({ frames: [] }, clipboard, 'tool');
  const frame = activePane(state)!;
  const entry = paneView(frame)!;
  entry.operationId = 'first';
  const owner = paneOwner(state)!;
  const next = {
    frames: [{ ...frame, history: [{ ...entry, operationId: 'second' }] }],
  };
  assert.equal(ownerIsCurrent(next, owner), false);
});
