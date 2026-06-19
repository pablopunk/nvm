import assert from 'node:assert/strict';
import { test } from 'node:test';
import { requireSameOrigin } from './csrf';

test('returns null when origin header is missing', () => {
  const req = new Request('https://nvm.fyi/api/auth/signout', {
    method: 'POST',
  });
  assert.equal(requireSameOrigin(req), null);
});

test('returns null when origin matches the request URL origin', () => {
  const req = new Request('https://nvm.fyi/api/auth/signout', {
    method: 'POST',
    headers: { origin: 'https://nvm.fyi' },
  });
  assert.equal(requireSameOrigin(req), null);
});

test('rejects cross-origin POST with 403', () => {
  const req = new Request('https://nvm.fyi/api/auth/signout', {
    method: 'POST',
    headers: { origin: 'https://evil.example.com' },
  });
  const res = requireSameOrigin(req);
  assert.notEqual(res, null);
  assert.equal(res!.status, 403);
});

test('normalizes origin with trailing slash via URL.origin', () => {
  const req = new Request('https://nvm.fyi/api/auth/signout', {
    method: 'POST',
    headers: { origin: 'https://nvm.fyi/' },
  });
  // URL.origin strips trailing slash on both sides
  const res = requireSameOrigin(req);
  assert.equal(res, null);
});

test('rejects malformed origin headers', () => {
  const req = new Request('https://nvm.fyi/api/auth/signout', {
    method: 'POST',
    headers: { origin: 'not-a-valid-url' },
  });
  const res = requireSameOrigin(req);
  assert.notEqual(res, null);
  assert.equal(res!.status, 403);
});

test('rejects different scheme even if host matches', () => {
  const req = new Request('https://nvm.fyi/api/auth/signout', {
    method: 'POST',
    headers: { origin: 'http://nvm.fyi' },
  });
  const res = requireSameOrigin(req);
  assert.notEqual(res, null);
  assert.equal(res!.status, 403);
});

test('rejects different port', () => {
  const req = new Request('https://nvm.fyi:443/api/auth/signout', {
    method: 'POST',
    headers: { origin: 'https://nvm.fyi:8080' },
  });
  const res = requireSameOrigin(req);
  assert.notEqual(res, null);
  assert.equal(res!.status, 403);
});
