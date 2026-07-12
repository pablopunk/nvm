import assert from 'node:assert/strict';
import test from 'node:test';
import { requireSameOrigin } from './csrf';
import { normalizeProviderRecipients } from './email';

test('admin waitlist mutations reject cross-origin requests before body parsing', () => {
  const request = new Request('https://nvm.fyi/api/admin/waitlist', { method: 'POST', headers: { Origin: 'https://evil.example' } });
  assert.equal(requireSameOrigin(request)?.status, 403);
});

test('provider recipients use Resend data.to and message mapping without duplicates', () => {
  assert.deepEqual(normalizeProviderRecipients([' User@Example.com ', 'user@example.com'], 'mapped@example.com'), ['user@example.com', 'mapped@example.com']);
});
