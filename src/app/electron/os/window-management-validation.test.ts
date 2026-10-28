// biome-ignore-all lint/style/noMagicNumbers: fixture geometry reads best as literals
import assert from 'node:assert/strict';
import test from 'node:test';
import { INVALID_SPEC_MESSAGE } from './window-management-failures';
import { resolveWindowManagementOptions } from './window-management-options';
import type {
  WindowCycleGroup,
  WindowEdge,
  WindowLayoutSpec,
  WindowManagementOptions,
} from './window-management-types';
import { describeInvalidRequest } from './window-management-validation';

const VALID_OPTIONS: WindowManagementOptions = resolveWindowManagementOptions();
const INVALID_EDGE = 'diagonal' as WindowEdge;
const INVALID_DIRECTION = 'bigger' as 'larger';
const INVALID_DISPLAY_DIRECTION = 'sideways' as 'next';
const INVALID_CYCLE_GROUP = 'middle' as WindowCycleGroup;

const INVALID_SPECS: [string, WindowLayoutSpec][] = [
  [
    'scaled-center width of zero',
    { kind: 'scaled-center', widthFraction: 0, heightFraction: 0.9 },
  ],
  [
    'scaled-center width above one',
    { kind: 'scaled-center', widthFraction: 1.5, heightFraction: 0.9 },
  ],
  [
    'scaled-center NaN height',
    { kind: 'scaled-center', widthFraction: 0.9, heightFraction: Number.NaN },
  ],
  [
    'scaled-center zero maxWidth',
    {
      kind: 'scaled-center',
      widthFraction: 0.6,
      heightFraction: 0.6,
      maxWidth: 0,
    },
  ],
  [
    'scaled-center infinite maxHeight',
    {
      kind: 'scaled-center',
      widthFraction: 0.6,
      heightFraction: 0.6,
      maxHeight: Number.POSITIVE_INFINITY,
    },
  ],
  ['edge outside the four sides', { kind: 'edge', edge: INVALID_EDGE }],
  [
    'resize-step below the minimum',
    { kind: 'resize-step', direction: 'larger', step: 0.005 },
  ],
  [
    'resize-step above the maximum',
    { kind: 'resize-step', direction: 'smaller', step: 0.3 },
  ],
  [
    'resize-step NaN',
    { kind: 'resize-step', direction: 'larger', step: Number.NaN },
  ],
  [
    'resize-step unknown direction',
    { kind: 'resize-step', direction: INVALID_DIRECTION, step: 0.05 },
  ],
  [
    'display unknown direction',
    { kind: 'display', direction: INVALID_DISPLAY_DIRECTION },
  ],
  [
    'fraction unknown cycle group',
    {
      kind: 'fraction',
      x: 0,
      y: 0,
      width: 0.5,
      height: 1,
      cycleGroup: INVALID_CYCLE_GROUP,
    },
  ],
  ['unknown kind', { kind: 'bogus' } as unknown as WindowLayoutSpec],
];

const VALID_SPECS: [string, WindowLayoutSpec][] = [
  ['center', { kind: 'center' }],
  ['maximize-width', { kind: 'maximize-width' }],
  ['maximize-height', { kind: 'maximize-height' }],
  ['restore', { kind: 'restore' }],
  ['toggle-full-screen', { kind: 'toggle-full-screen' }],
  [
    'scaled-center without caps',
    { kind: 'scaled-center', widthFraction: 0.9, heightFraction: 0.9 },
  ],
  [
    'scaled-center with caps',
    {
      kind: 'scaled-center',
      widthFraction: 0.6,
      heightFraction: 0.6,
      maxWidth: 1025,
      maxHeight: 900,
    },
  ],
  ['edge left', { kind: 'edge', edge: 'left' }],
  ['edge right', { kind: 'edge', edge: 'right' }],
  ['edge top', { kind: 'edge', edge: 'top' }],
  ['edge bottom', { kind: 'edge', edge: 'bottom' }],
  [
    'resize-step larger at the minimum',
    { kind: 'resize-step', direction: 'larger', step: 0.01 },
  ],
  [
    'resize-step smaller at the maximum',
    { kind: 'resize-step', direction: 'smaller', step: 0.25 },
  ],
  ['display next', { kind: 'display', direction: 'next' }],
  ['display previous', { kind: 'display', direction: 'previous' }],
  [
    'fraction with a cycle group',
    { kind: 'fraction', x: 0, y: 0, width: 1, height: 0.5, cycleGroup: 'top' },
  ],
];

for (const [name, spec] of INVALID_SPECS) {
  test(`rejects an invalid ${name} with the spec message`, () => {
    assert.equal(
      describeInvalidRequest(spec, VALID_OPTIONS),
      INVALID_SPEC_MESSAGE,
    );
  });
}

for (const [name, spec] of VALID_SPECS) {
  test(`accepts a valid ${name}`, () => {
    assert.equal(describeInvalidRequest(spec, VALID_OPTIONS), null);
  });
}

const CENTER: WindowLayoutSpec = { kind: 'center' };
function outerGapRangeMessage(edge: string): string {
  return `The outer ${edge} gap must be a whole number from 0 to 40.`;
}
const INNER_GAP_RANGE_MESSAGE =
  'The inner gap must be a whole number from 0 to 40.';

function optionsWith(patch: Record<string, unknown>): WindowManagementOptions {
  return { ...VALID_OPTIONS, ...patch } as unknown as WindowManagementOptions;
}

const INVALID_OPTION_CASES: Array<[string, Record<string, unknown>, string]> = [
  [
    'a non-boolean cycle',
    { cycle: 'yes' },
    'The cycle option must be true or false.',
  ],
  [
    'a missing cycle',
    { cycle: undefined },
    'The cycle option must be true or false.',
  ],
  ['an inner gap above 40', { innerGap: 41 }, INNER_GAP_RANGE_MESSAGE],
  ['a fractional inner gap', { innerGap: 1.5 }, INNER_GAP_RANGE_MESSAGE],
  ['a negative inner gap', { innerGap: -1 }, INNER_GAP_RANGE_MESSAGE],
  ['a string inner gap', { innerGap: '4' }, INNER_GAP_RANGE_MESSAGE],
  ['a missing outer gap', { outerGap: undefined }, outerGapRangeMessage('top')],
  [
    'an outer right gap above 40',
    { outerGap: { ...VALID_OPTIONS.outerGap, right: 41 } },
    outerGapRangeMessage('right'),
  ],
  [
    'a partial outer gap',
    { outerGap: { top: 8 } },
    outerGapRangeMessage('right'),
  ],
  [
    'a negative outer bottom gap',
    { outerGap: { ...VALID_OPTIONS.outerGap, bottom: -2 } },
    outerGapRangeMessage('bottom'),
  ],
  [
    'a non-boolean apply gap to maximize',
    { applyGapToMaximize: 'no' },
    'The apply gap to maximize option must be true or false.',
  ],
  [
    'an unknown cycle order',
    { cycleOrder: 'third-first' },
    'The cycle order must be half-two-thirds-third or half-third-two-thirds.',
  ],
  [
    'an unknown display order',
    { displayOrder: 'diagonal' },
    'The display order must be left-to-right or top-to-bottom.',
  ],
  [
    'an unknown display move sizing',
    { displayMoveSizing: 'stretch' },
    'The display move sizing must be proportional or keep-size.',
  ],
];

for (const [name, patch, message] of INVALID_OPTION_CASES) {
  test(`rejects ${name} with the option message`, () => {
    assert.equal(describeInvalidRequest(CENTER, optionsWith(patch)), message);
  });
}

test('accepts gaps at the 0 and 40 point limits on every edge', () => {
  const options = optionsWith({
    outerGap: { top: 0, right: 40, bottom: 0, left: 40 },
    innerGap: 40,
  });
  assert.equal(describeInvalidRequest(CENTER, options), null);
});

test('accepts cycle set to false', () => {
  assert.equal(
    describeInvalidRequest(CENTER, optionsWith({ cycle: false })),
    null,
  );
});

test('accepts every cycle order, display order and display move sizing', () => {
  for (const cycleOrder of ['half-two-thirds-third', 'half-third-two-thirds']) {
    for (const displayOrder of ['left-to-right', 'top-to-bottom']) {
      for (const displayMoveSizing of ['proportional', 'keep-size']) {
        assert.equal(
          describeInvalidRequest(
            CENTER,
            optionsWith({ cycleOrder, displayOrder, displayMoveSizing }),
          ),
          null,
        );
      }
    }
  }
});
