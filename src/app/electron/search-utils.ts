import crypto from 'node:crypto';
import { scoreFuzzy } from '../palette/search-ranking';

export {
  calculate,
  calculateDetailed,
  calculateRateResult,
  parseRateExpression,
} from './calculator';

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
  return scoreFuzzy(v, q);
}

export function prioritizedTitleSearchScore(value: unknown, q: string): number {
  if (!q) return 0;
  const normalizedTitle = normalize(value);
  const score = scoreNormalized(normalizedTitle, q);
  if (score === 100) return 1000;
  if (normalizedTitle.startsWith(q)) return 950;
  if (score === 95) return 925;
  if (score === 90) return 900;
  if (score === 70) return 875;
  if (score === 60) return 860;
  if (score === 50) return 850;
  if (score === 20) return 820;
  if (score === 10) return 810;
  return 0;
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
