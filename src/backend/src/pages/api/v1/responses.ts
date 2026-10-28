import type { APIRoute } from 'astro';
import {
  completeStreamLines,
  parseStreamUsageJson,
  proxyAndBill,
  type StreamUsageAccumulator,
  type UsageTokens,
} from '../../../lib/proxy';
import {
  rewriteResponsesRequest,
  validateResponsesRequest,
} from '../../../lib/responses-request';

export const config = { maxDuration: 300 };

function parseResponsesUsage(json: any): UsageTokens | null {
  const usage = json?.usage;
  if (!usage) return null;
  return {
    inputTokens: usage.input_tokens ?? 0,
    outputTokens: usage.output_tokens ?? 0,
    cachedInputTokens: usage.input_tokens_details?.cached_tokens ?? 0,
    reasoningTokens: usage.output_tokens_details?.reasoning_tokens ?? 0,
    providerCostUsd: typeof usage.cost === 'number' ? usage.cost : undefined,
  };
}

function parseResponsesStream(
  chunk: string,
  acc: StreamUsageAccumulator,
  finalize = false,
): void {
  for (const line of completeStreamLines(chunk, acc, finalize)) {
    const trimmed = line.trim();
    if (!trimmed.startsWith('data:')) continue;
    const payload = trimmed.slice(5).trim();
    if (!payload || payload === '[DONE]') continue;
    const event = parseStreamUsageJson(payload, acc);
    if (
      [
        'response.output_text.delta',
        'response.function_call_arguments.delta',
        'response.reasoning_summary_text.delta',
      ].includes(event?.type) &&
      typeof event.delta === 'string'
    )
      acc.observedOutputCharacters =
        (acc.observedOutputCharacters ?? 0) + event.delta.length;
    const usage = parseResponsesUsage(event?.response);
    if (usage) {
      acc.inputTokens = usage.inputTokens;
      acc.outputTokens = usage.outputTokens;
      acc.cachedInputTokens = usage.cachedInputTokens;
      acc.reasoningTokens = usage.reasoningTokens;
      acc.providerCostUsd = usage.providerCostUsd;
      acc.finalized = true;
    }
  }
}

export const POST: APIRoute = ({ request }) =>
  proxyAndBill({
    request,
    wireApi: 'openai-responses',
    authHeaderName: 'authorization',
    upstreamAuthHeaderName: 'authorization',
    formatUpstreamAuthValue: (key) => `Bearer ${key}`,
    buildUpstreamUrl: ({ upstreamBaseUrl }) => `${upstreamBaseUrl}/responses`,
    validateRequestBody: validateResponsesRequest,
    rewriteRequestBody: rewriteResponsesRequest,
    parseUsageFromJson: parseResponsesUsage,
    parseUsageFromStreamChunk: parseResponsesStream,
    idempotencyKey: request.headers.get('Idempotency-Key') ?? undefined,
  });
