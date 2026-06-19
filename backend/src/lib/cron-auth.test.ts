import assert from 'node:assert/strict';
import { afterEach, test } from 'node:test';
import { isAuthorizedCron } from './cron-auth';

function setEnv(name: string, value: string | undefined) {
  if (value === undefined) {
    delete process.env[name];
  } else {
    process.env[name] = value;
  }
}

afterEach(() => {
  delete process.env.CRON_SECRET;
  delete process.env.VERCEL_ENV;
  delete process.env.NODE_ENV;
});

test('permits requests when CRON_SECRET is not set in dev', () => {
  assert.equal(isAuthorizedCron(new Request('https://api.nvm.fyi/api/cron/health-check', {
    headers: { authorization: 'Bearer anything' },
  })), true);
  assert.equal(isAuthorizedCron(new Request('https://api.nvm.fyi/api/cron/health-check')), true);
});

test('denies requests when CRON_SECRET is not set in production (fail closed)', () => {
  process.env.VERCEL_ENV = 'production';
  assert.equal(isAuthorizedCron(new Request('https://api.nvm.fyi/api/cron/health-check', {
    headers: { authorization: 'Bearer anything' },
  })), false);
  assert.equal(isAuthorizedCron(new Request('https://api.nvm.fyi/api/cron/health-check')), false);
});

test('denies requests when CRON_SECRET is not set and NODE_ENV is production', () => {
  process.env.NODE_ENV = 'production';
  assert.equal(isAuthorizedCron(new Request('https://api.nvm.fyi/api/cron/health-check', {
    headers: { authorization: 'Bearer anything' },
  })), false);
});

test('permits in non-production VERCEL_ENV', () => {
  process.env.VERCEL_ENV = 'preview';
  assert.equal(isAuthorizedCron(new Request('https://api.nvm.fyi/api/cron/health-check')), true);
});

test('validates bearer token when CRON_SECRET is set', () => {
  process.env.CRON_SECRET = 's3cret';
  assert.equal(isAuthorizedCron(new Request('https://api.nvm.fyi/api/cron/health-check', {
    headers: { authorization: 'Bearer s3cret' },
  })), true);
  assert.equal(isAuthorizedCron(new Request('https://api.nvm.fyi/api/cron/health-check', {
    headers: { authorization: 'Bearer wrong' },
  })), false);
  assert.equal(isAuthorizedCron(new Request('https://api.nvm.fyi/api/cron/health-check')), false);
});

test('validates bearer token when CRON_SECRET is set even in production', () => {
  process.env.CRON_SECRET = 's3cret';
  process.env.VERCEL_ENV = 'production';
  assert.equal(isAuthorizedCron(new Request('https://api.nvm.fyi/api/cron/health-check', {
    headers: { authorization: 'Bearer s3cret' },
  })), true);
  assert.equal(isAuthorizedCron(new Request('https://api.nvm.fyi/api/cron/health-check', {
    headers: { authorization: 'Bearer wrong' },
  })), false);
});

test('NODE_ENV development is not production', () => {
  process.env.NODE_ENV = 'development';
  assert.equal(isAuthorizedCron(new Request('https://api.nvm.fyi/api/cron/health-check')), true);
});
