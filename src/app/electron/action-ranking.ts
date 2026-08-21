import { prioritizedTitleSearchScore, scoreNormalized } from './search-utils';

interface RankedAction {
  id?: unknown;
  kind?: unknown;
  extensionId?: unknown;
  score: number;
  lastUsed: number;
  title?: unknown;
  isAppResult?: unknown;
}

const APP_RESULT_PRIORITY_BOOST = 25;
const EXACT_SEARCH_SCORE = 1000;
const MAX_RECENCY_BOOST = 20;
const MILLISECONDS_PER_HOUR = 36e5;

const AI_BUILDER_CHAT_TIE_PRIORITY = -1;
const DEFAULT_TIE_PRIORITY = 0;

export function effectiveLastUsed(...timestamps: unknown[]) {
  return Math.max(0, ...timestamps.map((timestamp) => Number(timestamp) || 0));
}

export function recencyBoost(lastUsed: unknown, now = Date.now()) {
  const timestamp = Number(lastUsed) || 0;
  if (!timestamp) {
    return 0;
  }
  const ageHours = Math.max(0, (now - timestamp) / MILLISECONDS_PER_HOUR);
  return Math.max(0, MAX_RECENCY_BOOST - ageHours);
}

export function actionTextSearchScore(
  action: { aliases?: unknown; subtitle?: unknown; title?: unknown },
  query: string,
  additionalAliases: unknown[] = [],
) {
  let best = prioritizedTitleSearchScore(action.title, query);
  best = Math.max(best, scoreNormalized(action.subtitle, query));
  for (const alias of [
    ...(Array.isArray(action.aliases) ? action.aliases : []),
    ...additionalAliases,
  ]) {
    best = Math.max(best, prioritizedTitleSearchScore(alias, query));
    if (best >= EXACT_SEARCH_SCORE) {
      break;
    }
  }
  return best;
}

export function appResultMarker(
  item: { isAppResult?: unknown } & Record<string, unknown>,
): {
  isAppResult?: true;
} {
  if (item.isAppResult === true) {
    return { isAppResult: true };
  }
  return {};
}

export function priorityBoost(action: RankedAction): number {
  if (action.isAppResult === true) {
    return APP_RESULT_PRIORITY_BOOST;
  }
  return 0;
}

export function rankedActionTiePriority(action: RankedAction): number {
  if (
    action.kind === 'extension-root-item' &&
    action.extensionId === 'nevermind.ai-builder' &&
    String(action.id || '').startsWith(
      'extension-root:nevermind.ai-builder:ai-chat:',
    )
  ) {
    return AI_BUILDER_CHAT_TIE_PRIORITY;
  }
  return DEFAULT_TIE_PRIORITY;
}

export function compareRankedActions(a: RankedAction, b: RankedAction): number {
  return (
    b.score - a.score ||
    rankedActionTiePriority(b) - rankedActionTiePriority(a) ||
    b.lastUsed - a.lastUsed ||
    String(a.title || '').localeCompare(String(b.title || ''))
  );
}
