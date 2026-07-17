import { warn as logWarn } from '../logger';
import {
  calculateDetailed,
  calculateRateResult,
  parseRateExpression,
} from '../search-utils';
import { extensionContext } from './_context';

const FRANKFURTER_API_BASE = 'https://api.frankfurter.dev';
const RATE_CACHE_MAX_AGE_MS = 24 * 60 * 60 * 1000;
const rateRefreshes = new Map<string, Promise<void>>();

function rateCacheKey(base: string, quote: string) {
  return `${String(base || '').toUpperCase()}/${String(quote || '').toUpperCase()}`;
}

function cachedRateQuote(base: string, quote: string) {
  const direct =
    extensionContext.userState.rateCache?.[rateCacheKey(base, quote)];
  if (direct?.rate) return direct;
  const inverse =
    extensionContext.userState.rateCache?.[rateCacheKey(quote, base)];
  if (inverse?.rate)
    return {
      ...inverse,
      rate: 1 / Number(inverse.rate),
      updatedAt: inverse.updatedAt,
      fetchedAt: inverse.fetchedAt,
    };
  return null;
}

function scheduleRateRefresh(base: string, quote: string, options: any = {}) {
  const key = rateCacheKey(base, quote);
  const cached = extensionContext.userState.rateCache?.[key];
  if (
    !options.force &&
    cached?.fetchedAt &&
    Date.now() - cached.fetchedAt < RATE_CACHE_MAX_AGE_MS
  )
    return;
  if (rateRefreshes.has(key)) return;
  const promise = refreshRate(base, quote)
    .catch((error) =>
      logWarn(
        'calculator.rate.refresh.failed',
        {
          base,
          quote,
          error: error instanceof Error ? error.message : String(error),
        },
        { source: 'host', scope: 'calculator' },
      ),
    )
    .finally(() => rateRefreshes.delete(key));
  rateRefreshes.set(key, promise);
}

async function refreshRate(base: string, quote: string) {
  const key = rateCacheKey(base, quote);
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), 5000);
  try {
    const response = await fetch(
      `${FRANKFURTER_API_BASE}/v2/rate/${encodeURIComponent(base)}/${encodeURIComponent(quote)}`,
      { signal: controller.signal },
    );
    if (!response.ok) throw new Error(`HTTP ${response.status}`);
    const data = (await response.json()) as any;
    const rate = Number(data.rate);
    if (!Number.isFinite(rate) || rate <= 0) throw new Error('Invalid rate');
    if (!extensionContext.userState.rateCache)
      extensionContext.userState.rateCache = {};
    extensionContext.userState.rateCache[key] = {
      rate,
      provider: 'Frankfurter',
      updatedAt: Date.parse(data.date || data.updated_at || '') || Date.now(),
      fetchedAt: Date.now(),
    };
    extensionContext.scheduleSaveState();
    extensionContext.invalidateExtensionRootItems();
  } finally {
    clearTimeout(timer);
  }
}

function calculatorResultItem(query: string, result: any) {
  const actions = [
    {
      type: 'copyText',
      title: 'Copy Result',
      text: result.formatted,
      dismissAfterRun: 'auto',
    },
    {
      type: 'setSearchQuery',
      title: 'Continue Calculation',
      subtitle: 'Replace search with the result',
      query: result.raw,
      shortcut: 'Command+Enter',
      keepPaletteOpen: true,
    },
    result.swapQuery
      ? {
          type: 'setSearchQuery',
          title: 'Swap Units',
          subtitle: 'Swap source and target',
          query: result.swapQuery,
          shortcut: 'Command+Shift+Enter',
          keepPaletteOpen: true,
        }
      : null,
    result.raw === result.formatted
      ? null
      : {
          type: 'copyText',
          title: 'Copy Unformatted Result',
          text: result.raw,
          dismissAfterRun: 'auto',
        },
    {
      type: 'pasteText',
      title: 'Paste Result',
      text: result.formatted,
      dismissAfterRun: 'auto',
    },
  ].filter(Boolean);
  return {
    id: `calculate:${query}`,
    title: result.title,
    subtitle: result.subtitle,
    aliases: [String(query || '').trim()],
    icon: 'calculator',
    score: 105,
    dismissAfterRun: 'auto',
    primaryAction: actions[0],
    actions: actions.slice(1),
  };
}

export function createCalculatorExtension() {
  return {
    id: 'nevermind.calculator',
    title: 'Calculator',
    capabilities: [] as const,
    commands: [],
    searchItems(_ctx, query) {
      const result = query ? calculateDetailed(query) : null;
      if (result) return [calculatorResultItem(query, result)];

      const rate = query ? parseRateExpression(query) : null;
      if (!rate) return [];
      const cached = cachedRateQuote(rate.sourceCurrency, rate.targetCurrency);
      scheduleRateRefresh(rate.sourceCurrency, rate.targetCurrency);
      if (!cached) return [];
      const rateResult = calculateRateResult(query, rate, cached);
      return rateResult ? [calculatorResultItem(query, rateResult)] : [];
    },
  };
}
