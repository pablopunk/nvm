import assert from 'node:assert/strict';
import { test } from 'node:test';
import { freeSpendCapNoticeFromText } from './free-spend-cap-notice';

test('free-spend cap response becomes a dedicated desktop notice without implying a paid-only bypass', () => {
  const notice = freeSpendCapNoticeFromText(
    '{"error":{"type":"free_spend_cap_reached","message":"Free-spend capacity is currently unavailable.","resetsAt":"2026-11-01T00:00:00.000Z"}}',
  );

  assert.ok(notice);
  assert.equal(notice.kind, 'free_spend_cap_reached');
  assert.equal(notice.resetsAt, '2026-11-01T00:00:00.000Z');
  assert.match(notice.message, /may return earlier/);
  assert.doesNotMatch(notice.message, /paid credits/i);
});

test('unrelated backend errors do not become free-spend cap notices', () => {
  assert.equal(
    freeSpendCapNoticeFromText('{"error":{"type":"insufficient_credits"}}'),
    null,
  );
});
