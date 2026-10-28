export type SettingsRecord = Record<string, unknown>;

export function isRecord(value: unknown): value is SettingsRecord {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

export function isFiniteNumber(value: unknown): value is number {
  return typeof value === 'number' && Number.isFinite(value);
}

export function clampWholeNumber(value: number, min: number, max: number) {
  return Math.min(max, Math.max(min, value));
}
