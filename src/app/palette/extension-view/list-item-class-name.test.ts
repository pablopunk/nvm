import assert from 'node:assert/strict';
import test from 'node:test';
import { extensionListItemClassName } from './list-item-class-name';

test('selectable items keep the compact single-line subtitle', () => {
  assert.equal(
    extensionListItemClassName('converter', {}),
    'extensionListItem',
  );
});

test('contextual disabled rows such as errors render legibly and wrap their guidance', () => {
  assert.equal(
    extensionListItemClassName('extension-error', {
      disabled: true,
      contextual: true,
    }),
    'extensionListItem extensionListItemWrapSubtitle extensionListItemContext',
  );
});

test('plain disabled rows keep the dimmed single-line informational styling', () => {
  assert.equal(
    extensionListItemClassName('getting-started', { disabled: true }),
    'extensionListItem',
  );
});

test('contextual styling never applies to a selectable row', () => {
  assert.equal(
    extensionListItemClassName('converter', { contextual: true }),
    'extensionListItem',
  );
});

test('permission guidance wraps without becoming contextual', () => {
  assert.equal(
    extensionListItemClassName('os-permissions', {}),
    'extensionListItem extensionListItemWrapSubtitle',
  );
});
