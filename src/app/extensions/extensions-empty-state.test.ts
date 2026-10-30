import assert from 'node:assert/strict';
import test from 'node:test';
import {
  extensionsEmptyState,
  SIGNED_IN_EXTENSIONS_EMPTY_VIEW,
} from './extensions-empty-state';

const signInAction = { type: 'runExtensionAction', title: 'Log in' };

test('signed-out users get a sign-in row that reuses the account flow', () => {
  const state = extensionsEmptyState({
    id: 'account-login',
    primaryAction: signInAction,
  });

  assert.equal(state.items.length, 1);
  assert.equal(state.items[0]?.title, 'Sign in to build extensions');
  assert.equal(state.items[0]?.primaryAction, signInAction);
});

test('signed-in users get an accurate empty view without a sign-in prompt', () => {
  const state = extensionsEmptyState({
    id: 'account-logout',
    primaryAction: { type: 'runExtensionAction', title: 'Log out' },
  });

  assert.deepEqual(state.items, []);
  assert.deepEqual(state.emptyView, SIGNED_IN_EXTENSIONS_EMPTY_VIEW);
  assert.doesNotMatch(state.emptyView.title, /local extensions found/i);
});

test('the signed-out empty state is clone-safe once the host strips handlers', () => {
  const state = extensionsEmptyState({
    id: 'account-login',
    primaryAction: signInAction,
  });

  assert.doesNotThrow(() => structuredClone(state));
});
