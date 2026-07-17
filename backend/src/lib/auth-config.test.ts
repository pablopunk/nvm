import assert from 'node:assert/strict';
import test from 'node:test';
import {
  assertPreviewAuthConfiguration,
  assertProductionAuthConfiguration,
  AuthConfigurationError,
  isProductionGatewayOrigin,
  productionAuthConfigured,
  resolveAuthRedirectConfiguration,
} from './auth-config';

const keys = [
  'NODE_ENV',
  'VERCEL_ENV',
  'PRODUCTION_ORIGIN',
  'PUBLIC_DASHBOARD_URL',
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
    PRODUCTION_ORIGIN: 'https://www.nvm.fyi',
    PUBLIC_DASHBOARD_URL: 'https://www.nvm.fyi',
    WORKOS_API_KEY: 'sk_test_production',
    WORKOS_CLIENT_ID: 'client_test_production',
    WORKOS_REDIRECT_URI: 'https://www.nvm.fyi/api/auth/callback',
    WORKOS_COOKIE_PASSWORD: 'production-session-key-with-32-characters',
  });
}

function validPreview() {
  validProduction();
  Object.assign(process.env, {
    VERCEL_ENV: 'preview',
    DATABASE_URL: 'postgres://production',
    PREVIEW_GATEWAY_ORIGIN: 'https://www.nvm.fyi',
    PREVIEW_START_KEY: 'preview-start',
    GATEWAY_STATE_KEY: 'gateway-state',
    GATEWAY_STATE_REDIS_URL: 'https://gateway',
    GATEWAY_STATE_REDIS_TOKEN: 'gateway-acl',
    UPSTASH_REDIS_REST_URL: 'https://production-redis',
    UPSTASH_REDIS_REST_TOKEN: 'production-acl',
    PREVIEW_SESSION_KEY: 'preview-session',
  });
}

test('accepts complete canonical production WorkOS configuration', () => {
  validProduction();
  assert.equal(assertProductionAuthConfiguration(), true);
  assert.equal(productionAuthConfigured(), true);
  assert.deepEqual(resolveAuthRedirectConfiguration(), {
    productionOrigin: 'https://www.nvm.fyi',
    redirectUri: 'https://www.nvm.fyi/api/auth/callback',
  });
});

test('fails closed when production WorkOS values are missing', () => {
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

test('rejects weak cookie material and unsafe or stale callback URLs', () => {
  validProduction();
  process.env.WORKOS_COOKIE_PASSWORD = 'too-short';
  assert.throws(assertProductionAuthConfiguration, AuthConfigurationError);

  for (const redirectUri of [
    'not-a-url',
    'http://www.nvm.fyi/api/auth/callback',
    'https://evil.example/api/auth/callback',
    'https://nvm.fyi/api/auth/callback',
    'https://www.nvm.fyi/api/auth/callback?code=leak',
    'https://www.nvm.fyi/wrong-path',
    'https://user:pass@www.nvm.fyi/api/auth/callback',
  ]) {
    validProduction();
    process.env.WORKOS_REDIRECT_URI = redirectUri;
    assert.throws(assertProductionAuthConfiguration, AuthConfigurationError);
  }

  validProduction();
  process.env.PRODUCTION_ORIGIN = 'https://nvm.fyi';
  assert.throws(assertProductionAuthConfiguration, AuthConfigurationError);

  validProduction();
  process.env.WORKOS_REDIRECT_URI =
    'https://www.nvm.fyi/api/auth/callback ';
  assert.throws(assertProductionAuthConfiguration, AuthConfigurationError);
});

test('allows documented loopback callbacks only during local development', () => {
  validProduction();
  process.env.NODE_ENV = 'development';
  delete process.env.VERCEL_ENV;
  process.env.WORKOS_REDIRECT_URI = 'http://localhost:4321/api/auth/callback';
  assert.equal(assertProductionAuthConfiguration(), true);

  validProduction();
  process.env.WORKOS_REDIRECT_URI = 'http://127.0.0.1:4321/api/auth/callback';
  assert.throws(assertProductionAuthConfiguration, AuthConfigurationError);
});

test('rejects loopback callbacks in deployed runtimes', () => {
  validProduction();
  process.env.WORKOS_REDIRECT_URI = 'http://localhost:4321/api/auth/callback';
  assert.throws(assertProductionAuthConfiguration, AuthConfigurationError);
});

test('accepts production-faithful Preview auth configuration', () => {
  validPreview();
  assert.equal(assertPreviewAuthConfiguration(), true);
});

test('rejects equal Preview and production trust material', () => {
  validPreview();
  process.env.PREVIEW_SESSION_KEY = process.env.WORKOS_COOKIE_PASSWORD;
  assert.throws(assertPreviewAuthConfiguration, AuthConfigurationError);
});

test('rejects duplicate gateway and production Redis ACL credentials', () => {
  validPreview();
  process.env.UPSTASH_REDIS_REST_TOKEN =
    process.env.GATEWAY_STATE_REDIS_TOKEN;
  assert.throws(assertPreviewAuthConfiguration, AuthConfigurationError);
});

test('fails closed when the production database binding is absent', () => {
  validPreview();
  delete process.env.DATABASE_URL;
  assert.throws(assertPreviewAuthConfiguration, AuthConfigurationError);
});

test('accepts only the canonical production web gateway', () => {
  validProduction();
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
