import assert from 'node:assert/strict';
import { test } from 'node:test';
import { safeRelativeRedirectPath } from './safe-redirect';

test('keeps relative auth redirect paths', () => {
  assert.equal(safeRelativeRedirectPath('/dashboard'), '/dashboard');
  assert.equal(safeRelativeRedirectPath('/auth/device?code=abc#section'), '/auth/device?code=abc#section');
});

test('falls back for external or malformed auth redirect paths', () => {
  assert.equal(safeRelativeRedirectPath('https://evil.example/path'), '/dashboard');
  assert.equal(safeRelativeRedirectPath('//evil.example/path'), '/dashboard');
  assert.equal(safeRelativeRedirectPath('/%2F%2Fevil.example/path'), '/dashboard');
  assert.equal(safeRelativeRedirectPath('/%5C%5Cevil.example/path'), '/dashboard');
  assert.equal(safeRelativeRedirectPath('javascript:alert(1)'), '/dashboard');
  assert.equal(safeRelativeRedirectPath(''), '/dashboard');
  assert.equal(safeRelativeRedirectPath(null), '/dashboard');
});
