import assert from 'node:assert/strict';
import { afterEach, test } from 'node:test';
import { isAuthorizedCron } from './cron-auth';

afterEach(function resetCronAuthEnv() {
  delete process.env.CRON_SECRET;
  delete process.env.VERCEL_ENV;
  delete process.env.NODE_ENV;
});

test('permits requests when CRON_SECRET is not set in dev', function permitsWithoutSecretInDev() {
  assert.equal(isAuthorizedCron(new Request('https://api.nvm.fyi/api/cron/health-check', {
    headers: { authorization: 'Bearer anything' },
  })), true);
  assert.equal(isAuthorizedCron(new Request('https://api.nvm.fyi/api/cron/health-check')), true);
});

test('denies requests when CRON_SECRET is not set in production (fail closed)', function deniesWithoutSecretInProd() {
  process.env.VERCEL_ENV = 'production';
  assert.equal(isAuthorizedCron(new Request('https://api.nvm.fyi/api/cron/health-check', {
    headers: { authorization: 'Bearer anything' },
  })), false);
  assert.equal(isAuthorizedCron(new Request('https://api.nvm.fyi/api/cron/health-check')), false);
});

test('denies requests when CRON_SECRET is not set and NODE_ENV is production', function deniesWithoutSecretNodeEnvProd() {
  process.env.NODE_ENV = 'production';
  assert.equal(isAuthorizedCron(new Request('https://api.nvm.fyi/api/cron/health-check', {
    headers: { authorization: 'Bearer anything' },
  })), false);
});

test('permits in non-production VERCEL_ENV', function permitsInPreview() {
  process.env.VERCEL_ENV = 'preview';
  assert.equal(isAuthorizedCron(new Request('https://api.nvm.fyi/api/cron/health-check')), true);
});

test('validates bearer token when CRON_SECRET is set', function validatesToken() {
  process.env.CRON_SECRET = 's3cret';
  assert.equal(isAuthorizedCron(new Request('https://api.nvm.fyi/api/cron/health-check', {
    headers: { authorization: 'Bearer s3cret' },
  })), true);
  assert.equal(isAuthorizedCron(new Request('https://api.nvm.fyi/api/cron/health-check', {
    headers: { authorization: 'Bearer wrong' },
  })), false);
  assert.equal(isAuthorizedCron(new Request('https://api.nvm.fyi/api/cron/health-check')), false);
});

test('validates bearer token when CRON_SECRET is set even in production', function validatesTokenInProd() {
  process.env.CRON_SECRET = 's3cret';
  process.env.VERCEL_ENV = 'production';
  assert.equal(isAuthorizedCron(new Request('https://api.nvm.fyi/api/cron/health-check', {
    headers: { authorization: 'Bearer s3cret' },
  })), true);
  assert.equal(isAuthorizedCron(new Request('https://api.nvm.fyi/api/cron/health-check', {
    headers: { authorization: 'Bearer wrong' },
  })), false);
});

test('NODE_ENV development is not production', function devIsNotProduction() {
  process.env.NODE_ENV = 'development';
  assert.equal(isAuthorizedCron(new Request('https://api.nvm.fyi/api/cron/health-check')), true);
});
