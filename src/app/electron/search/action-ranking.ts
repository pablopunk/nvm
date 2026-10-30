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

interface SearchableAction {
  aliases?: unknown;
  queryFallback?: unknown;
  subtitle?: unknown;
  title?: unknown;
}

const APP_RESULT_PRIORITY_BOOST = 25;
const EXACT_SEARCH_SCORE = 1000;
const EXACT_ALIAS_SEARCH_SCORE = 1050;
export const QUERY_FALLBACK_SEARCH_SCORE = 840;
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
  action: SearchableAction,
  query: string,
  additionalAliases: unknown[] = [],
) {
  const aliases = [
    ...(Array.isArray(action.aliases) ? action.aliases : []),
    ...additionalAliases,
  ];
  if (action.queryFallback === true)
    return Math.max(
      QUERY_FALLBACK_SEARCH_SCORE,
      aliasesSearchScore(aliases, query),
    );
  const titleScore = prioritizedTitleSearchScore(action.title, query);
  if (titleScore >= EXACT_SEARCH_SCORE) return titleScore;
  return Math.max(
    titleScore,
    scoreNormalized(action.subtitle, query),
    aliasesSearchScore(aliases, query),
  );
}

function aliasesSearchScore(aliases: unknown[], query: string) {
  let best = 0;
  for (const alias of aliases) {
    const aliasScore = prioritizedTitleSearchScore(alias, query);
    if (aliasScore >= EXACT_SEARCH_SCORE) return EXACT_ALIAS_SEARCH_SCORE;
    best = Math.max(best, aliasScore);
  }
  return best;
}

export function queryFallbackMarker(item: { queryFallback?: unknown }): {
  queryFallback?: true;
} {
  return item.queryFallback === true ? { queryFallback: true } : {};
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
