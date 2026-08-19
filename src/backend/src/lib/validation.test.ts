import assert from 'node:assert/strict';
import { afterEach, test } from 'node:test';
import { z } from 'zod';
import { safeJsonBody } from './validation';

const schema = z.object({ name: z.string().min(1) });

function makeRequest(body: unknown, url = 'https://nvm.fyi/api/test'): Request {
  return new Request(url, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify(body),
  });
}

function makeRequestRawBody(raw: string, url = 'https://nvm.fyi/api/test'): Request {
  return new Request(url, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: raw,
  });
}

afterEach(() => {
  delete process.env.NEVERMIND_VALIDATION_MODE;
});

test('returns ok with parsed data for a valid body', async function validBody() {
  const result = await safeJsonBody(makeRequest({ name: 'test' }), schema);
  assert.equal(result.ok, true);
  if (result.ok) assert.deepEqual(result.data, { name: 'test' });
});

test('returns error for invalid JSON in strict mode', async function invalidJsonStrict() {
  process.env.NEVERMIND_VALIDATION_MODE = 'strict';
  const result = await safeJsonBody(makeRequestRawBody('not json'), schema);
  assert.equal(result.ok, false);
  if (!result.ok) assert.equal(result.error.error.type, 'invalid_request');
});

test('returns error for schema rejection in strict mode', async function schemaReject() {
  process.env.NEVERMIND_VALIDATION_MODE = 'strict';
  const result = await safeJsonBody(makeRequest({}), schema);
  assert.equal(result.ok, false);
  if (!result.ok) {
    assert.equal(result.error.error.type, 'invalid_request');
    assert.ok(Array.isArray(result.error.error.issues));
  }
});

test('returns error for empty body with required field in strict mode', async function emptyBody() {
  process.env.NEVERMIND_VALIDATION_MODE = 'strict';
  const result = await safeJsonBody(makeRequest({}), schema);
  assert.equal(result.ok, false);
});

test('warn mode returns raw body when schema rejects', async function warnModeReject() {
  process.env.NEVERMIND_VALIDATION_MODE = 'warn';
  const result = await safeJsonBody(makeRequest({}), schema);
  assert.equal(result.ok, true);
  if (result.ok) assert.deepEqual(result.data, {});
});

test('off mode returns raw body when schema rejects', async function offModeReject() {
  process.env.NEVERMIND_VALIDATION_MODE = 'off';
  const result = await safeJsonBody(makeRequest({}), schema);
  assert.equal(result.ok, true);
  if (result.ok) assert.deepEqual(result.data, {});
});

test('off mode returns raw body for invalid JSON', async function offModeInvalidJson() {
  process.env.NEVERMIND_VALIDATION_MODE = 'off';
  const result = await safeJsonBody(makeRequestRawBody('not json'), schema);
  assert.equal(result.ok, true);
  if (result.ok) assert.deepEqual(result.data, {});
});

test('warn mode falls through on invalid JSON', async function warnModeInvalidJson() {
  process.env.NEVERMIND_VALIDATION_MODE = 'warn';
  const result = await safeJsonBody(makeRequestRawBody('not json'), schema);
  assert.equal(result.ok, true);
  if (result.ok) assert.deepEqual(result.data, {});
});
