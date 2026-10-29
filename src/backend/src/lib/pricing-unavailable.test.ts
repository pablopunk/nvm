import { readFileSync } from 'node:fs';
import assert from 'node:assert/strict';
import { test } from 'node:test';
import { pricingUnavailableBody } from './pricing-unavailable';

test('pricing errors keep the released type and add a reason for newer clients', () => {
  for (const reason of ['unpriced', 'stale', 'unverified'] as const) {
    const body = pricingUnavailableBody(reason);
    assert.equal(body.error.type, 'pricing_unavailable');
    assert.equal(body.error.reason, reason);
    assert.equal(typeof body.error.message, 'string');
    assert.ok(body.error.message.length > 0);
  }
});

test('the unpriced pricing error matches the desktop contract fixture', () => {
  const fixture = JSON.parse(
    readFileSync(
      new URL(
        '../fixtures/contracts/desktop-v1/pricing-unavailable-error.json',
        import.meta.url,
      ),
      'utf8',
    ),
  );
  assert.deepEqual(pricingUnavailableBody('unpriced'), fixture);
});
