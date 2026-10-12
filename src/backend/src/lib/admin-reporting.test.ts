import assert from 'node:assert/strict';
import { test } from 'node:test';
import {
  DEFAULT_ADMIN_PAGE_SIZE,
  decodeAdminPageCursor,
  encodeAdminPageCursor,
  MAX_ADMIN_PAGE_SIZE,
  parseAdminPageSize,
  reportBounds,
  resolveAdminReportRange,
  usageCursorIdIsValid,
} from './admin-reporting';

test('admin UTC report ranges default to 30 days and enforce bounds', () => {
  const now = new Date('2026-10-07T12:00:00.000Z');
  const range = resolveAdminReportRange(null, null, now);
  assert.deepEqual(range, {
    from: new Date('2026-09-07T12:00:00.000Z'),
    to: now,
  });
  assert.equal(resolveAdminReportRange('2025-10-07', '2026-10-08', now), null);
  assert.equal(resolveAdminReportRange('2026-10-08', '2026-10-07', now), null);
  assert.equal(resolveAdminReportRange('2026-02-30', '2026-03-01', now), null);
});

test('admin request cursors normalize PostgreSQL timestamps and bind to fixed bounds', () => {
  const range = resolveAdminReportRange(
    '2026-10-01',
    '2026-10-07T12:00:00Z',
    new Date('2026-10-07T12:00:00Z'),
  );
  assert.ok(range);
  const bounds = reportBounds(range);
  const encoded = encodeAdminPageCursor({
    createdAt: '2026-10-06T23:59:59.123456Z',
    id: '9223372036854775807',
    bounds,
  });
  const decoded = decodeAdminPageCursor(encoded, bounds, usageCursorIdIsValid);
  assert.equal(decoded?.createdAt, '2026-10-06T23:59:59.123456Z');
  assert.equal(decoded?.id, '9223372036854775807');
  assert.equal(
    decodeAdminPageCursor(
      encoded,
      { ...bounds, to: '2026-10-08T00:00:00.000Z' },
      usageCursorIdIsValid,
    ),
    undefined,
  );
  assert.equal(usageCursorIdIsValid('9223372036854775808'), false);
});

test('admin cursor preserves microseconds returned by PostgreSQL and pads JavaScript dates safely', () => {
  const bounds = {
    from: '2026-10-01T00:00:00.000Z',
    to: '2026-10-08T00:00:00.000Z',
  };
  const databaseTimestamp = encodeAdminPageCursor({
    createdAt: '2026-10-06T23:59:59.123456Z',
    id: '100',
    bounds,
  });
  const dateTimestamp = encodeAdminPageCursor({
    createdAt: new Date('2026-10-06T23:59:59.123Z'),
    id: '101',
    bounds,
  });

  assert.equal(
    decodeAdminPageCursor(databaseTimestamp, bounds, usageCursorIdIsValid)
      ?.createdAt,
    '2026-10-06T23:59:59.123456Z',
  );
  assert.equal(
    decodeAdminPageCursor(dateTimestamp, bounds, usageCursorIdIsValid)
      ?.createdAt,
    '2026-10-06T23:59:59.123000Z',
  );
});

test('admin page sizes are bounded', () => {
  assert.equal(parseAdminPageSize(null), DEFAULT_ADMIN_PAGE_SIZE);
  assert.equal(
    parseAdminPageSize(String(MAX_ADMIN_PAGE_SIZE)),
    MAX_ADMIN_PAGE_SIZE,
  );
  assert.equal(parseAdminPageSize('101'), null);
  assert.equal(parseAdminPageSize('-1'), null);
});
