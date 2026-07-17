import assert from 'node:assert/strict';
import test from 'node:test';
import { hasEnabledExtensionEventSubscriber } from './frontmost-app-polling';

test('frontmost polling requires an enabled extension subscriber', () => {
  const frontmostTrigger = {
    type: 'event' as const,
    event: 'app.frontmost.changed',
  };

  assert.equal(
    hasEnabledExtensionEventSubscriber(
      [
        { enabled: true, owner: 'host', triggers: [frontmostTrigger] },
        {
          enabled: true,
          owner: 'extension',
          triggers: [{ type: 'event', event: 'clipboard.changed' }],
        },
        { enabled: false, owner: 'extension', triggers: [frontmostTrigger] },
      ],
      'app.frontmost.changed',
    ),
    false,
  );

  assert.equal(
    hasEnabledExtensionEventSubscriber(
      [{ enabled: true, owner: 'extension', triggers: [frontmostTrigger] }],
      'app.frontmost.changed',
    ),
    true,
  );
});
