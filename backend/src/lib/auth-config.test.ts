import assert from 'node:assert/strict';
import test from 'node:test';
import { assertPreviewAuthConfiguration, assertPreviewDatabaseBinding, AuthConfigurationError } from './auth-config';

const keys = ['VERCEL_ENV', 'PREVIEW_GATEWAY_ORIGIN', 'PREVIEW_START_KEY', 'GATEWAY_STATE_KEY', 'GATEWAY_STATE_REDIS_URL', 'GATEWAY_STATE_REDIS_TOKEN', 'PREVIEW_GRANT_REDIS_URL', 'PREVIEW_GRANT_REDIS_TOKEN', 'PREVIEW_REDIS_URL', 'PREVIEW_REDIS_TOKEN', 'PREVIEW_SESSION_KEY', 'PREVIEW_DATABASE_URL', 'PRODUCTION_DATABASE_URL', 'DATABASE_URL', 'WORKOS_COOKIE_PASSWORD', 'WORKOS_PREVIEW_CLIENT_ID', 'WORKOS_CLIENT_ID'];
const original = Object.fromEntries(keys.map((key) => [key, process.env[key]]));

function valid() {
  Object.assign(process.env, {
    PREVIEW_GATEWAY_ORIGIN: 'https://nvm.fyi', PREVIEW_START_KEY: 'preview-start', GATEWAY_STATE_KEY: 'gateway-state', GATEWAY_STATE_REDIS_URL: 'https://gateway', GATEWAY_STATE_REDIS_TOKEN: 'gateway-acl',
    PREVIEW_GRANT_REDIS_URL: 'https://preview-grant', PREVIEW_GRANT_REDIS_TOKEN: 'grant-acl', PREVIEW_REDIS_URL: 'https://preview-runtime', PREVIEW_REDIS_TOKEN: 'runtime-acl',
    PREVIEW_SESSION_KEY: 'preview-session', DATABASE_URL: 'postgres://preview', PREVIEW_DATABASE_URL: 'postgres://preview', PRODUCTION_DATABASE_URL: 'postgres://production',
    WORKOS_COOKIE_PASSWORD: 'production-session', WORKOS_CLIENT_ID: 'prod-client', VERCEL_ENV: 'preview',
  });
}

test('accepts isolated Preview auth configuration', () => { valid(); assert.equal(assertPreviewAuthConfiguration(), true); });
test('binds Preview runtime database to the validated Preview URL', () => { valid(); assert.equal(assertPreviewDatabaseBinding(), true); });
test('rejects equal Preview and production trust material', () => {
  valid();
  process.env.PREVIEW_SESSION_KEY = process.env.WORKOS_COOKIE_PASSWORD;
  assert.throws(assertPreviewAuthConfiguration, AuthConfigurationError);
});
test('rejects a runtime database that is not the Preview database', () => {
  valid();
  process.env.DATABASE_URL = 'postgres://other-preview';
  assert.throws(assertPreviewDatabaseBinding, AuthConfigurationError);
  process.env.DATABASE_URL = process.env.PREVIEW_DATABASE_URL;
  process.env.PREVIEW_DATABASE_URL = process.env.PRODUCTION_DATABASE_URL;
  assert.throws(assertPreviewDatabaseBinding, AuthConfigurationError);
});
test('rejects any duplicate Redis ACL credential', () => {
  valid();
  process.env.PREVIEW_REDIS_TOKEN = process.env.GATEWAY_STATE_REDIS_TOKEN;
  assert.throws(assertPreviewAuthConfiguration, AuthConfigurationError);
  valid();
  process.env.PREVIEW_GRANT_REDIS_TOKEN = process.env.GATEWAY_STATE_REDIS_TOKEN;
  assert.throws(assertPreviewAuthConfiguration, AuthConfigurationError);
});

test.after(() => {
  for (const key of keys) {
    if (original[key] === undefined) delete process.env[key];
    else process.env[key] = original[key];
  }
});
