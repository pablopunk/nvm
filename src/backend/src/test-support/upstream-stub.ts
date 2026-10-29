import { resetDecisionModelCacheForTests } from '../lib/decisions/decision-models';
import { bundledModelsFor, ownBundledModel } from '../lib/pricing-bundled';
import { resetPricingCacheForTests } from '../lib/pricing';
import { refreshPricingSnapshots } from '../lib/pricing-refresh';
import { resetTranscriptionModelCacheForTests } from '../lib/transcription-models';
import { resetUpstreamReadinessCacheForTests } from '../lib/upstream-readiness';
import {
  openRouterEndpointsBodyFromCost,
  openRouterEndpointsModelId,
} from './openrouter-endpoints';

export const OPENROUTER_BASE_URL = 'https://openrouter.test/api/v1';
export const OPENCODE_BASE_URL = 'https://opencode.test/zen/v1';
export const OPENROUTER_MODELS_URL = `${OPENROUTER_BASE_URL}/models`;
export const OPENROUTER_KEY_URL = `${OPENROUTER_BASE_URL}/key`;
export const OPENROUTER_TRANSCRIPTION_URL = `${OPENROUTER_BASE_URL}/models?input_modalities=audio&output_modalities=transcription`;
export const OPENROUTER_DECISION_URL = `${OPENROUTER_BASE_URL}/models?output_modalities=decisions`;
export const OPENCODE_MODELS_URL = `${OPENCODE_BASE_URL}/models`;
export const MODELS_DEV_URL = 'https://models.dev/api.json';

export const CATALOG_BODY = JSON.stringify({ object: 'list', data: [{ id: 'model-a' }] });
export const KEY_BODY = JSON.stringify({ data: { label: 'test' } });
export const TRANSCRIPTION_BODY = JSON.stringify({
  data: [
    {
      id: 'nvidia/parakeet-tdt-0.6b-v3',
      name: 'Parakeet',
      pricing: { prompt: '0.000025', completion: '0' },
      architecture: { input_modalities: ['audio'], output_modalities: ['transcription'] },
    },
  ],
});
export const DECISION_BODY = JSON.stringify({
  data: [
    {
      id: 'typesafe/jev-1.14',
      name: 'Jev',
      context_length: 262144,
      pricing: { prompt: '0.000000042', completion: '0', image: '0', request: '0' },
      architecture: { input_modalities: ['text', 'image'], output_modalities: ['decisions'] },
    },
  ],
});

export type UpstreamReply = { status: number; contentType?: string; body?: string } | 'timeout';
export type UpstreamReplies = Record<string, UpstreamReply>;

// Every live OpenRouter route, with each catalog that the request path reads.
export const OPENROUTER_ACCEPTED_REPLIES: UpstreamReplies = {
  [OPENROUTER_MODELS_URL]: { status: 200, body: CATALOG_BODY },
  [OPENROUTER_KEY_URL]: { status: 200, body: KEY_BODY },
  [OPENROUTER_TRANSCRIPTION_URL]: { status: 200, body: TRANSCRIPTION_BODY },
  [OPENROUTER_DECISION_URL]: { status: 200, body: DECISION_BODY },
};

const originalFetch = globalThis.fetch;

// OpenRouter endpoint pricing answers as a listing whose one endpoint charges the test's
// models.dev cost, or the shipped bundled cost for that model. A model neither source
// prices answers 404, as OpenRouter does for an unknown model.
function openRouterEndpointsReply(
  modelId: string,
  catalogReply: UpstreamReply | undefined,
): UpstreamReply {
  const catalog =
    catalogReply && catalogReply !== 'timeout'
      ? JSON.parse(catalogReply.body ?? '{}')
      : {};
  const cost =
    catalog.openrouter?.models?.[modelId]?.cost ??
    ownBundledModel('openrouter', modelId)?.cost;
  if (!cost) return { status: 404, body: '{}' };
  return {
    status: 200,
    body: JSON.stringify(openRouterEndpointsBodyFromCost(modelId, cost)),
  };
}

// Answers pricing with an empty models.dev catalog unless a test supplies one. Pricing
// fetches are left out of the returned request list so probe assertions stay focused.
// The install then publishes prices and the audio and decision catalogs the way the
// scheduled refresh does, because the request path never fetches them. By default
// that is every bundled OpenRouter model. Install-time fetches are not recorded.
export async function installUpstreamStub(
  replies: UpstreamReplies,
  pricedOpenRouterModels: readonly string[] = Object.keys(bundledModelsFor('openrouter')),
): Promise<string[]> {
  const requested: string[] = [];
  let recording = false;
  const answers: UpstreamReplies = { [MODELS_DEV_URL]: { status: 200, body: '{}' }, ...replies };
  globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input);
    const endpointsModelId = openRouterEndpointsModelId(url);
    const isPricingFetch = url === MODELS_DEV_URL || endpointsModelId !== null;
    if (recording && !isPricingFetch) requested.push(url);
    const reply =
      answers[url] ??
      (endpointsModelId === null
        ? { status: 500, body: '{}' }
        : openRouterEndpointsReply(endpointsModelId, answers[MODELS_DEV_URL]));
    if (reply === 'timeout')
      return new Promise<Response>((_resolve, reject) =>
        init?.signal?.addEventListener('abort', () =>
          reject(new DOMException('The operation was aborted.', 'AbortError')),
        ),
      );
    return new Response(reply.body ?? '', {
      status: reply.status,
      headers: { 'content-type': reply.contentType ?? 'application/json' },
    });
  }) as typeof fetch;
  await refreshPricingSnapshots(pricedOpenRouterModels);
  recording = true;
  return requested;
}

const TEST_ENV_KEYS = [
  'OPENROUTER_API_KEY',
  'OPENROUTER_BASE_URL',
  'OPENCODE_API_KEY',
  'OPENCODE_BASE_URL',
  'ANTHROPIC_API_KEY',
  'ANTHROPIC_BASE_URL',
  'OPENAI_API_KEY',
  'OPENAI_BASE_URL',
  'GOOGLE_API_KEY',
  'GOOGLE_BASE_URL',
  'NEVERMIND_KILL_SWITCHES',
] as const;

function resetUpstreamTestCaches() {
  resetPricingCacheForTests();
  resetTranscriptionModelCacheForTests();
  resetDecisionModelCacheForTests();
  resetUpstreamReadinessCacheForTests();
}

// A deployment shaped like production: OpenRouter keyed, OpenCode unkeyed, no kill
// switches. Returns the teardown that restores env, fetch, and every module cache.
export function useUpstreamTestEnv(): () => void {
  const saved = TEST_ENV_KEYS.map((key) => [key, process.env[key]] as const);
  process.env.OPENROUTER_BASE_URL = OPENROUTER_BASE_URL;
  process.env.OPENCODE_BASE_URL = OPENCODE_BASE_URL;
  process.env.OPENROUTER_API_KEY = 'test-openrouter-key';
  delete process.env.OPENCODE_API_KEY;
  delete process.env.ANTHROPIC_API_KEY;
  delete process.env.OPENAI_API_KEY;
  delete process.env.GOOGLE_API_KEY;
  delete process.env.NEVERMIND_KILL_SWITCHES;
  resetUpstreamTestCaches();
  return () => {
    for (const [key, value] of saved) {
      if (value === undefined) delete process.env[key];
      else process.env[key] = value;
    }
    globalThis.fetch = originalFetch;
    resetUpstreamTestCaches();
  };
}
