import type { APIRoute } from 'astro';
import { completeStreamLines, parseStreamUsageJson, proxyAndBill, type StreamUsageAccumulator, type UsageTokens } from '../../../../lib/proxy';

export const config = { maxDuration: 300 };

function rewriteOpenAiModel(bodyText: string, activeModelId: string): string {
  if (!bodyText) return bodyText;
  const parsed = JSON.parse(bodyText);
  parsed.model = activeModelId;
  return JSON.stringify(parsed);
}

function parseUsageFromOpenAiJson(json: any): UsageTokens | null {
  const u = json?.usage;
  if (!u) return null;
  return { inputTokens: u.prompt_tokens ?? 0, outputTokens: u.completion_tokens ?? 0 };
}

function parseUsageFromOpenAiStreamChunk(chunkText: string, acc: StreamUsageAccumulator, finalize = false): void {
  for (const line of completeStreamLines(chunkText, acc, finalize)) {
    const trimmed = line.trim();
    if (!trimmed.startsWith('data:')) continue;
    const payload = trimmed.slice(5).trim();
    if (!payload || payload === '[DONE]') continue;
    const obj = parseStreamUsageJson(payload, acc);
    if (obj?.usage) {
      acc.inputTokens = obj.usage.prompt_tokens ?? acc.inputTokens;
      acc.outputTokens = obj.usage.completion_tokens ?? acc.outputTokens;
      acc.finalized = true;
    }
  }
}

export const POST: APIRoute = ({ request }) =>
  proxyAndBill({
    request,
    authHeaderName: 'authorization',
    upstreamAuthHeaderName: 'authorization',
    formatUpstreamAuthValue: (k) => `Bearer ${k}`,
    buildUpstreamUrl: ({ upstreamBaseUrl }) => `${upstreamBaseUrl}/chat/completions`,
    rewriteRequestBody: rewriteOpenAiModel,
    parseUsageFromJson: parseUsageFromOpenAiJson,
    parseUsageFromStreamChunk: parseUsageFromOpenAiStreamChunk,
    idempotencyKey: request.headers.get('Idempotency-Key') ?? undefined,
  });
