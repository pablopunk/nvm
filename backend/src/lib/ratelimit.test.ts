import assert from 'node:assert/strict';
import { afterEach, test, beforeEach } from 'node:test';
import { rateLimitChat, rateLimitIp, setRateLimitOverridesForTests, resetRateLimitOverridesForTests } from './ratelimit';

beforeEach(() => {
  resetRateLimitOverridesForTests();
});

afterEach(() => {
  delete process.env.UPSTASH_REDIS_REST_URL;
  delete process.env.UPSTASH_REDIS_REST_TOKEN;
  delete process.env.KV_REST_API_URL;
  delete process.env.KV_REST_API_TOKEN;
  delete process.env.VERCEL_ENV;
  delete process.env.NODE_ENV;
  resetRateLimitOverridesForTests();
});

test('rate limiting allows requests in dev when Redis is not configured', async () => {
  const decision = await rateLimitChat('user_1', 'free');
  assert.deepEqual(decision, { ok: true });
});

test('rate limiting allows IP requests in dev when Redis is not configured', async () => {
  const decision = await rateLimitIp('auth', '1.2.3.4');
  assert.deepEqual(decision, { ok: true });
});

test('rate limiting denies chat requests in production when Redis is not configured (fail closed)', async () => {
  process.env.VERCEL_ENV = 'production';
  const decision = await rateLimitChat('user_1', 'free');
  assert.equal(decision.ok, false);
  assert.equal((decision as any).scope, 'chat:free:misconfigured');
});

test('rate limiting denies IP requests in production when Redis is not configured (fail closed)', async () => {
  process.env.VERCEL_ENV = 'production';
  const decision = await rateLimitIp('auth', '1.2.3.4');
  assert.equal(decision.ok, false);
  assert.equal((decision as any).scope, 'ip:auth:misconfigured');
});

test('rate limiting denies in production via NODE_ENV when Redis is not configured', async () => {
  process.env.NODE_ENV = 'production';
  const decision = await rateLimitChat('user_1', 'paid');
  assert.equal(decision.ok, false);
  assert.equal((decision as any).scope, 'chat:paid:misconfigured');
});

test('non-production VERCEL_ENV is permissive when Redis is missing', async () => {
  process.env.VERCEL_ENV = 'preview';
  const decision = await rateLimitChat('user_1', 'free');
  assert.deepEqual(decision, { ok: true });
});

test('test overrides bypass Redis checks entirely', async () => {
  process.env.VERCEL_ENV = 'production';
  setRateLimitOverridesForTests({
    chat: async () => ({ ok: true }),
    ip: async () => ({ ok: false, scope: 'ip:test', retryAfterSec: 10 }),
  });

  const chatDecision = await rateLimitChat('user_1', 'free');
  assert.deepEqual(chatDecision, { ok: true });

  const ipDecision = await rateLimitIp('auth', '1.2.3.4');
  assert.equal(ipDecision.ok, false);
  assert.equal((ipDecision as any).scope, 'ip:test');
});
