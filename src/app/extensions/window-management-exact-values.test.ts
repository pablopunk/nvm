import assert from 'node:assert/strict';
import test from 'node:test';
import {
  type ExactDimensionInput,
  type ExactDimensionKey,
  exactDimensionParseFailed,
  exactWindowLayoutSpec,
  parseExactDimensions,
} from './window-management-exact-values';

const POINTS = 'points';
const PERCENT = 'percent';

function singleInput(key: ExactDimensionKey, text: string, unit: string) {
  return { [key]: { text, unit } } as Partial<
    Record<ExactDimensionKey, ExactDimensionInput>
  >;
}

test('blank fields are omitted so the window keeps its current value', () => {
  const result = parseExactDimensions({
    x: { text: '', unit: POINTS },
    y: { text: '  ', unit: POINTS },
  });
  assert.equal(exactDimensionParseFailed(result), true);
  assert.equal(result.message, 'Enter at least one value.');
});

test('a single provided value leaves the other dimensions out of the spec', () => {
  const result = parseExactDimensions({
    x: { text: '120', unit: POINTS },
    y: { text: '', unit: POINTS },
  });
  assert.equal(exactDimensionParseFailed(result), false);
  assert.deepEqual(exactWindowLayoutSpec(result.dimensions), {
    kind: 'exact',
    x: { value: 120, unit: POINTS },
  });
});

const VALID_CASES = [
  ['points offset at zero', 'x', '0', POINTS, { value: 0, unit: POINTS }],
  [
    'points offset at the cap',
    'y',
    '100000',
    POINTS,
    { value: 100_000, unit: POINTS },
  ],
  ['percent offset at 100', 'x', '100', PERCENT, { value: 100, unit: PERCENT }],
  [
    'decimal percent size',
    'width',
    '33.5',
    PERCENT,
    { value: 33.5, unit: PERCENT },
  ],
  [
    'points size just above zero',
    'height',
    '0.5',
    POINTS,
    { value: 0.5, unit: POINTS },
  ],
  ['padded text', 'width', ' 640 ', POINTS, { value: 640, unit: POINTS }],
] as const;

for (const [name, key, text, unit, expected] of VALID_CASES) {
  test(`accepts ${name}`, () => {
    const result = parseExactDimensions(singleInput(key, text, unit));
    assert.equal(exactDimensionParseFailed(result), false);
    assert.deepEqual(result.dimensions[key], expected);
  });
}

const INVALID_CASES = [
  [
    'negative points offset',
    'x',
    '-1',
    POINTS,
    'Enter a number from 0 to 100000.',
  ],
  [
    'points offset above cap',
    'x',
    '100001',
    POINTS,
    'Enter a number from 0 to 100000.',
  ],
  [
    'percent offset above 100',
    'y',
    '101',
    PERCENT,
    'Enter a number from 0 to 100%.',
  ],
  ['zero size', 'width', '0', POINTS, 'Enter a number above 0, up to 100000.'],
  [
    'zero percent size',
    'height',
    '0',
    PERCENT,
    'Enter a number above 0, up to 100%.',
  ],
  [
    'percent size above 100',
    'width',
    '150',
    PERCENT,
    'Enter a number above 0, up to 100%.',
  ],
  ['non-numeric text', 'x', 'abc', POINTS, 'Enter a number from 0 to 100000.'],
  [
    'scientific notation',
    'x',
    '1e3',
    POINTS,
    'Enter a number from 0 to 100000.',
  ],
  [
    'infinite text',
    'width',
    'Infinity',
    POINTS,
    'Enter a number above 0, up to 100000.',
  ],
] as const;

for (const [name, key, text, unit, message] of INVALID_CASES) {
  test(`rejects ${name} with a range message`, () => {
    const result = parseExactDimensions(singleInput(key, text, unit));
    assert.equal(exactDimensionParseFailed(result), true);
    assert.equal(result.errors[key], message);
  });
}

test('an unknown unit is rejected with a choice message', () => {
  const result = parseExactDimensions({
    width: { text: '50', unit: 'inches' },
  });
  assert.equal(result.errors.width, 'Choose points or percent.');
});

test('a unit is ignored when its value is blank', () => {
  const result = parseExactDimensions({
    width: { text: '', unit: 'inches' },
    height: { text: '50', unit: PERCENT },
  });
  assert.equal(exactDimensionParseFailed(result), false);
  assert.deepEqual(result.dimensions, {
    height: { value: 50, unit: PERCENT },
  });
});

test('every invalid field is reported, not only the first', () => {
  const result = parseExactDimensions({
    x: { text: '-4', unit: POINTS },
    y: { text: '200', unit: PERCENT },
  });
  assert.deepEqual(Object.keys(result.errors).sort(), ['x', 'y']);
});
