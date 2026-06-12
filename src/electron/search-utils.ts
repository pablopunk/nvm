import crypto from 'node:crypto';
import { scoreNormalizedNonEmpty } from '../search-ranking';
import {
  calculate,
  calculateDetailed,
  calculateRateResult,
  parseRateExpression,
} from './calculator';

export {
  calculate,
  calculateDetailed,
  calculateRateResult,
  parseRateExpression,
};

export function normalize(value: unknown) {
  return String(value || '')
    .toLowerCase()
    .trim();
}

export function hashValue(value: unknown) {
  return crypto.createHash('sha1').update(String(value)).digest('hex');
}

export function scoreNormalized(value: unknown, q: string): number {
  if (!q) return 0;
  const v = normalize(value);
  return scoreNormalizedNonEmpty(v, q);
}

export function score(value: unknown, query: unknown) {
  return scoreNormalized(value, normalize(query));
}

export function isLikelyUrl(input: string) {
  const value = input.trim();
  if (!value || value.includes(' ')) return false;
  if (/^https?:\/\//i.test(value)) return true;
  return /^[\w-]+(\.[\w-]+)+([/:?#].*)?$/i.test(value);
}

export function getUrlFromQuery(query: string) {
  const trimmed = query.trim();
  const opened = trimmed.match(/^open\s+(.+)$/i)?.[1]?.trim() || trimmed;
  if (!isLikelyUrl(opened)) return null;
  return /^https?:\/\//i.test(opened) ? opened : `https://${opened}`;
}
