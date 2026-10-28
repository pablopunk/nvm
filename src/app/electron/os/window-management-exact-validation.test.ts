// biome-ignore-all lint/style/noMagicNumbers: validation limits read best as literals
import assert from 'node:assert/strict';
import test from 'node:test';
import { resolveWindowManagementOptions } from './window-management-options';
import type {
  WindowDimension,
  WindowLayoutSpec,
  WindowManagementOptions,
} from './window-management-types';
import { describeInvalidRequest } from './window-management-validation';

const VALID_OPTIONS: WindowManagementOptions = resolveWindowManagementOptions();
const NO_FIELDS_MESSAGE =
  'Choose at least one window value: x, y, width, or height.';

function points(value: number): WindowDimension {
  return { value, unit: 'points' };
}

function percent(value: number): WindowDimension {
  return { value, unit: 'percent' };
}

function exactWith(
  field: 'x' | 'y' | 'width' | 'height',
  dimension: WindowDimension,
): WindowLayoutSpec {
  return { kind: 'exact', [field]: dimension };
}

const REJECTED_EXACT_SPECS: [string, WindowLayoutSpec, string][] = [
  ['no fields', { kind: 'exact' }, NO_FIELDS_MESSAGE],
  [
    'a non-finite x value',
    exactWith('x', points(Number.NaN)),
    'The window x must be a finite number.',
  ],
  [
    'an infinite width value',
    exactWith('width', points(Number.POSITIVE_INFINITY)),
    'The window width must be a finite number.',
  ],
  [
    'an unknown unit',
    exactWith('height', {
      value: 10,
      unit: 'pixels',
    } as unknown as WindowDimension),
    'The window height unit must be points or percent.',
  ],
  [
    'a negative point offset',
    exactWith('x', points(-1)),
    'The window x must be between 0 and 100000 points.',
  ],
  [
    'a point offset above 100000',
    exactWith('y', points(100_001)),
    'The window y must be between 0 and 100000 points.',
  ],
  [
    'a percent offset above 100',
    exactWith('x', percent(101)),
    'The window x must be between 0 and 100 percent.',
  ],
  [
    'a zero point width',
    exactWith('width', points(0)),
    'The window width must be above 0 and at most 100000 points.',
  ],
  [
    'a point height above 100000',
    exactWith('height', points(100_001)),
    'The window height must be above 0 and at most 100000 points.',
  ],
  [
    'a zero percent height',
    exactWith('height', percent(0)),
    'The window height must be above 0 and at most 100 percent.',
  ],
  [
    'a percent width above 100',
    exactWith('width', percent(100.5)),
    'The window width must be above 0 and at most 100 percent.',
  ],
];

const ACCEPTED_EXACT_SPECS: [string, WindowLayoutSpec][] = [
  ['only x in points', { kind: 'exact', x: points(0) }],
  [
    'only y in percent at the offset maximum',
    { kind: 'exact', y: percent(100) },
  ],
  [
    'a point width at the size maximum',
    { kind: 'exact', width: points(100_000) },
  ],
  [
    'every field in mixed units',
    {
      kind: 'exact',
      x: points(10),
      y: percent(10),
      width: percent(50),
      height: points(300),
    },
  ],
];

for (const [name, spec, message] of REJECTED_EXACT_SPECS) {
  test(`rejects an exact layout with ${name} and says why`, () => {
    assert.equal(describeInvalidRequest(spec, VALID_OPTIONS), message);
  });
}

for (const [name, spec] of ACCEPTED_EXACT_SPECS) {
  test(`accepts an exact layout with ${name}`, () => {
    assert.equal(describeInvalidRequest(spec, VALID_OPTIONS), null);
  });
}
