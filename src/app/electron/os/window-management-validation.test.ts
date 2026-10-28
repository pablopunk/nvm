// biome-ignore-all lint/style/noMagicNumbers: fixture geometry reads best as literals
import assert from 'node:assert/strict';
import test from 'node:test';
import {
  INVALID_OPTIONS_MESSAGE,
  INVALID_SPEC_MESSAGE,
} from './window-management-failures';
import type {
  WindowCycleGroup,
  WindowEdge,
  WindowLayoutSpec,
  WindowManagementOptions,
} from './window-management-types';
import { describeInvalidRequest } from './window-management-validation';

const VALID_OPTIONS: WindowManagementOptions = { gap: 0, cycle: true };
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

test('rejects a non-boolean cycle option', () => {
  const options = {
    gap: 0,
    cycle: 'yes',
  } as unknown as WindowManagementOptions;
  assert.equal(
    describeInvalidRequest({ kind: 'center' }, options),
    INVALID_OPTIONS_MESSAGE,
  );
});

test('rejects a missing cycle option', () => {
  const options = { gap: 0 } as unknown as WindowManagementOptions;
  assert.equal(
    describeInvalidRequest({ kind: 'center' }, options),
    INVALID_OPTIONS_MESSAGE,
  );
});

test('accepts cycle set to false', () => {
  assert.equal(
    describeInvalidRequest({ kind: 'center' }, { gap: 100, cycle: false }),
    null,
  );
});
