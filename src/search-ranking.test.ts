import assert from 'node:assert/strict';
import test from 'node:test';
import { normalize, scoreNormalized } from './electron/search-utils';
import { scoreText } from './filtering';
import { scoreFuzzy, scoreNormalizedNonEmpty } from './search-ranking';

// ── Core bands ───────────────────────────────────────────────────────
test('scoreNormalizedNonEmpty exact match', () => {
  assert.equal(scoreNormalizedNonEmpty('hello', 'hello'), 100);
  assert.equal(scoreNormalizedNonEmpty('a', 'a'), 100);
});

test('scoreNormalizedNonEmpty startsWith', () => {
  assert.equal(scoreNormalizedNonEmpty('hello-world', 'hello'), 90);
  assert.equal(scoreNormalizedNonEmpty('abc', 'a'), 80);
  assert.equal(scoreNormalizedNonEmpty('nevermind', 'never'), 80);
});

test('scoreNormalizedNonEmpty whole-word matches before prefix and substring', () => {
  assert.equal(scoreNormalizedNonEmpty('open terminal', 'terminal'), 90);
  assert.equal(scoreNormalizedNonEmpty('open-terminal', 'terminal'), 90);
  assert.equal(scoreNormalizedNonEmpty('terminal settings', 'terminal'), 90);
  assert.equal(scoreNormalizedNonEmpty('party', 'art'), 50);
  assert.equal(scoreNormalizedNonEmpty('terminally', 'terminal'), 80);
});

test('scoreNormalizedNonEmpty includes', () => {
  assert.equal(scoreNormalizedNonEmpty('hello world', 'world'), 90);
  assert.equal(scoreNormalizedNonEmpty('open settings', 'settings'), 90);
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
  assert.equal(scoreText('hello world', 'hello'), 90);
});

test('scoreText includes', () => {
  assert.equal(scoreText('hello world', 'world'), 90);
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

// ── scoreFuzzy (with character-set fallback) ─────────────────────────
test('scoreFuzzy delegates to sequential bands', () => {
  assert.equal(scoreFuzzy('hello', 'hello'), 100);
  assert.equal(scoreFuzzy('hello world', 'hello'), 90);
  assert.equal(scoreFuzzy('hello world', 'world'), 90);
  assert.equal(scoreFuzzy('settings', 'stng'), 20);
});

test('scoreFuzzy char-set fallback — chars in wrong order', () => {
  // "gmial" vs "gmail": sequential fails (i before a), but all chars exist
  assert.equal(scoreFuzzy('gmail', 'gmial'), 10);
  // "tpying" vs "typing"
  assert.equal(scoreFuzzy('typing', 'tpying'), 10);
});

test('scoreFuzzy char-set fallback — missing character', () => {
  assert.equal(scoreFuzzy('hello', 'xyz'), 0);
  assert.equal(scoreFuzzy('abc', 'abcd'), 0);
  assert.equal(scoreFuzzy('hello', 'helloo'), 0); // extra 'o' not in text
});

test('scoreFuzzy char-set fallback — multiplicity respected', () => {
  // Query has two 'l's, text has one
  assert.equal(scoreFuzzy('hello', 'helll'), 0);
  // Query has two 'o's, text has one
  assert.equal(scoreFuzzy('hello', 'heloo'), 0);
});

// ── scoreNormalized wrapper (now uses scoreFuzzy) ────────────────────
test('scoreNormalized empty query returns 0', () => {
  assert.equal(scoreNormalized('anything', ''), 0);
  assert.equal(scoreNormalized('', ''), 0);
  assert.equal(scoreNormalized(undefined, ''), 0);
});

test('scoreNormalized exact match', () => {
  assert.equal(scoreNormalized('hello', normalize('hello')), 100);
});

test('scoreNormalized startsWith', () => {
  assert.equal(scoreNormalized('hello world', normalize('hello')), 90);
});

test('scoreNormalized includes', () => {
  assert.equal(scoreNormalized('hello world', normalize('world')), 90);
});

test('scoreNormalized fuzzy', () => {
  assert.equal(scoreNormalized('settings', normalize('stng')), 20);
});

test('scoreNormalized char-set fallback via scoreFuzzy', () => {
  assert.equal(scoreNormalized('gmail', normalize('gmial')), 10);
});

test('scoreNormalized no match', () => {
  assert.equal(scoreNormalized('hello', normalize('xyz')), 0);
});

test('scoreNormalized case insensitive and trim via normalize', () => {
  assert.equal(scoreNormalized('  Hello  ', normalize('hello')), 100);
  assert.equal(scoreNormalized(123, normalize('123')), 100);
});
