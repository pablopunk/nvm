import assert from 'node:assert/strict';
import test from 'node:test';
import { diagnosticEnvelope } from './diagnostic-envelope';
import { failureRecord } from './observability';

test('transport rejects attachments, unknown envelopes and raw payload fields', () => {
  const record = failureRecord();
  const envelope = diagnosticEnvelope(
    [
      { trace: { private: 'PRIVATE_TRACE' } } as never,
      [
        [
          { type: 'event' },
          {
            event_id: 'a'.repeat(32),
            tags: { support_ref: record.reference },
            extra: { secret: 'PRIVATE_EXTRA' },
            breadcrumbs: [
              {
                category: 'command.execute.invoke',
                timestamp: 1,
                message: 'PRIVATE_CRUMB',
              },
              { category: 'PRIVATE_CATEGORY', timestamp: 1 },
            ],
          },
        ],
        [{ type: 'attachment' }, new TextEncoder().encode('PRIVATE_MEMORY')],
        [{ type: 'session' }, { private: 'PRIVATE_SESSION' }],
        [{ type: 'replay_event' }, { private: 'PRIVATE_SCREEN' }],
      ],
    ],
    (reference) => (reference === record.reference ? record : undefined),
    'test',
    'test',
  );
  assert.equal(envelope?.[1].length, 1);
  assert.ok(!JSON.stringify(envelope).includes('PRIVATE_'));
});

test('transport does not accept unregistered support identities', () => {
  assert.equal(
    diagnosticEnvelope(
      [{}, [[{ type: 'event' }, { tags: { support_ref: 'a'.repeat(32) } }]]],
      () => undefined,
      'test',
      'test',
    ),
    undefined,
  );
});
