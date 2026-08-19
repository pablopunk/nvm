import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import {
  parseAuthDeepLink,
  buildAuthDeepLinkUrl,
  isPatLike,
} from './deep-link';

const ACTIVE_BASE = 'https://api.nvm.fyi';

describe('isPatLike', () => {
  it('rejects nvm_ prefix', () => {
    assert.equal(isPatLike('nvm_pat_abc123'), true);
  });

  it('rejects JWT-like strings', () => {
    assert.equal(isPatLike('eyJhbGciOiJIUzI1NiJ9.abc.def'), true);
  });

  it('accepts short device codes', () => {
    assert.equal(isPatLike('abc123def456'), false);
  });
});

describe('parseAuthDeepLink', () => {
  it('parses a valid deep link with code and base_url', () => {
    const result = parseAuthDeepLink(
      `nvm://auth?code=abc123&base_url=${encodeURIComponent(ACTIVE_BASE)}`,
      ACTIVE_BASE,
    );
    assert.ok(result);
    assert.equal(result.code, 'abc123');
    assert.equal(result.baseUrl, ACTIVE_BASE);
    assert.equal(result.intent, 'connect');
  });

  it('defaults mode to connect for unknown modes', () => {
    const result = parseAuthDeepLink(
      `nvm://auth?code=abc123&base_url=${encodeURIComponent(ACTIVE_BASE)}&mode=unknown`,
      ACTIVE_BASE,
    );
    assert.ok(result);
    assert.equal(result.intent, 'connect');
  });

  it('parses reconnect mode', () => {
    const result = parseAuthDeepLink(
      `nvm://auth?code=abc123&base_url=${encodeURIComponent(ACTIVE_BASE)}&mode=reconnect`,
      ACTIVE_BASE,
    );
    assert.ok(result);
    assert.equal(result.intent, 'reconnect');
  });

  it('returns null for missing code', () => {
    const result = parseAuthDeepLink(
      `nvm://auth?base_url=${encodeURIComponent(ACTIVE_BASE)}`,
      ACTIVE_BASE,
    );
    assert.equal(result, null);
  });

  it('returns null for non-auth deep link', () => {
    const result = parseAuthDeepLink('nvm://other?code=abc123', ACTIVE_BASE);
    assert.equal(result, null);
  });

  it('returns null for non-nvm protocol', () => {
    const result = parseAuthDeepLink(
      'https://nvm.fyi/auth?code=abc123',
      ACTIVE_BASE,
    );
    assert.equal(result, null);
  });

  it('rejects PAT-like codes (nvm_ prefix)', () => {
    const result = parseAuthDeepLink(
      `nvm://auth?code=nvm_pat_abc123`,
      ACTIVE_BASE,
    );
    assert.equal(result, null);
  });

  it('rejects JWT-like codes', () => {
    const result = parseAuthDeepLink(
      `nvm://auth?code=eyJhbGciOiJIUzI1NiJ9.abc.def`,
      ACTIVE_BASE,
    );
    assert.equal(result, null);
  });

  it('falls back to activeBaseUrl for untrusted base_url', () => {
    const result = parseAuthDeepLink(
      `nvm://auth?code=abc123&base_url=${encodeURIComponent('https://evil.com')}`,
      ACTIVE_BASE,
    );
    assert.ok(result);
    assert.equal(result.baseUrl, ACTIVE_BASE);
  });

  it('accepts localhost base_url', () => {
    const result = parseAuthDeepLink(
      `nvm://auth?code=abc123&base_url=${encodeURIComponent('http://localhost:4321')}`,
      'http://localhost:4321',
    );
    assert.ok(result);
    assert.equal(result.baseUrl, 'http://localhost:4321');
  });

  it('returns null for empty URL', () => {
    const result = parseAuthDeepLink('', ACTIVE_BASE);
    assert.equal(result, null);
  });

  it('uses activeBaseUrl when base_url param is missing', () => {
    const result = parseAuthDeepLink('nvm://auth?code=abc123', ACTIVE_BASE);
    assert.ok(result);
    assert.equal(result.baseUrl, ACTIVE_BASE);
  });

  it('returns null for unusable URL', () => {
    const result = parseAuthDeepLink('not-a-url', ACTIVE_BASE);
    assert.equal(result, null);
  });
});

describe('buildAuthDeepLinkUrl', () => {
  it('builds a connect URL', () => {
    const url = buildAuthDeepLinkUrl('abc123', ACTIVE_BASE, 'connect');
    assert.ok(url.startsWith('nvm://auth?code=abc123'));
    assert.ok(url.includes('base_url='));
    assert.ok(!url.includes('mode=reconnect'));
  });

  it('builds a reconnect URL with mode param', () => {
    const url = buildAuthDeepLinkUrl('abc123', ACTIVE_BASE, 'reconnect');
    assert.ok(url.includes('mode=reconnect'));
  });
});
