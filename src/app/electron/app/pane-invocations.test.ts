import assert from 'node:assert/strict';
import test from 'node:test';
import { PaneInvocations } from './pane-invocations';

test('presentation context is bound to the issuing window and does not accept synthetic IDs', () => {
  const registry = new PaneInvocations();
  registry.register('host-issued', 1, 0);
  assert.equal(registry.owns({ requestId: 'host-issued' }, 1, 1), true);
  assert.equal(registry.owns({ requestId: 'host-issued' }, 2, 1), false);
  assert.equal(registry.owns({ requestId: 'made-up' }, 1, 1), false);
  assert.equal(registry.owns(null, 1, 1), false);
  assert.equal(registry.owns({ requestId: {} }, 1, 1), false);
});

test('hide invalidates only the corresponding window and expired contexts are rejected', () => {
  const registry = new PaneInvocations();
  registry.register('a', 1, 0);
  registry.register('b', 2, 0);
  registry.invalidate(1);
  assert.equal(registry.owns({ requestId: 'a' }, 1, 1), false);
  assert.equal(registry.owns({ requestId: 'b' }, 2, 1), true);
  assert.equal(
    registry.owns({ requestId: 'b' }, 2, 24 * 60 * 60 * 1000 + 1),
    false,
  );
});
