export type AdminReportRange = { from: Date; to: Date };
export type AdminReportBounds = { from: string; to: string };
export type AdminPageCursor = {
  createdAt: string;
  id: string;
  bounds: AdminReportBounds;
};

const DAY_MS = 24 * 60 * 60 * 1000;
export const DEFAULT_ADMIN_PAGE_SIZE = 50;
export const MAX_ADMIN_PAGE_SIZE = 100;

function parseUtcBound(value: string): Date | null {
  const dateOnly = /^\d{4}-\d{2}-\d{2}$/.test(value);
  const timestamp = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,3})?Z$/.test(
    value,
  );
  if (!dateOnly && !timestamp) return null;
  const date = new Date(dateOnly ? `${value}T00:00:00.000Z` : value);
  if (
    Number.isNaN(date.getTime()) ||
    date.toISOString().slice(0, 10) !== value.slice(0, 10)
  )
    return null;
  return date;
}

function addUtcMonths(date: Date, count: number): Date {
  const targetYear =
    date.getUTCFullYear() + Math.floor((date.getUTCMonth() + count) / 12);
  const targetMonth = (date.getUTCMonth() + count) % 12;
  const lastDay = new Date(
    Date.UTC(targetYear, targetMonth + 1, 0),
  ).getUTCDate();
  return new Date(
    Date.UTC(
      targetYear,
      targetMonth,
      Math.min(date.getUTCDate(), lastDay),
      date.getUTCHours(),
      date.getUTCMinutes(),
      date.getUTCSeconds(),
      date.getUTCMilliseconds(),
    ),
  );
}

export function resolveAdminReportRange(
  fromInput: string | null,
  toInput: string | null,
  now = new Date(),
): AdminReportRange | null {
  if (Boolean(fromInput) !== Boolean(toInput)) return null;
  const effectiveNow = new Date(Math.floor(now.getTime() / 1000) * 1000);
  const from = fromInput
    ? parseUtcBound(fromInput)
    : new Date(effectiveNow.getTime() - 30 * DAY_MS);
  const to = toInput ? parseUtcBound(toInput) : effectiveNow;
  if (
    !from ||
    !to ||
    to.getTime() <= from.getTime() ||
    to.getTime() > effectiveNow.getTime()
  )
    return null;
  if (to.getTime() > addUtcMonths(from, 12).getTime()) return null;
  return { from, to };
}

export function reportBounds(range: AdminReportRange): AdminReportBounds {
  return { from: range.from.toISOString(), to: range.to.toISOString() };
}

export function parseAdminPageSize(value: string | null): number | null {
  if (value == null || value === '') return DEFAULT_ADMIN_PAGE_SIZE;
  if (!/^\d+$/.test(value)) return null;
  const size = Number(value);
  return Number.isInteger(size) && size >= 1 && size <= MAX_ADMIN_PAGE_SIZE
    ? size
    : null;
}

function normalizeCursorTimestamp(value: Date | string): string {
  if (value instanceof Date) {
    if (Number.isNaN(value.getTime()))
      throw new Error('Invalid cursor timestamp');
    return value.toISOString().replace(/Z$/, '000Z');
  }

  if (/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{6}Z$/.test(value)) {
    const millisecondTimestamp = value.replace(/(\.\d{3})\d{3}Z$/, '$1Z');
    const parsed = new Date(millisecondTimestamp);
    if (
      !Number.isNaN(parsed.getTime()) &&
      parsed.toISOString() === millisecondTimestamp
    )
      return value;
  }

  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime()))
    throw new Error('Invalid cursor timestamp');
  return parsed.toISOString().replace(/Z$/, '000Z');
}

export function encodeAdminPageCursor(input: {
  createdAt: Date | string;
  id: string | number;
  bounds: AdminReportBounds;
}): string {
  const cursor: AdminPageCursor = {
    createdAt: normalizeCursorTimestamp(input.createdAt),
    id: String(input.id),
    bounds: input.bounds,
  };
  return Buffer.from(JSON.stringify(cursor)).toString('base64url');
}

export function decodeAdminPageCursor(
  value: string | null,
  bounds: AdminReportBounds,
  isValidId: (id: string) => boolean,
): AdminPageCursor | null | undefined {
  if (value == null || value === '') return null;
  if (value.length > 1024) return undefined;
  try {
    if (!/^[A-Za-z0-9_-]+$/.test(value)) return undefined;
    const parsed = JSON.parse(
      Buffer.from(value, 'base64url').toString('utf8'),
    ) as Partial<AdminPageCursor>;
    const millisecondTimestamp =
      typeof parsed.createdAt === 'string'
        ? parsed.createdAt.replace(/(\.\d{3})\d{3}Z$/, '$1Z')
        : undefined;
    if (
      typeof parsed.createdAt !== 'string' ||
      !/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{6}Z$/.test(parsed.createdAt) ||
      !millisecondTimestamp ||
      Number.isNaN(new Date(millisecondTimestamp).getTime()) ||
      new Date(millisecondTimestamp).toISOString() !== millisecondTimestamp ||
      typeof parsed.id !== 'string' ||
      !isValidId(parsed.id) ||
      !parsed.bounds ||
      parsed.bounds.from !== bounds.from ||
      parsed.bounds.to !== bounds.to
    ) {
      return undefined;
    }
    return parsed as AdminPageCursor;
  } catch {
    return undefined;
  }
}

export function usageCursorIdIsValid(id: string): boolean {
  if (!/^[1-9]\d{0,18}$/.test(id)) return false;
  return BigInt(id) <= 9_223_372_036_854_775_807n;
}

export function userCursorIdIsValid(id: string): boolean {
  return /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(
    id,
  );
}
