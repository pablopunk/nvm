import assert from 'node:assert/strict';
import test from 'node:test';
import { assertPreviewAuthConfiguration, AuthConfigurationError, isProductionGatewayOrigin } from './auth-config';

const keys = ['VERCEL_ENV', 'PRODUCTION_ORIGIN', 'DATABASE_URL', 'PREVIEW_GATEWAY_ORIGIN', 'PREVIEW_START_KEY', 'GATEWAY_STATE_KEY', 'GATEWAY_STATE_REDIS_URL', 'GATEWAY_STATE_REDIS_TOKEN', 'UPSTASH_REDIS_REST_URL', 'UPSTASH_REDIS_REST_TOKEN', 'PREVIEW_SESSION_KEY', 'WORKOS_COOKIE_PASSWORD', 'WORKOS_CLIENT_ID'];
const original = Object.fromEntries(keys.map((key) => [key, process.env[key]]));

function valid() {
  Object.assign(process.env, {
    DATABASE_URL: 'postgres://production', PREVIEW_GATEWAY_ORIGIN: 'https://www.nvm.fyi', PREVIEW_START_KEY: 'preview-start', GATEWAY_STATE_KEY: 'gateway-state', GATEWAY_STATE_REDIS_URL: 'https://gateway', GATEWAY_STATE_REDIS_TOKEN: 'gateway-acl',
    UPSTASH_REDIS_REST_URL: 'https://production-redis', UPSTASH_REDIS_REST_TOKEN: 'production-acl', PREVIEW_SESSION_KEY: 'preview-session',
    WORKOS_COOKIE_PASSWORD: 'production-session', WORKOS_CLIENT_ID: 'prod-client', VERCEL_ENV: 'preview',
  });
}

test('accepts production-faithful Preview auth configuration', () => { valid(); assert.equal(assertPreviewAuthConfiguration(), true); });
test('rejects equal Preview and production trust material', () => {
  valid();
  process.env.PREVIEW_SESSION_KEY = process.env.WORKOS_COOKIE_PASSWORD;
  assert.throws(assertPreviewAuthConfiguration, AuthConfigurationError);
});
test('rejects duplicate gateway and production Redis ACL credentials', () => {
  valid();
  process.env.UPSTASH_REDIS_REST_TOKEN = process.env.GATEWAY_STATE_REDIS_TOKEN;
  assert.throws(assertPreviewAuthConfiguration, AuthConfigurationError);
});
test('fails closed when the production database binding is absent', () => {
  valid();
  delete process.env.DATABASE_URL;
  assert.throws(assertPreviewAuthConfiguration, AuthConfigurationError);
});

test('accepts only the canonical production web gateway', () => {
  process.env.PRODUCTION_ORIGIN = 'https://www.nvm.fyi';
  assert.equal(isProductionGatewayOrigin('https://nvm.fyi'), false);
  assert.equal(isProductionGatewayOrigin('https://www.nvm.fyi'), true);
  assert.equal(isProductionGatewayOrigin('https://evil.example'), false);
});

test.after(() => {
  for (const key of keys) {
    if (original[key] === undefined) delete process.env[key];
    else process.env[key] = original[key];
  }
});
