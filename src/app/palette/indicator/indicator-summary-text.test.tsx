import assert from 'node:assert/strict';
import test from 'node:test';
import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { IndicatorSummaryText } from './indicator-summary-text';

const MORPHING_LABEL_PATTERN = /indicatorMorphingText/;
const WRAPPED_LABEL_PATTERN = /indicatorSummaryWrapped/;
const LONG_ERROR =
  'The decision model is unavailable because the provider returned an unexpected response. Try again shortly.';

test('long indicator summaries render as complete wrapping text', () => {
  const markup = renderToStaticMarkup(
    <IndicatorSummaryText value={LONG_ERROR} animate={true} />,
  );

  assert.equal(
    markup,
    `<span class="indicatorSummaryWrapped">${LONG_ERROR}</span>`,
  );
});

test('short indicator summaries keep the morphing label', () => {
  const markup = renderToStaticMarkup(
    <IndicatorSummaryText value="Listening" animate={true} />,
  );

  assert.match(markup, MORPHING_LABEL_PATTERN);
  assert.doesNotMatch(markup, WRAPPED_LABEL_PATTERN);
});

test('non-indicator progress summaries render plain text', () => {
  assert.equal(
    renderToStaticMarkup(
      <IndicatorSummaryText value={LONG_ERROR} animate={false} />,
    ),
    LONG_ERROR,
  );
});
