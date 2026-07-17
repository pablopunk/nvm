import assert from 'node:assert/strict';
import test from 'node:test';
import {
  assertPreviewAuthConfiguration,
  assertProductionAuthConfiguration,
  AuthConfigurationError,
  isProductionGatewayOrigin,
  productionAuthConfigured,
} from './auth-config';

const keys = [
  'NODE_ENV',
  'VERCEL_ENV',
  'PRODUCTION_ORIGIN',
  'DATABASE_URL',
  'PREVIEW_GATEWAY_ORIGIN',
  'PREVIEW_START_KEY',
  'GATEWAY_STATE_KEY',
  'GATEWAY_STATE_REDIS_URL',
  'GATEWAY_STATE_REDIS_TOKEN',
  'UPSTASH_REDIS_REST_URL',
  'UPSTASH_REDIS_REST_TOKEN',
  'PREVIEW_SESSION_KEY',
  'WORKOS_API_KEY',
  'WORKOS_COOKIE_PASSWORD',
  'WORKOS_CLIENT_ID',
  'WORKOS_REDIRECT_URI',
];
const original = Object.fromEntries(keys.map((key) => [key, process.env[key]]));

function validProduction() {
  Object.assign(process.env, {
    VERCEL_ENV: 'production',
    PRODUCTION_ORIGIN: 'https://nvm.fyi',
    WORKOS_API_KEY: 'sk_test_production',
    WORKOS_CLIENT_ID: 'client_test_production',
    WORKOS_REDIRECT_URI: 'https://nvm.fyi/api/auth/callback',
    WORKOS_COOKIE_PASSWORD: 'production-session-key-with-32-characters',
  });
}

function validPreview() {
  validProduction();
  Object.assign(process.env, {
    VERCEL_ENV: 'preview',
    DATABASE_URL: 'postgres://production',
    PREVIEW_GATEWAY_ORIGIN: 'https://nvm.fyi',
    PREVIEW_START_KEY: 'preview-start',
    GATEWAY_STATE_KEY: 'gateway-state',
    GATEWAY_STATE_REDIS_URL: 'https://gateway',
    GATEWAY_STATE_REDIS_TOKEN: 'gateway-acl',
    UPSTASH_REDIS_REST_URL: 'https://production-redis',
    UPSTASH_REDIS_REST_TOKEN: 'production-acl',
    PREVIEW_SESSION_KEY: 'preview-session',
  });
}

test('accepts complete production WorkOS configuration', function acceptsProductionConfiguration() {
  validProduction();
  assert.equal(assertProductionAuthConfiguration(), true);
  assert.equal(productionAuthConfigured(), true);
});

test('fails closed when production WorkOS values are missing', function rejectsMissingProductionValues() {
  for (const key of [
    'WORKOS_API_KEY',
    'WORKOS_CLIENT_ID',
    'WORKOS_REDIRECT_URI',
    'WORKOS_COOKIE_PASSWORD',
  ]) {
    validProduction();
    delete process.env[key];
    assert.throws(assertProductionAuthConfiguration, AuthConfigurationError);
    assert.equal(productionAuthConfigured(), false);
  }
});

test('rejects weak cookie material and unsafe callback URLs', function rejectsUnsafeProductionValues() {
  validProduction();
  process.env.WORKOS_COOKIE_PASSWORD = 'too-short';
  assert.throws(assertProductionAuthConfiguration, AuthConfigurationError);

  for (const redirectUri of [
    'not-a-url',
    'http://nvm.fyi/api/auth/callback',
    'https://evil.example/api/auth/callback',
    'https://nvm.fyi/api/auth/callback?code=leak',
    'https://nvm.fyi/wrong-path',
  ]) {
    validProduction();
    process.env.WORKOS_REDIRECT_URI = redirectUri;
    assert.throws(assertProductionAuthConfiguration, AuthConfigurationError);
  }

  validProduction();
  process.env.PRODUCTION_ORIGIN = 'not-an-origin';
  assert.throws(assertProductionAuthConfiguration, AuthConfigurationError);

  validProduction();
  process.env.WORKOS_REDIRECT_URI =
    'https://nvm.fyi/api/auth/callback ';
  assert.throws(assertProductionAuthConfiguration, AuthConfigurationError);
});

test('allows the documented localhost callback during local development', function acceptsLocalCallback() {
  validProduction();
  process.env.NODE_ENV = 'development';
  delete process.env.VERCEL_ENV;
  process.env.WORKOS_REDIRECT_URI = 'http://localhost:4321/api/auth/callback';
  assert.equal(assertProductionAuthConfiguration(), true);
});

test('rejects a localhost callback outside development', function rejectsProductionLocalCallback() {
  validProduction();
  delete process.env.VERCEL_ENV;
  process.env.NODE_ENV = 'production';
  process.env.WORKOS_REDIRECT_URI = 'http://localhost:4321/api/auth/callback';
  assert.throws(assertProductionAuthConfiguration, AuthConfigurationError);
});

test('accepts production-faithful Preview auth configuration', function acceptsPreviewConfiguration() {
  validPreview();
  assert.equal(assertPreviewAuthConfiguration(), true);
});

test('rejects equal Preview and production trust material', function rejectsSharedSessionKeys() {
  validPreview();
  process.env.PREVIEW_SESSION_KEY = process.env.WORKOS_COOKIE_PASSWORD;
  assert.throws(assertPreviewAuthConfiguration, AuthConfigurationError);
});

test('rejects duplicate gateway and production Redis ACL credentials', function rejectsSharedRedisCredentials() {
  validPreview();
  process.env.UPSTASH_REDIS_REST_TOKEN =
    process.env.GATEWAY_STATE_REDIS_TOKEN;
  assert.throws(assertPreviewAuthConfiguration, AuthConfigurationError);
});

test('fails closed when the production database binding is absent', function rejectsMissingDatabase() {
  validPreview();
  delete process.env.DATABASE_URL;
  assert.throws(assertPreviewAuthConfiguration, AuthConfigurationError);
});

test('accepts the Vercel www alias for the production preview gateway', function acceptsProductionAlias() {
  process.env.PRODUCTION_ORIGIN = 'https://nvm.fyi';
  assert.equal(isProductionGatewayOrigin('https://nvm.fyi'), true);
  assert.equal(isProductionGatewayOrigin('https://www.nvm.fyi'), true);
  assert.equal(isProductionGatewayOrigin('https://evil.example'), false);
});

test.after(function restoreEnvironment() {
  for (const key of keys) {
    if (original[key] === undefined) delete process.env[key];
    else process.env[key] = original[key];
  }
});
