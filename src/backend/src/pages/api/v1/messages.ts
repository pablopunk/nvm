import type { APIRoute } from 'astro';
import { completeStreamLines, parseStreamUsageJson, proxyAndBill, type StreamUsageAccumulator, type UsageTokens } from '../../../lib/proxy';

export const config = { maxDuration: 300 };

function rewriteAnthropicModel(
  bodyText: string,
  routing: { activeModelId: string },
  maxOutputTokens: number,
): string {
  if (!bodyText) return bodyText;
  const parsed = JSON.parse(bodyText);
  parsed.model = routing.activeModelId;
  if (parsed.max_tokens !== undefined) parsed.max_tokens = maxOutputTokens;
  return JSON.stringify(parsed);
}

function parseUsageFromAnthropicJson(json: any): UsageTokens | null {
  const u = json?.usage;
  if (!u) return null;
  return {
    inputTokens: (u.input_tokens ?? 0) + (u.cache_read_input_tokens ?? 0) + (u.cache_creation_input_tokens ?? 0),
    outputTokens: u.output_tokens ?? 0,
    cachedInputTokens: u.cache_read_input_tokens ?? 0,
    cacheWriteInputTokens: u.cache_creation_input_tokens ?? 0,
  };
}

function parseUsageFromAnthropicStreamChunk(chunkText: string, acc: StreamUsageAccumulator, finalize = false): void {
  for (const line of completeStreamLines(chunkText, acc, finalize)) {
    const trimmed = line.trim();
    if (!trimmed.startsWith('data:')) continue;
    const payload = trimmed.slice(5).trim();
    if (!payload) continue;
    const obj = parseStreamUsageJson(payload, acc);
    const text = obj?.delta?.text;
    if (typeof text === 'string') acc.observedOutputCharacters = (acc.observedOutputCharacters ?? 0) + text.length;
    const partialJson = obj?.delta?.partial_json;
    if (typeof partialJson === 'string') acc.observedOutputCharacters = (acc.observedOutputCharacters ?? 0) + partialJson.length;
    if (obj?.type === 'message_start' && obj?.message?.usage) {
      const usage = obj.message.usage;
      acc.cachedInputTokens = usage.cache_read_input_tokens ?? acc.cachedInputTokens;
      acc.cacheWriteInputTokens = usage.cache_creation_input_tokens ?? acc.cacheWriteInputTokens;
      acc.inputTokens = (usage.input_tokens ?? 0) + (acc.cachedInputTokens ?? 0) + (acc.cacheWriteInputTokens ?? 0);
      acc.outputTokens = obj.message.usage.output_tokens ?? acc.outputTokens;
    } else if (obj?.type === 'message_delta' && obj?.usage) {
      if (typeof obj.usage.input_tokens === 'number') acc.inputTokens = obj.usage.input_tokens;
      if (typeof obj.usage.output_tokens === 'number') acc.outputTokens = obj.usage.output_tokens;
      acc.finalized = true;
    }
  }
}

export const POST: APIRoute = ({ request }) =>
  proxyAndBill({
    request,
    authHeaderName: 'x-api-key',
    upstreamAuthHeaderName: 'x-api-key',
    formatUpstreamAuthValue: (k) => k,
    buildUpstreamUrl: ({ upstreamBaseUrl }) => `${upstreamBaseUrl}/messages`,
    rewriteRequestBody: rewriteAnthropicModel,
    parseUsageFromJson: parseUsageFromAnthropicJson,
    parseUsageFromStreamChunk: parseUsageFromAnthropicStreamChunk,
  });
