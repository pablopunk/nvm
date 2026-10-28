import type { APIRoute } from 'astro';
import { completeStreamLines, parseStreamUsageJson, proxyAndBill, type StreamUsageAccumulator, type UsageTokens } from '../../../../lib/proxy';

export const config = { maxDuration: 300 };

const SUPPORTED_GOOGLE_OPERATION_PATH = /^[^:]*:(generateContent|streamGenerateContent|countTokens)$/;
const FORWARDED_GOOGLE_ALT_VALUES = new Set(['sse', 'json']);

function isSupportedGoogleOperationPath(path: string): boolean {
  return SUPPORTED_GOOGLE_OPERATION_PATH.test(path);
}

function googleOperationFromPath(supportedPath: string): string {
  return supportedPath.slice(supportedPath.indexOf(':') + 1);
}

function forwardedGoogleSearch(url: URL): string {
  const alt = url.searchParams.get('alt');
  return alt !== null && FORWARDED_GOOGLE_ALT_VALUES.has(alt) ? `?alt=${alt}` : '';
}

function googleUpstreamUrl(upstreamBaseUrl: string, activeModelId: string, operation: string, search: string): string {
  const expectedPathname = new URL(`${upstreamBaseUrl}/models/${activeModelId}:${operation}`).pathname;
  const builtUrl = `${upstreamBaseUrl}/models/${activeModelId}:${operation}${search}`;
  if (new URL(builtUrl).pathname !== expectedPathname) {
    throw new Error('Google upstream URL path does not match the requested model operation');
  }
  return builtUrl;
}

function unsupportedGoogleOperationResponse(): Response {
  return Response.json(
    { error: { type: 'unsupported_request_shape', message: 'Unsupported Google model operation.' } },
    { status: 400 },
  );
}

function parseUsageFromGoogleJson(json: any): UsageTokens | null {
  const u = json?.usageMetadata;
  if (!u) return null;
  return {
    inputTokens: u.promptTokenCount ?? 0,
    outputTokens: googleOutputTokens(u),
    cachedInputTokens: u.cachedContentTokenCount ?? 0,
    reasoningTokens: u.thoughtsTokenCount ?? 0,
  };
}

function googleOutputTokens(usageMetadata: any): number {
  return (usageMetadata.candidatesTokenCount ?? 0) + (usageMetadata.thoughtsTokenCount ?? 0);
}

function parseUsageFromGoogleStreamChunk(chunkText: string, acc: StreamUsageAccumulator, finalize = false): void {
  for (const line of completeStreamLines(chunkText, acc, finalize)) {
    const trimmed = line.trim();
    if (!trimmed.startsWith('data:')) continue;
    const payload = trimmed.slice(5).trim();
    if (!payload) continue;
    const obj = parseStreamUsageJson(payload, acc);
    const text = obj?.candidates?.[0]?.content?.parts?.map((part: any) => part?.text ?? '').join('');
    if (typeof text === 'string') acc.observedOutputCharacters = (acc.observedOutputCharacters ?? 0) + text.length;
    for (const part of obj?.candidates?.[0]?.content?.parts ?? []) {
      if (part?.functionCall) acc.observedOutputCharacters = (acc.observedOutputCharacters ?? 0) + JSON.stringify(part.functionCall).length;
    }
    const usage = obj?.usageMetadata;
    if (!usage) continue;
    if (typeof usage.promptTokenCount === 'number') {
      acc.inputTokens = usage.promptTokenCount;
    }
    if (typeof usage.cachedContentTokenCount === 'number') acc.cachedInputTokens = usage.cachedContentTokenCount;
    if (typeof usage.thoughtsTokenCount === 'number') acc.reasoningTokens = usage.thoughtsTokenCount;
    const outputTokens = googleOutputTokens(usage);
    if (outputTokens > 0) {
      acc.outputTokens = outputTokens;
      acc.finalized = true;
    }
  }
}

export const POST: APIRoute = async ({ request, params, url }) => {
  const rawPath = (params.path ?? '').toString();
  if (!isSupportedGoogleOperationPath(rawPath)) return unsupportedGoogleOperationResponse();
  const operation = googleOperationFromPath(rawPath);
  const search = forwardedGoogleSearch(url);
  return proxyAndBill({
    request,
    requiredApi: 'google-generative-ai',
    authHeaderName: 'x-goog-api-key',
    upstreamAuthHeaderName: 'x-goog-api-key',
    formatUpstreamAuthValue: (k) => k,
    buildUpstreamUrl: ({ upstreamBaseUrl, activeModelId }) =>
      googleUpstreamUrl(upstreamBaseUrl, activeModelId, operation, search),
    parseUsageFromJson: parseUsageFromGoogleJson,
    parseUsageFromStreamChunk: parseUsageFromGoogleStreamChunk,
  });
};
