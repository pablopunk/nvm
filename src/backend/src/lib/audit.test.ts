import assert from 'node:assert/strict';
import { test } from 'node:test';
import { recordAudit, recordAuditStrict } from './audit';

const input = { actorUserId: 'actor', action: 'route_breaker.reenabled' as const };

test('strict audit API propagates executor insert failures', async () => {
  await assert.rejects(
    recordAuditStrict(
      {
        insert() {
          return {
            values() {
              throw new Error('audit insert failed');
            },
          };
        },
      } as any,
      input,
    ),
    /audit insert failed/,
  );
});

test('legacy audit API preserves best-effort behavior', async () => {
  const originalError = console.error;
  console.error = () => {};
  try {
    await recordAudit(input);
  } finally {
    console.error = originalError;
  }
});
