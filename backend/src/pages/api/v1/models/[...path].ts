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
  return { inputTokens: u.promptTokenCount ?? 0, outputTokens: googleOutputTokens(u) };
}

function googleOutputTokens(usageMetadata: any): number {
  return (usageMetadata.candidatesTokenCount ?? 0) + (usageMetadata.thoughtsTokenCount ?? 0);
}

function completeStreamLines(chunkText: string, acc: StreamUsageAccumulator): string[] {
  const text = `${acc.pendingText ?? ''}${chunkText}`;
  const lines = text.split(/\r?\n/);
  acc.pendingText = lines.pop() ?? '';
  return lines;
}

function parseUsageFromGoogleStreamChunk(chunkText: string, acc: StreamUsageAccumulator): void {
  for (const line of completeStreamLines(chunkText, acc)) {
    const trimmed = line.trim();
    if (!trimmed.startsWith('data:')) continue;
    const payload = trimmed.slice(5).trim();
    if (!payload) continue;
    try {
      const obj = JSON.parse(payload);
      const usage = obj?.usageMetadata;
      if (!usage) continue;
      if (typeof usage.promptTokenCount === 'number') {
        acc.inputTokens = usage.promptTokenCount;
      }
      const outputTokens = googleOutputTokens(usage);
      if (outputTokens > 0) {
        acc.outputTokens = outputTokens;
        acc.finalized = true;
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
