import assert from 'node:assert/strict';
import test from 'node:test';
import { normalize, scoreNormalized } from './electron/search-utils';
import { scoreText } from './filtering';
import { scoreNormalizedNonEmpty } from './search-ranking';

// ── Core bands ───────────────────────────────────────────────────────
test('scoreNormalizedNonEmpty exact match', () => {
  assert.equal(scoreNormalizedNonEmpty('hello', 'hello'), 100);
  assert.equal(scoreNormalizedNonEmpty('a', 'a'), 100);
});

test('scoreNormalizedNonEmpty startsWith', () => {
  assert.equal(scoreNormalizedNonEmpty('hello world', 'hello'), 80);
  assert.equal(scoreNormalizedNonEmpty('abc', 'a'), 80);
  assert.equal(scoreNormalizedNonEmpty('nevermind', 'never'), 80);
});

test('scoreNormalizedNonEmpty includes', () => {
  assert.equal(scoreNormalizedNonEmpty('hello world', 'world'), 50);
  assert.equal(scoreNormalizedNonEmpty('open settings', 'settings'), 50);
  assert.equal(scoreNormalizedNonEmpty('abc123', '123'), 50);
});

test('scoreNormalizedNonEmpty fuzzy match', () => {
  assert.equal(scoreNormalizedNonEmpty('hello', 'hlo'), 20);
  assert.equal(scoreNormalizedNonEmpty('typescript', 'ts'), 20);
  assert.equal(scoreNormalizedNonEmpty('settings', 'stng'), 20);
});

test('scoreNormalizedNonEmpty no match', () => {
  assert.equal(scoreNormalizedNonEmpty('hello', 'xyz'), 0);
  assert.equal(scoreNormalizedNonEmpty('abc', 'abx'), 0);
});

test('scoreNormalizedNonEmpty case sensitivity is caller responsibility', () => {
  // Core expects already-lowered input; passing mixed case may not match
  assert.equal(scoreNormalizedNonEmpty('Hello', 'hello'), 0);
  assert.equal(scoreNormalizedNonEmpty('HELLO', 'hello'), 0);
});

// ── scoreText wrapper ─────────────────────────────────────────────────
test('scoreText empty filter returns 1', () => {
  assert.equal(scoreText('anything', ''), 1);
  assert.equal(scoreText(undefined, ''), 1);
});

test('scoreText exact match', () => {
  assert.equal(scoreText('hello', 'hello'), 100);
});

test('scoreText startsWith', () => {
  assert.equal(scoreText('hello world', 'hello'), 80);
});

test('scoreText includes', () => {
  assert.equal(scoreText('hello world', 'world'), 50);
});

test('scoreText fuzzy', () => {
  assert.equal(scoreText('settings', 'stng'), 20);
});

test('scoreText no match', () => {
  assert.equal(scoreText('hello', 'xyz'), 0);
});

test('scoreText undefined value', () => {
  assert.equal(scoreText(undefined, 'query'), 0);
});

test('scoreText case insensitive via toLowerCase', () => {
  assert.equal(scoreText('Hello', 'hello'), 100);
  assert.equal(scoreText('HELLO', 'hello'), 100);
});

// ── scoreNormalized wrapper ───────────────────────────────────────────
test('scoreNormalized empty query returns 0', () => {
  assert.equal(scoreNormalized('anything', ''), 0);
  assert.equal(scoreNormalized('', ''), 0);
  assert.equal(scoreNormalized(undefined, ''), 0);
});

test('scoreNormalized exact match', () => {
  assert.equal(scoreNormalized('hello', normalize('hello')), 100);
});

test('scoreNormalized startsWith', () => {
  assert.equal(scoreNormalized('hello world', normalize('hello')), 80);
});

test('scoreNormalized includes', () => {
  assert.equal(scoreNormalized('hello world', normalize('world')), 50);
});

test('scoreNormalized fuzzy', () => {
  assert.equal(scoreNormalized('settings', normalize('stng')), 20);
});

test('scoreNormalized no match', () => {
  assert.equal(scoreNormalized('hello', normalize('xyz')), 0);
});

test('scoreNormalized case insensitive and trim via normalize', () => {
  assert.equal(scoreNormalized('  Hello  ', normalize('hello')), 100);
  assert.equal(scoreNormalized(123, normalize('123')), 100);
});
