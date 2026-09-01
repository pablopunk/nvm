import type { APIRoute } from 'astro';
import { completeStreamLines, parseStreamUsageJson, proxyAndBill, type StreamUsageAccumulator, type UsageTokens } from '../../../../lib/proxy';

export const config = { maxDuration: 300 };

function rewriteOpenAiModel(
  bodyText: string,
  routing: {
    activeModelId: string;
    provider: string;
    thinkingLevel: string;
  },
  maxOutputTokens: number,
): string {
  if (!bodyText) return bodyText;
  const parsed = JSON.parse(bodyText);
  parsed.model = routing.activeModelId;
  if (parsed.max_tokens !== undefined) parsed.max_tokens = maxOutputTokens;
  if (parsed.max_completion_tokens !== undefined)
    parsed.max_completion_tokens = maxOutputTokens;
  if (routing.provider === 'openrouter') {
    delete parsed.models;
    delete parsed.route;
    delete parsed.provider;
    delete parsed.plugins;
    delete parsed.web_search_options;
    delete parsed.reasoning_effort;
    parsed.reasoning = {
      effort: routing.thinkingLevel === 'off' ? 'none' : routing.thinkingLevel,
    };
    if (
      parsed.max_tokens === undefined &&
      parsed.max_completion_tokens !== undefined
    ) {
      parsed.max_tokens = parsed.max_completion_tokens;
    }
    delete parsed.max_completion_tokens;
  }
  if (parsed.stream === true) {
    parsed.stream_options = { ...(parsed.stream_options ?? {}), include_usage: true };
  }
  return JSON.stringify(parsed);
}

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

function parseUsageFromOpenAiStreamChunk(chunkText: string, acc: StreamUsageAccumulator, finalize = false): void {
  for (const line of completeStreamLines(chunkText, acc, finalize)) {
    const trimmed = line.trim();
    if (!trimmed.startsWith('data:')) continue;
    const payload = trimmed.slice(5).trim();
    if (!payload || payload === '[DONE]') continue;
    const obj = parseStreamUsageJson(payload, acc);
    const content = obj?.choices?.[0]?.delta?.content;
    if (typeof content === 'string') acc.observedOutputCharacters = (acc.observedOutputCharacters ?? 0) + content.length;
    const reasoning = obj?.choices?.[0]?.delta?.reasoning;
    if (typeof reasoning === 'string') acc.observedOutputCharacters = (acc.observedOutputCharacters ?? 0) + reasoning.length;
    for (const toolCall of obj?.choices?.[0]?.delta?.tool_calls ?? []) {
      const argumentsText = toolCall?.function?.arguments;
      if (typeof argumentsText === 'string') acc.observedOutputCharacters = (acc.observedOutputCharacters ?? 0) + argumentsText.length;
    }
    if (obj?.usage) {
      acc.inputTokens = obj.usage.prompt_tokens ?? acc.inputTokens;
      acc.outputTokens = obj.usage.completion_tokens ?? acc.outputTokens;
      acc.cachedInputTokens = obj.usage.prompt_tokens_details?.cached_tokens ?? acc.cachedInputTokens;
      acc.cacheWriteInputTokens = obj.usage.prompt_tokens_details?.cache_write_tokens ?? acc.cacheWriteInputTokens;
      acc.reasoningTokens = obj.usage.completion_tokens_details?.reasoning_tokens ?? acc.reasoningTokens;
      if (typeof obj.usage.cost === 'number') acc.providerCostUsd = obj.usage.cost;
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
