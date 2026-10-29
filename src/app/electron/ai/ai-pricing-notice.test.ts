import assert from 'node:assert/strict';
import { test } from 'node:test';
import { pricingUnavailableNoticeFromText } from './ai-pricing-notice';

test('a pricing_unavailable error body becomes a meaningful AI model notice', () => {
  const notice = pricingUnavailableNoticeFromText(
    '{"error":{"type":"pricing_unavailable","reason":"stale"}}',
  );
  assert.equal(notice?.kind, 'pricing_unavailable');
  assert.equal(notice?.title, 'AI model unavailable');
  assert.match(notice?.message ?? '', /choose another model/);
});

test('unrelated errors do not become pricing notices', () => {
  assert.equal(pricingUnavailableNoticeFromText('upstream_unavailable'), null);
  assert.equal(pricingUnavailableNoticeFromText('insufficient_credits'), null);
});
