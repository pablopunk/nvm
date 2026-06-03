import type { APIRoute } from 'astro';
import { proxyAndBill, type StreamUsageAccumulator, type UsageTokens } from '../../../../lib/proxy';

export const config = { maxDuration: 300 };

function rewriteGoogleModelInPath(originalPath: string, activeModelId: string): string {
  const colonIdx = originalPath.indexOf(':');
  const operation = colonIdx >= 0 ? originalPath.slice(colonIdx) : '';
  return `${activeModelId}${operation}`;
}

function parseUsageFromGoogleJson(json: any): UsageTokens | null {
  const u = json?.usageMetadata;
  if (!u) return null;
  return { inputTokens: u.promptTokenCount ?? 0, outputTokens: u.candidatesTokenCount ?? 0 };
}

function parseUsageFromGoogleStreamChunk(chunkText: string, acc: StreamUsageAccumulator): void {
  for (const line of chunkText.split('\n')) {
    const trimmed = line.trim();
    if (!trimmed.startsWith('data:')) continue;
    const payload = trimmed.slice(5).trim();
    if (!payload) continue;
    try {
      const obj = JSON.parse(payload);
      if (obj?.usageMetadata) {
        if (typeof obj.usageMetadata.promptTokenCount === 'number') {
          acc.inputTokens = obj.usageMetadata.promptTokenCount;
        }
        if (typeof obj.usageMetadata.candidatesTokenCount === 'number') {
          acc.outputTokens = obj.usageMetadata.candidatesTokenCount;
          acc.finalized = true;
        }
      }
    } catch {}
  }
}

export const POST: APIRoute = async ({ request, params, url }) => {
  const rawPath = (params.path ?? '').toString();
  const search = url.search ?? '';
  return proxyAndBill({
    request,
    authHeaderName: 'x-goog-api-key',
    upstreamAuthHeaderName: 'x-goog-api-key',
    formatUpstreamAuthValue: (k) => k,
    buildUpstreamUrl: ({ upstreamBaseUrl, activeModelId }) => {
      const rewritten = rewriteGoogleModelInPath(rawPath, activeModelId);
      return `${upstreamBaseUrl}/models/${rewritten}${search}`;
    },
    parseUsageFromJson: parseUsageFromGoogleJson,
    parseUsageFromStreamChunk: parseUsageFromGoogleStreamChunk,
  });
};
