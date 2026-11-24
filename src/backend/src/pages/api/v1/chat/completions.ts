import type { APIRoute } from 'astro';
import {
  completeStreamLines,
  parseStreamUsageJson,
  proxyAndBill,
  type StreamUsageAccumulator,
  type UsageTokens,
} from '../../../../lib/proxy';
import { rewriteChatCompletionsBody } from '../../../../lib/chat-completions-request';

export const config = { maxDuration: 300 };

function parseUsageFromOpenAiJson(json: any): UsageTokens | null {
  const u = json?.usage;
  if (!u) return null;
  return {
    inputTokens: u.prompt_tokens ?? 0,
    outputTokens: u.completion_tokens ?? 0,
    cachedInputTokens: u.prompt_tokens_details?.cached_tokens ?? 0,
    cacheWriteInputTokens: u.prompt_tokens_details?.cache_write_tokens ?? 0,
    reasoningTokens: u.completion_tokens_details?.reasoning_tokens ?? 0,
    providerCostUsd: typeof u.cost === 'number' ? u.cost : undefined,
  };
}

function parseUsageFromOpenAiStreamChunk(
  chunkText: string,
  acc: StreamUsageAccumulator,
  finalize = false,
): void {
  for (const line of completeStreamLines(chunkText, acc, finalize)) {
    const trimmed = line.trim();
    if (!trimmed.startsWith('data:')) continue;
    const payload = trimmed.slice(5).trim();
    if (!payload || payload === '[DONE]') continue;
    const obj = parseStreamUsageJson(payload, acc);
    for (const choice of obj?.choices ?? []) {
      const content = choice?.delta?.content;
      if (typeof content === 'string')
        acc.observedOutputCharacters =
          (acc.observedOutputCharacters ?? 0) + content.length;
      const reasoning = choice?.delta?.reasoning;
      if (typeof reasoning === 'string')
        acc.observedOutputCharacters =
          (acc.observedOutputCharacters ?? 0) + reasoning.length;
      for (const toolCall of choice?.delta?.tool_calls ?? []) {
        const argumentsText = toolCall?.function?.arguments;
        if (typeof argumentsText === 'string')
          acc.observedOutputCharacters =
            (acc.observedOutputCharacters ?? 0) + argumentsText.length;
      }
    }
    if (obj?.usage) {
      acc.inputTokens = obj.usage.prompt_tokens ?? acc.inputTokens;
      acc.outputTokens = obj.usage.completion_tokens ?? acc.outputTokens;
      acc.cachedInputTokens =
        obj.usage.prompt_tokens_details?.cached_tokens ?? acc.cachedInputTokens;
      acc.cacheWriteInputTokens =
        obj.usage.prompt_tokens_details?.cache_write_tokens ??
        acc.cacheWriteInputTokens;
      acc.reasoningTokens =
        obj.usage.completion_tokens_details?.reasoning_tokens ??
        acc.reasoningTokens;
      if (typeof obj.usage.cost === 'number')
        acc.providerCostUsd = obj.usage.cost;
      acc.finalized = true;
    }
  }
}

export const POST: APIRoute = ({ request }) =>
  proxyAndBill({
    request,
    wireApi: 'openai-completions',
    authHeaderName: 'authorization',
    upstreamAuthHeaderName: 'authorization',
    formatUpstreamAuthValue: (k) => `Bearer ${k}`,
    buildUpstreamUrl: ({ upstreamBaseUrl }) =>
      `${upstreamBaseUrl}/chat/completions`,
    rewriteRequestBody: rewriteChatCompletionsBody,
    parseUsageFromJson: parseUsageFromOpenAiJson,
    parseUsageFromStreamChunk: parseUsageFromOpenAiStreamChunk,
    idempotencyKey: request.headers.get('Idempotency-Key') ?? undefined,
  });
