import assert from 'node:assert/strict';
import { test } from 'node:test';
import { isCreditDeltaAllowedUntilPlan006 } from './credits';

test('admin credit adjustments reject every negative delta before Plan 006', () => {
  assert.equal(isCreditDeltaAllowedUntilPlan006(-1), false);
  assert.equal(isCreditDeltaAllowedUntilPlan006(-100), false);
  assert.equal(isCreditDeltaAllowedUntilPlan006(0), true);
  assert.equal(isCreditDeltaAllowedUntilPlan006(10), true);
});
