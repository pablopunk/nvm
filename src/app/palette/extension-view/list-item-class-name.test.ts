import assert from 'node:assert/strict';
import test from 'node:test';
import { extensionListItemClassName } from './list-item-class-name';

test('selectable items keep the compact single-line subtitle', () => {
  assert.equal(
    extensionListItemClassName('converter', {}),
    'extensionListItem',
  );
});

test('contextual rows such as errors render legibly and wrap their guidance', () => {
  assert.equal(
    extensionListItemClassName('extension-error', { disabled: true }),
    'extensionListItem extensionListItemWrapSubtitle extensionListItemContext',
  );
});

test('permission guidance wraps without becoming contextual', () => {
  assert.equal(
    extensionListItemClassName('os-permissions', {}),
    'extensionListItem extensionListItemWrapSubtitle',
  );
});
