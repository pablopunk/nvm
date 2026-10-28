import type {
  WindowDimension,
  WindowLayoutSpec,
} from './window-management-types';

type ExactLayoutSpec = Extract<WindowLayoutSpec, { kind: 'exact' }>;
type ExactField = 'x' | 'y' | 'width' | 'height';
type ExactFieldKind = 'offset' | 'size';
type DimensionUnit = WindowDimension['unit'];

const EXACT_FIELDS: readonly ExactField[] = ['x', 'y', 'width', 'height'];
const EXACT_FIELD_KINDS: Record<ExactField, ExactFieldKind> = {
  x: 'offset',
  y: 'offset',
  width: 'size',
  height: 'size',
};
const MAX_VALUE_BY_UNIT: Record<DimensionUnit, number> = {
  points: 100_000,
  percent: 100,
};
const KNOWN_UNITS: readonly string[] = ['points', 'percent'];
const NO_EXACT_FIELDS_MESSAGE =
  'Choose at least one window value: x, y, width, or height.';

function unknownUnitMessage(field: ExactField): string {
  return `The window ${field} unit must be points or percent.`;
}

function notFiniteMessage(field: ExactField): string {
  return `The window ${field} must be a finite number.`;
}

function isWithinExactRange(
  value: number,
  kind: ExactFieldKind,
  unit: DimensionUnit,
): boolean {
  const aboveMinimum = kind === 'offset' ? value >= 0 : value > 0;
  return aboveMinimum && value <= MAX_VALUE_BY_UNIT[unit];
}

function describeOutOfRange(
  field: ExactField,
  kind: ExactFieldKind,
  unit: DimensionUnit,
): string {
  const max = MAX_VALUE_BY_UNIT[unit];
  return kind === 'offset'
    ? `The window ${field} must be between 0 and ${max} ${unit}.`
    : `The window ${field} must be above 0 and at most ${max} ${unit}.`;
}

function describeInvalidExactDimension(
  field: ExactField,
  dimension: WindowDimension,
): string | null {
  if (!Number.isFinite(dimension?.value)) {
    return notFiniteMessage(field);
  }
  if (!KNOWN_UNITS.includes(dimension.unit)) {
    return unknownUnitMessage(field);
  }
  const kind = EXACT_FIELD_KINDS[field];
  return isWithinExactRange(dimension.value, kind, dimension.unit)
    ? null
    : describeOutOfRange(field, kind, dimension.unit);
}

export function describeInvalidExactSpec(spec: ExactLayoutSpec): string | null {
  const presentFields = EXACT_FIELDS.filter(
    (field) => spec[field] !== undefined,
  );
  if (presentFields.length === 0) {
    return NO_EXACT_FIELDS_MESSAGE;
  }
  for (const field of presentFields) {
    const message = describeInvalidExactDimension(field, spec[field]);
    if (message !== null) {
      return message;
    }
  }
  return null;
}
