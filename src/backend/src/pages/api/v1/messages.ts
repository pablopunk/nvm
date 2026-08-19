import type { APIRoute } from 'astro';
import { completeStreamLines, parseStreamUsageJson, proxyAndBill, type StreamUsageAccumulator, type UsageTokens } from '../../../lib/proxy';

export const config = { maxDuration: 300 };

function rewriteAnthropicModel(bodyText: string, activeModelId: string): string {
  if (!bodyText) return bodyText;
  const parsed = JSON.parse(bodyText);
  parsed.model = activeModelId;
  return JSON.stringify(parsed);
}

function parseUsageFromAnthropicJson(json: any): UsageTokens | null {
  const u = json?.usage;
  if (!u) return null;
  return { inputTokens: u.input_tokens ?? 0, outputTokens: u.output_tokens ?? 0 };
}

function parseUsageFromAnthropicStreamChunk(chunkText: string, acc: StreamUsageAccumulator, finalize = false): void {
  for (const line of completeStreamLines(chunkText, acc, finalize)) {
    const trimmed = line.trim();
    if (!trimmed.startsWith('data:')) continue;
    const payload = trimmed.slice(5).trim();
    if (!payload) continue;
    const obj = parseStreamUsageJson(payload, acc);
    if (obj?.type === 'message_start' && obj?.message?.usage) {
      acc.inputTokens = obj.message.usage.input_tokens ?? acc.inputTokens;
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
