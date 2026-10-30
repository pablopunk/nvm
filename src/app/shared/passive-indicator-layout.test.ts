import assert from 'node:assert/strict';
import test from 'node:test';
import {
  INDICATOR_MAX_SUMMARY_LINES,
  INDICATOR_WINDOW_HEIGHT,
  indicatorReadableDurationMs,
  indicatorSummaryLineCount,
  indicatorSummaryWraps,
  indicatorWindowSize,
} from './passive-indicator-layout';

const LONG_ERROR =
  'Could not reach the decision model because the network request timed out after thirty seconds. Check your connection and try again.';

test('short indicator labels stay on one content-sized line', () => {
  assert.equal(indicatorSummaryWraps('Listening'), false);
  assert.deepEqual(
    indicatorWindowSize({ title: 'Dictation', subtitle: 'Listening' }),
    {
      width: 240,
      height: INDICATOR_WINDOW_HEIGHT,
    },
  );
});

test('long indicator labels wrap within a bounded window height', () => {
  const lines = indicatorSummaryLineCount(LONG_ERROR);
  const size = indicatorWindowSize({
    title: 'Nevermind',
    subtitle: LONG_ERROR,
  });

  assert.equal(indicatorSummaryWraps(LONG_ERROR), true);
  assert.equal(lines, INDICATOR_MAX_SUMMARY_LINES);
  assert.equal(size.width, 500);
  assert.ok(size.height > INDICATOR_WINDOW_HEIGHT);
  assert.equal(
    indicatorSummaryLineCount(LONG_ERROR.repeat(10)),
    INDICATOR_MAX_SUMMARY_LINES,
  );
});

test('timed indicators stay visible long enough to read their label', () => {
  assert.equal(
    indicatorReadableDurationMs({ subtitle: 'Saved', durationMs: 2200 }),
    2200,
  );
  assert.ok(
    (indicatorReadableDurationMs({ subtitle: LONG_ERROR, durationMs: 4000 }) ??
      0) > 4000,
  );
  assert.equal(
    indicatorReadableDurationMs({
      subtitle: LONG_ERROR.repeat(20),
      durationMs: 4000,
    }),
    60_000,
  );
  assert.equal(
    indicatorReadableDurationMs({ subtitle: LONG_ERROR }),
    undefined,
  );
});
