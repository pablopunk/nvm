import assert from 'node:assert/strict';
import test from 'node:test';
import {
  joinPublicApiUrl,
  migrateLegacyDesktopOrigin,
  parsePublicOrigin,
} from './public-origin';

test('normalizes and joins canonical origins', () => {
  assert.equal(
    parsePublicOrigin(' HTTPS://API.NVM.FYI:443/ ', 'production_api'),
    'https://api.nvm.fyi',
  );
  assert.equal(
    parsePublicOrigin('http://LOCALHOST:4321/', 'local'),
    'http://localhost:4321',
  );
  assert.equal(
    joinPublicApiUrl('https://api.nvm.fyi', 'api/v1/active-model'),
    'https://api.nvm.fyi/api/v1/active-model',
  );
});

test('rejects unsafe or path-bearing origins', () => {
  for (const value of [
    'ftp://api.nvm.fyi',
    'https://user:pass@api.nvm.fyi',
    'https://api.nvm.fyi?x=1',
    'https://api.nvm.fyi#x',
    'https://api.nvm.fyi?',
    'https://api.nvm.fyi#',
    'https://api.nvm.fyi/api',
    'https://api.nvm.fyi/api/',
    'https://api.nvm.fyi/%2e%2e',
    'https://api.nvm.fyi//',
    'http://api.nvm.fyi',
  ])
    assert.throws(() => parsePublicOrigin(value, 'production_api'));
  assert.throws(() =>
    parsePublicOrigin('https://api.nvm.fyi:8443', 'production_api'),
  );
  assert.throws(() => parsePublicOrigin('http://example.com', 'local'));
});

test('enforces policy hosts and exact Preview deployments', () => {
  assert.throws(() => parsePublicOrigin('https://nvm.fyi', 'production_web'));
  const preview =
    'https://nvm-feature-pablo-varelas-projects-4f86af8b.vercel.app';
  assert.equal(parsePublicOrigin(preview, 'preview', preview), preview);
  assert.throws(() =>
    parsePublicOrigin('https://other.example.vercel.app', 'preview', preview),
  );
});

test('migrates only known legacy desktop production values', () => {
  assert.equal(
    migrateLegacyDesktopOrigin('https://nvm.fyi/api/'),
    'https://api.nvm.fyi',
  );
  assert.equal(
    migrateLegacyDesktopOrigin('https://www.nvm.fyi'),
    'https://api.nvm.fyi',
  );
  assert.equal(migrateLegacyDesktopOrigin('https://evil.example/api'), null);
});
