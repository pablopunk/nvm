import { env } from './env';
import type { ModelCost } from './pricing';
import {
  loadSnapshotRecord,
  type PriceValidity,
  saveSnapshotRecord,
  type SnapshotLookup,
  snapshotLookup,
  type ValidatedSnapshot,
} from './pricing-snapshot-store';

/**
 * OpenRouter prices are per upstream endpoint. Routing may fail over between
 * endpoints, so admission reserves at the most expensive live endpoint rate.
 * Verified against GET /api/v1/models/{id}/endpoints: `data.id` echoes the
 * requested model, `pricing` values are USD-per-token decimal strings, and a
 * dynamic router reports `-1`, which is unpriceable.
 */
const DEFAULT_OPENROUTER_BASE_URL = 'https://openrouter.ai/api/v1';
const ENDPOINTS_TIMEOUT_MS = 8000;
const PER_MTOK = 1_000_000;

type TokenRates = {
  input: number;
  output: number;
  cacheRead: number;
  cacheWrite: number;
};
type TierRates = { thresholdTokens: number; rates: TokenRates };
type EndpointPrice = { base: TokenRates; tiers: TierRates[] };

const DECIMAL_RATE = /^\d+(?:\.\d+)?$/;

function isPlainRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

/** USD per token, or null when the field is not a non-negative decimal string. */
function perTokenRate(value: unknown): number | null {
  if (typeof value !== 'string' || !DECIMAL_RATE.test(value)) return null;
  const rate = Number(value);
  return Number.isFinite(rate) ? rate : null;
}

/** Absent or null optional rates are zero; a present malformed rate is null. */
function zeroableRate(fields: Record<string, unknown>, key: string) {
  if (fields[key] === undefined || fields[key] === null) return 0;
  return perTokenRate(fields[key]);
}

/** Absent cache rates bill as ordinary input, so they inherit the prompt rate. */
function optionalRate(fields: Record<string, unknown>, key: string) {
  if (fields[key] === undefined || fields[key] === null) return undefined;
  return perTokenRate(fields[key]);
}

function isZeroFee(value: unknown): boolean {
  return value === undefined || value === null || perTokenRate(value) === 0;
}

/**
 * Folds one pricing object into the rates a request can be charged. Image input
 * is billed per token and reachable, so it raises the input ceiling; internal
 * reasoning raises the output ceiling; a 1-hour cache write raises the cache
 * write ceiling. A per-request fee has no per-token form, so it is refused.
 * Audio input is refused by the chat request shape, and `web_search` and
 * non-text output are unreachable because the proxy strips plugins and routes
 * text output only, so none of those are folded.
 */
function ratesFromFields(fields: Record<string, unknown>): TokenRates | null {
  const prompt = perTokenRate(fields.prompt);
  const completion = perTokenRate(fields.completion);
  const image = zeroableRate(fields, 'image');
  const reasoning = zeroableRate(fields, 'internal_reasoning');
  const cacheRead = optionalRate(fields, 'input_cache_read');
  const cacheWrite = optionalRate(fields, 'input_cache_write');
  const cacheWrite1h = optionalRate(fields, 'input_cache_write_1h');
  if (
    prompt === null ||
    completion === null ||
    image === null ||
    reasoning === null ||
    cacheRead === null ||
    cacheWrite === null ||
    cacheWrite1h === null ||
    !isZeroFee(fields.request)
  )
    return null;
  return {
    input: Math.max(prompt, image),
    output: Math.max(completion, reasoning),
    cacheRead: cacheRead ?? prompt,
    cacheWrite: Math.max(cacheWrite ?? prompt, cacheWrite1h ?? 0),
  };
}

function maxRates(left: TokenRates, right: TokenRates): TokenRates {
  return {
    input: Math.max(left.input, right.input),
    output: Math.max(left.output, right.output),
    cacheRead: Math.max(left.cacheRead, right.cacheRead),
    cacheWrite: Math.max(left.cacheWrite, right.cacheWrite),
  };
}

/** Time-scoped overrides: a time-of-day window, a day-of-week set, or both. */
function isTimeScopedOverride(override: Record<string, unknown>): boolean {
  return (
    (typeof override.utc_start === 'number' &&
      typeof override.utc_end === 'number') ||
    Array.isArray(override.utc_days)
  );
}

/**
 * One endpoint's effective prices. `min_prompt_tokens` overrides are tiers that
 * apply from their threshold upward. Time-scoped overrides cannot be known at
 * admission, so every one folds into the base at its most expensive rate.
 */
function endpointPriceFrom(pricing: Record<string, unknown>): EndpointPrice | null {
  const base = ratesFromFields(pricing);
  if (!base) return null;
  const overrides = pricing.overrides ?? [];
  if (!Array.isArray(overrides)) return null;
  let folded = base;
  const tiers: TierRates[] = [];
  for (const override of overrides) {
    if (!isPlainRecord(override)) return null;
    const rates = ratesFromFields({ ...pricing, ...override });
    if (!rates) return null;
    if (
      Number.isSafeInteger(override.min_prompt_tokens) &&
      (override.min_prompt_tokens as number) >= 0
    ) {
      tiers.push({
        thresholdTokens: override.min_prompt_tokens as number,
        rates,
      });
    } else if (isTimeScopedOverride(override)) {
      folded = maxRates(folded, rates);
    } else {
      return null;
    }
  }
  return { base: folded, tiers };
}

function rateAt(price: EndpointPrice, inputTokens: number): TokenRates {
  let applied = price.base;
  let appliedThreshold = -1;
  for (const tier of price.tiers)
    if (
      tier.thresholdTokens <= inputTokens &&
      tier.thresholdTokens > appliedThreshold
    ) {
      applied = tier.rates;
      appliedThreshold = tier.thresholdTokens;
    }
  return applied;
}

/**
 * The most expensive charge across endpoints at every prompt size. Each tier
 * boundary from any endpoint becomes a boundary here, and each boundary carries
 * the maximum of the rates every endpoint would charge there.
 */
function aggregateEndpointPrices(prices: EndpointPrice[]) {
  const base = prices.map((price) => price.base).reduce(maxRates);
  const thresholds = [
    ...new Set(prices.flatMap((price) => price.tiers.map((t) => t.thresholdTokens))),
  ].sort((left, right) => left - right);
  const tiers = thresholds.map((thresholdTokens) => ({
    thresholdTokens,
    rates: prices
      .map((price) => rateAt(price, thresholdTokens))
      .reduce(maxRates),
  }));
  return { base, tiers };
}

/** Per-token decimal rates to USD per million tokens, without float drift. */
function perMtok(rate: number): number {
  return Number((rate * PER_MTOK).toPrecision(12));
}

function modelCostFromRates(
  modelId: string,
  aggregate: ReturnType<typeof aggregateEndpointPrices>,
): ModelCost {
  const priced = (rates: TokenRates) => ({
    inputUsdPerMtok: perMtok(rates.input),
    outputUsdPerMtok: perMtok(rates.output),
    cacheReadUsdPerMtok: perMtok(rates.cacheRead),
    cacheWriteUsdPerMtok: perMtok(rates.cacheWrite),
  });
  return {
    provider: 'openrouter',
    modelId,
    ...priced(aggregate.base),
    ...(aggregate.tiers.length
      ? {
          tiers: aggregate.tiers.map((tier) => ({
            thresholdTokens: tier.thresholdTokens,
            ...priced(tier.rates),
          })),
        }
      : {}),
  };
}

/**
 * Parses a `/models/{id}/endpoints` body. `valid: false` means the body is not an
 * endpoint listing at all and must not replace a prior validation. `cost: null`
 * means a valid listing that cannot be priced, such as a dynamic router or a
 * fee refused above.
 */
export function parseOpenRouterEndpoints(
  modelId: string,
  body: unknown,
): { valid: true; cost: ModelCost | null } | { valid: false } {
  if (
    !isPlainRecord(body) ||
    !isPlainRecord(body.data) ||
    body.data.id !== modelId ||
    !Array.isArray(body.data.endpoints)
  )
    return { valid: false };
  const endpoints = body.data.endpoints;
  if (endpoints.length === 0) return { valid: true, cost: null };
  const prices: EndpointPrice[] = [];
  for (const endpoint of endpoints) {
    const price = isPlainRecord(endpoint) && isPlainRecord(endpoint.pricing)
      ? endpointPriceFrom(endpoint.pricing)
      : null;
    if (!price) return { valid: true, cost: null };
    prices.push(price);
  }
  return {
    valid: true,
    cost: modelCostFromRates(modelId, aggregateEndpointPrices(prices)),
  };
}

function isModelCostPayload(value: unknown): value is ModelCost {
  return (
    isPlainRecord(value) &&
    typeof value.inputUsdPerMtok === 'number' &&
    Number.isFinite(value.inputUsdPerMtok) &&
    typeof value.outputUsdPerMtok === 'number' &&
    Number.isFinite(value.outputUsdPerMtok)
  );
}

function isEndpointPricePayload(
  value: unknown,
): value is { cost: ModelCost | null } {
  return (
    isPlainRecord(value) &&
    (value.cost === null || isModelCostPayload(value.cost))
  );
}

function openRouterApiBaseUrl() {
  return String(env('OPENROUTER_BASE_URL') ?? DEFAULT_OPENROUTER_BASE_URL).replace(
    /\/$/,
    '',
  );
}

function endpointsUrl(modelId: string) {
  const path = modelId
    .split('/')
    .map((segment) => encodeURIComponent(segment).replace(/%3A/gi, ':'))
    .join('/');
  return `${openRouterApiBaseUrl()}/models/${path}/endpoints`;
}

async function fetchEndpointCost(modelId: string): Promise<ModelCost | null> {
  const response = await fetch(endpointsUrl(modelId), {
    signal: AbortSignal.timeout(ENDPOINTS_TIMEOUT_MS),
  });
  if (!response.ok) throw new Error(`OpenRouter endpoints ${response.status}`);
  const parsed = parseOpenRouterEndpoints(modelId, await response.json());
  if (!parsed.valid) throw new Error('OpenRouter endpoints failed shape validation');
  return parsed.cost;
}

const RESTORE_RETRY_MS = 30 * 1000;
const prices = new Map<string, ValidatedSnapshot<ModelCost | null>>();
const restoreAttemptedAt = new Map<string, number>();
const refreshInflight = new Map<string, Promise<void>>();

function snapshotKey(modelId: string) {
  return `openrouter_endpoints:${modelId}`;
}

async function restoreEndpointPrice(modelId: string) {
  const now = Date.now();
  const attemptedAt = restoreAttemptedAt.get(modelId);
  if (attemptedAt !== undefined && now - attemptedAt < RESTORE_RETRY_MS) return;
  restoreAttemptedAt.set(modelId, now);
  const record = await loadSnapshotRecord(snapshotKey(modelId));
  if (!record || !isEndpointPricePayload(record.payload)) return;
  const current = prices.get(modelId);
  if (!current || current.validatedAt < record.validatedAt)
    prices.set(modelId, {
      value: record.payload.cost,
      validatedAt: record.validatedAt,
    });
}

async function publishEndpointPrice(modelId: string) {
  const cost = await fetchEndpointCost(modelId);
  const validatedAt = Date.now();
  prices.set(modelId, { value: cost, validatedAt });
  const stored = await saveSnapshotRecord({
    key: snapshotKey(modelId),
    validatedAt,
    payload: { cost },
  });
  if (!stored) throw new Error('OpenRouter endpoint price was not stored durably');
}

/**
 * Fetches one model's endpoint listing and publishes it. Only the scheduled refresh
 * and admin-selected routes call this, never the inference request path. Concurrent
 * callers for one model share one fetch.
 */
export function refreshOpenRouterEndpointPrice(modelId: string): Promise<void> {
  const inflight = refreshInflight.get(modelId);
  if (inflight) return inflight;
  const refresh = publishEndpointPrice(modelId).finally(() => {
    refreshInflight.delete(modelId);
  });
  refreshInflight.set(modelId, refresh);
  return refresh;
}

/**
 * The durable endpoint price for one OpenRouter model, restored when it is not current.
 * The network is never reached here.
 */
export async function currentOpenRouterPrice(
  modelId: string,
): Promise<SnapshotLookup<ModelCost | null>> {
  if (snapshotLookup(prices.get(modelId)).status !== 'current')
    await restoreEndpointPrice(modelId);
  return snapshotLookup(prices.get(modelId));
}

/** Synchronous view for hot paths: the endpoint price as last loaded, when it may admit. */
export function peekOpenRouterPrice(
  modelId: string,
): { value: ModelCost; validity: PriceValidity } | undefined {
  const lookup = snapshotLookup(prices.get(modelId));
  if (lookup.status === 'current' && lookup.value)
    return { value: lookup.value, validity: 'current' };
  if (lookup.status === 'stale' && lookup.value)
    return { value: lookup.value, validity: 'stale' };
  return undefined;
}

export function expireOpenRouterPricesForTests(expiredAt: number) {
  for (const [modelId, entry] of prices)
    prices.set(modelId, {
      ...entry,
      validatedAt: Math.min(entry.validatedAt, expiredAt),
    });
  restoreAttemptedAt.clear();
}

export function resetOpenRouterPricesForTests() {
  prices.clear();
  restoreAttemptedAt.clear();
  refreshInflight.clear();
}
