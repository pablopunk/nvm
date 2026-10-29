'use strict';
const assert = require('node:assert/strict');
const { test } = require('node:test');
const { assertTagCanBeLatest } = require('./release-tag-guard.cjs');

test('a tag that sorts below the latest stable cannot publish', () => {
  assert.throws(
    () => assertTagCanBeLatest('v0.26.0', 'v2026.1029.5'),
    /older than latest stable/,
  );
});

test('the same tag or a newer tag may publish, including after a sibling job published it', () => {
  assertTagCanBeLatest('v2026.1029.5', 'v2026.1029.5');
  assertTagCanBeLatest('v2026.1030.6', 'v2026.1029.5');
  assertTagCanBeLatest('v0.26.0', 'v0.25.0');
});

test('unsupported tags and an unknown latest release fail closed', () => {
  assert.throws(
    () => assertTagCanBeLatest('v1.0.0-dev', 'v0.25.0'),
    /Unsupported/,
  );
  assert.throws(() => assertTagCanBeLatest('v1.0.0', undefined), /unavailable/);
});
