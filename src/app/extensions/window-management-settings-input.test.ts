// biome-ignore-all lint/style/noMagicNumbers: parse tables are literal boundary values.
import assert from 'node:assert/strict';
import test from 'node:test';
import { wholeNumberSettingFor } from './window-management-settings-definitions';
import { parseWholeNumberSettingInput } from './window-management-settings-input';

const OUTER_GAP_SETTING_FOR_TEST = wholeNumberSettingFor('outerGap');
const OUTER_GAP_EDGE_SETTING_FOR_TEST = wholeNumberSettingFor('outerGapTop');
const RESIZE_STEP_SETTING_FOR_TEST = wholeNumberSettingFor('resizeStepPercent');
const ALMOST_MAXIMIZE_WIDTH_SETTING_FOR_TEST = wholeNumberSettingFor(
  'almostMaximizeWidthPercent',
);

const PARSE_CASES: [string, unknown, boolean][] = [
  ['zero is a valid outer gap', '0', true],
  ['upper bound is valid', '40', true],
  ['surrounding spaces are ignored', '  7 ', true],
  ['above the gap maximum is rejected', '41', false],
  ['fractions are rejected', '12.5', false],
  ['negative values are rejected', '-1', false],
  ['empty input is rejected for a required setting', '', false],
  ['text is rejected', 'wide', false],
];

for (const [label, input, accepted] of PARSE_CASES) {
  test(`parse: ${label}`, () => {
    const result = parseWholeNumberSettingInput(
      input,
      OUTER_GAP_SETTING_FOR_TEST,
    );
    assert.equal(result.ok, accepted);
    if (result.ok === false) {
      assert.equal(result.message, 'Enter a whole number from 0 to 40.');
    } else {
      assert.equal(result.value, Number(String(input).trim()));
    }
  });
}

test('parse reports the range of the setting being edited', () => {
  assert.deepEqual(
    parseWholeNumberSettingInput('0', RESIZE_STEP_SETTING_FOR_TEST),
    { ok: false, message: 'Enter a whole number from 1 to 25.' },
  );
  assert.deepEqual(
    parseWholeNumberSettingInput('49', ALMOST_MAXIMIZE_WIDTH_SETTING_FOR_TEST),
    { ok: false, message: 'Enter a whole number from 50 to 100.' },
  );
});

test('a blank edge override parses to null, meaning follow the outer gap', () => {
  assert.deepEqual(
    parseWholeNumberSettingInput('  ', OUTER_GAP_EDGE_SETTING_FOR_TEST),
    { ok: true, value: null },
  );
});

test('an out-of-range edge override names the blank fallback in its error', () => {
  assert.deepEqual(
    parseWholeNumberSettingInput('41', OUTER_GAP_EDGE_SETTING_FOR_TEST),
    {
      ok: false,
      message:
        'Enter a whole number from 0 to 40, or leave blank to follow Outer Gap.',
    },
  );
});
