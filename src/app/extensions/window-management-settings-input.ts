import type { WholeNumberSettingDefinition } from './window-management-settings-definitions';

const WHOLE_NUMBER_TEXT_PATTERN = /^\d+$/;

function wholeNumberRangeMessage(definition: WholeNumberSettingDefinition) {
  const range = `Enter a whole number from ${definition.min} to ${definition.max}`;
  return definition.nullable
    ? `${range}, or leave blank to follow Outer Gap.`
    : `${range}.`;
}

function isInRange(value: number, definition: WholeNumberSettingDefinition) {
  return value >= definition.min && value <= definition.max;
}

export function parseWholeNumberSettingInput(
  input: unknown,
  definition: WholeNumberSettingDefinition,
): WholeNumberParseResult {
  const text = String(input ?? '').trim();
  if (text === '' && definition.nullable) {
    return { ok: true, value: null };
  }
  const value = Number(text);
  const isWholeNumber =
    WHOLE_NUMBER_TEXT_PATTERN.test(text) && isInRange(value, definition);
  if (!isWholeNumber) {
    return { ok: false, message: wholeNumberRangeMessage(definition) };
  }
  return { ok: true, value };
}

export type WholeNumberParseResult =
  | { ok: true; value: number | null }
  | { ok: false; message: string };
