import type { WindowDimension } from '../electron/os/window-management-types';
import type { ExtensionWindowLayoutSpec } from '../resources/nevermind-extension-api';

interface DimensionRule {
  min: number;
  max: number;
  minExclusive: boolean;
}

type DimensionAxis = 'offset' | 'size';

const MAX_POINTS = 100_000;

const MAX_PERCENT = 100;

const DECIMAL_TEXT_PATTERN = /^\d+(\.\d+)?$/;

const DIMENSION_RULES: Record<
  DimensionAxis,
  Record<ExactDimensionUnit, DimensionRule>
> = {
  offset: {
    points: { min: 0, max: MAX_POINTS, minExclusive: false },
    percent: { min: 0, max: MAX_PERCENT, minExclusive: false },
  },
  size: {
    points: { min: 0, max: MAX_POINTS, minExclusive: true },
    percent: { min: 0, max: MAX_PERCENT, minExclusive: true },
  },
};

function isBlankText(text: unknown) {
  return String(text ?? '').trim() === '';
}

function parseDimensionText(text: unknown) {
  const trimmed = String(text).trim();
  return DECIMAL_TEXT_PATTERN.test(trimmed) ? Number(trimmed) : Number.NaN;
}

function parseExactDimensionField(
  key: ExactDimensionKey,
  input: ExactDimensionInput,
): { dimension?: WindowDimension; message?: string } {
  if (isBlankText(input.text)) {
    return {};
  }
  if (!isExactDimensionUnit(input.unit)) {
    return { message: 'Choose points or percent.' };
  }
  const dimension = { value: parseDimensionText(input.text), unit: input.unit };
  if (!exactDimensionValueIsInRange(key, dimension)) {
    return { message: exactDimensionRangeMessage(key, input.unit) };
  }
  return { dimension };
}

export const EXACT_DIMENSION_KEYS = ['x', 'y', 'width', 'height'] as const;

export type ExactDimensionKey = (typeof EXACT_DIMENSION_KEYS)[number];

export type ExactDimensionUnit = WindowDimension['unit'];

export type ExactDimensionValues = Partial<
  Record<ExactDimensionKey, WindowDimension>
>;

export type ExactDimensionErrors = Partial<Record<ExactDimensionKey, string>>;

export interface ExactDimensionInput {
  text: unknown;
  unit: unknown;
}

export interface ExactDimensionParseResult {
  dimensions: ExactDimensionValues;
  errors: ExactDimensionErrors;
  message?: string;
}

export const EXACT_UNIT_OPTIONS: Array<{
  title: string;
  value: ExactDimensionUnit;
}> = [
  { title: 'Points', value: 'points' },
  { title: 'Percent of work area', value: 'percent' },
];

export function dimensionAxisFor(key: ExactDimensionKey): DimensionAxis {
  return key === 'x' || key === 'y' ? 'offset' : 'size';
}

export function isExactDimensionUnit(
  value: unknown,
): value is ExactDimensionUnit {
  return value === 'points' || value === 'percent';
}

export function exactDimensionRangeMessage(
  key: ExactDimensionKey,
  unit: ExactDimensionUnit,
) {
  const rule = DIMENSION_RULES[dimensionAxisFor(key)][unit];
  const maximum = `${rule.max}${unit === 'percent' ? '%' : ''}`;
  return rule.minExclusive
    ? `Enter a number above 0, up to ${maximum}.`
    : `Enter a number from 0 to ${maximum}.`;
}

export function exactDimensionValueIsInRange(
  key: ExactDimensionKey,
  dimension: WindowDimension,
) {
  const rule = DIMENSION_RULES[dimensionAxisFor(key)][dimension.unit];
  if (!Number.isFinite(dimension.value)) {
    return false;
  }
  if (dimension.value > rule.max) {
    return false;
  }
  if (rule.minExclusive) {
    return dimension.value > rule.min;
  }
  return dimension.value >= rule.min;
}

export function parseExactDimensions(
  inputs: Partial<Record<ExactDimensionKey, ExactDimensionInput>>,
): ExactDimensionParseResult {
  const dimensions: ExactDimensionValues = {};
  const errors: ExactDimensionErrors = {};
  for (const key of EXACT_DIMENSION_KEYS) {
    const input = inputs[key];
    if (!input) {
      continue;
    }
    const field = parseExactDimensionField(key, input);
    if (field.message) {
      errors[key] = field.message;
    }
    if (field.dimension) {
      dimensions[key] = field.dimension;
    }
  }
  if (Object.keys(errors).length > 0) {
    return { dimensions, errors };
  }
  if (Object.keys(dimensions).length === 0) {
    return { dimensions, errors, message: 'Enter at least one value.' };
  }
  return { dimensions, errors };
}

export function exactDimensionParseFailed(result: ExactDimensionParseResult) {
  return Object.keys(result.errors).length > 0 || result.message !== undefined;
}

export function exactWindowLayoutSpec(
  dimensions: ExactDimensionValues,
): ExtensionWindowLayoutSpec {
  return { kind: 'exact', ...dimensions };
}

export function formatExactDimension(dimension: WindowDimension) {
  return dimension.unit === 'percent'
    ? `${dimension.value}%`
    : `${dimension.value} pt`;
}
