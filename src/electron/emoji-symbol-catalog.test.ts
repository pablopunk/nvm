import assert from 'node:assert/strict';
import test from 'node:test';
import {
  CHARACTER_RECORDS_BY_ID,
  characterCodePoints,
  characterWithSkinTone,
} from './emoji-symbol-catalog';

test('catalog exposes stable opaque ids for emoji and symbols', () => {
  assert.equal(CHARACTER_RECORDS_BY_ID.get('emoji:U+1F600')?.glyph, '😀');
  assert.equal(
    CHARACTER_RECORDS_BY_ID.get('symbol:U+2713')?.name,
    'check mark',
  );
});

test('formats Unicode code points for multi-codepoint sequences', () => {
  assert.equal(characterCodePoints('👩‍💻'), 'U+1F469 U+200D U+1F4BB');
});

test('applies skin tones after variation selectors and before joiners', () => {
  assert.equal(characterWithSkinTone('☝️', 1), '☝🏻');
  assert.equal(characterWithSkinTone('👩‍💻', 3), '👩🏽‍💻');
  assert.equal(characterWithSkinTone('🧑‍⚕️', 3), '🧑🏽‍⚕️');
  assert.equal(characterWithSkinTone('🧑‍🤝‍🧑', 3), '🧑🏽‍🤝‍🧑🏽');
});
