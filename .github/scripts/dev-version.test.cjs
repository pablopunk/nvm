'use strict';
const assert = require('node:assert/strict');
const { test } = require('node:test');
const { versionFromHistory } = require('./dev-version.cjs');

test('orders first-parent commits even when timestamps go backwards', () => {
  const first = Date.parse('2026-10-29T12:00:00Z') / 1000;
  assert.equal(versionFromHistory([first]), '2026.1029.1');
  assert.equal(versionFromHistory([first - 86400, first]), '2026.1029.2');
  assert.equal(versionFromHistory([first + 86400, first]), '2026.1030.2');
});

test('rejects missing history and Windows component overflow', () => {
  assert.throws(() => versionFromHistory([]));
  assert.throws(() => versionFromHistory(Array(65536).fill(1)));
});
