import {
  type ModelsDevApi,
  type ModelsDevModel,
  ownCatalogModel,
} from './pricing-catalog';
import { type PriceValidity, snapshotLookup } from './pricing-snapshot-store';

/**
 * When every bundled entry below was last reconciled against models.dev. One date
 * covers the whole bundle, so no model is special-cased: a bundled entry ages on the
 * same windows as a durable snapshot validated that day. Nothing here is refreshed by
 * the scheduled job, so the bundle is admissible only inside the hard bound. Moving
 * the date forward requires re-reconciling the entries in a reviewed change.
 */
export const BUNDLED_CATALOG_VERIFIED_AT = Date.parse('2026-10-29T00:00:00Z');

/**
 * Exact provider/model entries shipped with the server. They describe a model's
 * capabilities and, for opencode, its admission price while models.dev has not yet
 * been validated for that entry. OpenRouter prices never come from here, since a
 * bundled model-level price is not the endpoint a request may be routed to.
 */
const BUNDLED_RUNTIME_MODELS: ModelsDevApi = {
  opencode: {
    models: {
      'gpt-5.6-luna': {
        id: 'gpt-5.6-luna',
        name: 'GPT-5.6 Luna',
        reasoning: true,
        tool_call: true,
        modalities: { input: ['text', 'image', 'pdf'], output: ['text'] },
        limit: { context: 1_050_000, output: 128_000 },
        cost: {
          input: 0.2,
          output: 1.2,
          tiers: [
            { input: 0.4, output: 1.8, tier: { type: 'context', size: 272_000 } },
          ],
        },
      },
      'deepseek-v4-flash': {
        id: 'deepseek-v4-flash',
        name: 'DeepSeek V4 Flash',
        reasoning: true,
        tool_call: true,
        modalities: { input: ['text'], output: ['text'] },
        limit: { context: 1_000_000, output: 384_000 },
        cost: { input: 0.14, output: 0.28 },
      },
    },
  },
  openrouter: {
    models: {
      'deepseek/deepseek-v4-flash-0731': {
        id: 'deepseek/deepseek-v4-flash-0731',
        name: 'DeepSeek V4 Flash 0731',
        reasoning: true,
        tool_call: true,
        modalities: { input: ['text'], output: ['text'] },
        limit: { context: 1_048_576, output: 943_718 },
        cost: { input: 0.119, output: 0.238 },
      },
      'deepseek/deepseek-v4-pro-0813': {
        id: 'deepseek/deepseek-v4-pro-0813',
        name: 'DeepSeek V4 Pro 0813',
        reasoning: true,
        tool_call: true,
        modalities: { input: ['text'], output: ['text'] },
        limit: { context: 1_048_576, output: 393_216 },
        cost: { input: 0.66, output: 1.98 },
      },
      'google/gemini-2.5-flash': {
        id: 'google/gemini-2.5-flash',
        name: 'Gemini 2.5 Flash',
        reasoning: true,
        tool_call: true,
        attachment: true,
        modalities: {
          input: ['text', 'image', 'audio', 'video', 'pdf'],
          output: ['text'],
        },
        limit: { context: 1_048_576, output: 65_535 },
        cost: { input: 0.3, output: 2.5 },
      },
    },
  },
};

let bundledClock: () => number = Date.now;

/**
 * The bundle's validity on the snapshot windows, or null once it is past the hard
 * bound. A null validity means no bundled entry may be used for any model.
 */
export function bundledEntryValidity(): PriceValidity | null {
  const lookup = snapshotLookup(
    { value: true, validatedAt: BUNDLED_CATALOG_VERIFIED_AT },
    bundledClock(),
  );
  return lookup.status === 'current' || lookup.status === 'stale'
    ? lookup.status
    : null;
}

export function ownBundledModel(
  catalogKey: string,
  modelId: string,
): ModelsDevModel | undefined {
  return ownCatalogModel(BUNDLED_RUNTIME_MODELS[catalogKey], modelId);
}

export function bundledModelsFor(
  catalogKey: string,
): Record<string, ModelsDevModel> {
  return BUNDLED_RUNTIME_MODELS[catalogKey]?.models ?? {};
}

/** Evaluates the bundle at `now`; null restores the wall clock. Tests only. */
export function setBundledClockForTests(now: number | null) {
  bundledClock = now === null ? Date.now : () => now;
}
