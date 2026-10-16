import assert from 'node:assert/strict';
import test from 'node:test';
import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { activityGraphemes, MorphingActivityText } from './morphing-text';

const VISIBLE_BASE_PATTERN = /class="chatActivityText">Read<\/span>/;
const LETTER_INDEX_PATTERN = /--activity-letter-index:/g;
const ANIMATED_OVERLAY_PATTERN =
  /class="chatActivityLetters" aria-hidden="true"/;
const READ_GRAPHEME_COUNT = 4;

test('splits activity labels into visible graphemes', () => {
  assert.deepEqual(activityGraphemes('Go 👨‍👩‍👧‍👦'), [
    'G',
    'o',
    '\u00a0',
    '👨‍👩‍👧‍👦',
  ]);
});

test('renders a Torph base label and one animated span per grapheme', () => {
  const markup = renderToStaticMarkup(<MorphingActivityText value="Read" />);

  assert.match(markup, VISIBLE_BASE_PATTERN);
  assert.equal(markup.match(LETTER_INDEX_PATTERN)?.length, READ_GRAPHEME_COUNT);
  assert.match(markup, ANIMATED_OVERLAY_PATTERN);
});
