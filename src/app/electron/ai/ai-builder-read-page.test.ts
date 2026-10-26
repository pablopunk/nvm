import assert from 'node:assert/strict';
import test from 'node:test';
import * as pi from '@earendil-works/pi-coding-agent';
import {
  boundedBuilderReadPage,
  formatBuilderValidationError,
  recordExtensionReadCoverage,
} from './ai-builder-read-page';

test('bounded source pages are deterministic and expose complete line continuation', () => {
  const source = Array.from(
    { length: 5 },
    (_value, index) => `line ${index + 1}`,
  ).join('\n');
  const first = boundedBuilderReadPage(pi, source, { limit: 2 });
  const second = boundedBuilderReadPage(pi, source, {
    offset: first.nextOffset ?? undefined,
    limit: 2,
  });
  const third = boundedBuilderReadPage(pi, source, {
    offset: second.nextOffset ?? undefined,
    limit: 2,
  });

  assert.match(first.text, /lines 1-2 of 5/);
  assert.equal(first.nextOffset, 2);
  assert.equal(first.revision, second.revision);
  assert.equal(second.nextOffset, 4);
  assert.equal(third.nextOffset, null);
  assert.equal(third.complete, true);
  assert.match(third.text, /line 5/);
  assert.ok(Buffer.byteLength(first.text, 'utf8') <= pi.DEFAULT_MAX_BYTES);
});

test('oversized UTF-8 lines have bounded byte continuation and count as read only when complete', () => {
  const source = `${'é'.repeat(pi.DEFAULT_MAX_BYTES)}\nnext line`;
  const coverage = new Map();
  const first = boundedBuilderReadPage(pi, source);
  assert.ok(first.byteRange);
  assert.ok(first.nextByteOffset);
  assert.equal(first.nextOffset, 0);
  assert.equal(first.complete, false);
  assert.ok(Buffer.byteLength(first.text, 'utf8') <= pi.DEFAULT_MAX_BYTES);
  assert.equal(recordExtensionReadCoverage(coverage, 'large.ts', first), false);

  const second = boundedBuilderReadPage(pi, source, {
    offset: first.nextOffset ?? undefined,
    byteOffset: first.nextByteOffset ?? undefined,
  });
  assert.equal(second.byteRange?.start, first.byteRange?.end);
  assert.equal(
    recordExtensionReadCoverage(coverage, 'large.ts', second),
    false,
  );

  const third = boundedBuilderReadPage(pi, source, {
    offset: second.nextOffset ?? undefined,
    byteOffset: second.nextByteOffset ?? undefined,
  });
  assert.equal(third.byteRange?.start, second.byteRange?.end);
  assert.equal(third.nextOffset, 1);
  assert.equal(third.nextByteOffset, null);
  assert.equal(recordExtensionReadCoverage(coverage, 'large.ts', third), false);

  const fourth = boundedBuilderReadPage(pi, source, {
    offset: third.nextOffset ?? undefined,
  });
  assert.match(fourth.text, /next line/);
  assert.equal(recordExtensionReadCoverage(coverage, 'large.ts', fourth), true);
});

test('validation errors retain filenames and actionable locations within the Pi output ceiling', () => {
  const error = new Error(
    `source.ts:17:9 - Type mismatch\n${'x'.repeat(pi.DEFAULT_MAX_BYTES * 2)}`,
  );
  const diagnostic = formatBuilderValidationError(pi, 'source.ts', error);

  assert.match(diagnostic, /^source\.ts:/);
  assert.match(diagnostic, /source\.ts:17:9 - Type mismatch/);
  assert.match(diagnostic, /Validation diagnostics truncated/);
  assert.ok(Buffer.byteLength(diagnostic, 'utf8') <= pi.DEFAULT_MAX_BYTES);
});
