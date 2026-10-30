const INDICATOR_WINDOW_HEIGHT = 104;
const INDICATOR_MAX_SUMMARY_LINES = 10;
const PASSIVE_INDICATOR_MIN_WIDTH = 160;
const INDICATOR_WINDOW_MAX_WIDTH = 420;
const INDICATOR_HORIZONTAL_CHROME = 56;
const INDICATOR_APPROXIMATE_CHARACTER_WIDTH = 8;
const INDICATOR_SHADOW_HORIZONTAL_GUTTER = 80;
const INDICATOR_SUMMARY_LINE_HEIGHT = 18;
const INDICATOR_CHARACTERS_PER_LINE = Math.floor(
  (INDICATOR_WINDOW_MAX_WIDTH - INDICATOR_HORIZONTAL_CHROME) /
    INDICATOR_APPROXIMATE_CHARACTER_WIDTH,
);
const READING_TIME_BASE_MS = 1000;
const READING_TIME_PER_WORD_MS = 250;
const INDICATOR_MAX_DURATION_MS = 60_000;
const WHITESPACE_RUN = /\s+/;

interface PassiveIndicatorText {
  title?: unknown;
  subtitle?: unknown;
  status?: unknown;
  durationMs?: unknown;
}

function indicatorSummaryLabel(input: PassiveIndicatorText) {
  return String(input.subtitle || input.status || '');
}

function indicatorSummaryLineCount(label: string) {
  const estimatedLines = Math.ceil(
    Array.from(label).length / INDICATOR_CHARACTERS_PER_LINE,
  );
  return Math.max(1, Math.min(INDICATOR_MAX_SUMMARY_LINES, estimatedLines));
}

function indicatorSummaryWraps(label: string) {
  return indicatorSummaryLineCount(label) > 1;
}

function longestIndicatorLabelLength(input: PassiveIndicatorText) {
  return [input.title, input.subtitle, input.status].reduce<number>(
    (longest, value) =>
      Math.max(longest, Array.from(String(value || '')).length),
    0,
  );
}

function indicatorWindowSize(input: PassiveIndicatorText) {
  const contentWidth = Math.min(
    INDICATOR_WINDOW_MAX_WIDTH,
    Math.max(
      PASSIVE_INDICATOR_MIN_WIDTH,
      INDICATOR_HORIZONTAL_CHROME +
        longestIndicatorLabelLength(input) *
          INDICATOR_APPROXIMATE_CHARACTER_WIDTH,
    ),
  );
  const extraLines =
    indicatorSummaryLineCount(indicatorSummaryLabel(input)) - 1;
  return {
    width: contentWidth + INDICATOR_SHADOW_HORIZONTAL_GUTTER,
    height:
      INDICATOR_WINDOW_HEIGHT + extraLines * INDICATOR_SUMMARY_LINE_HEIGHT,
  };
}

function readingTimeMs(label: string) {
  const words = label.trim().split(WHITESPACE_RUN).filter(Boolean).length;
  return READING_TIME_BASE_MS + words * READING_TIME_PER_WORD_MS;
}

function indicatorReadableDurationMs(input: PassiveIndicatorText) {
  const requested = Number(input.durationMs);
  if (!(Number.isFinite(requested) && requested > 0)) {
    return;
  }
  return Math.min(
    INDICATOR_MAX_DURATION_MS,
    Math.max(requested, readingTimeMs(indicatorSummaryLabel(input))),
  );
}

export type { PassiveIndicatorText };
export {
  INDICATOR_MAX_SUMMARY_LINES,
  INDICATOR_WINDOW_HEIGHT,
  indicatorReadableDurationMs,
  indicatorSummaryLabel,
  indicatorSummaryLineCount,
  indicatorSummaryWraps,
  indicatorWindowSize,
};
