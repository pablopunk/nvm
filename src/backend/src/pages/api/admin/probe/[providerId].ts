import type { APIRoute } from 'astro';
import { requireAdmin } from '../../../../lib/admin';
import { getUpstreamConfig, selectApiForModel, UpstreamConfigError } from '../../../../lib/upstream';

export function modelProbeRequest(providerId: string, modelId: string, baseUrl: string, apiKey: string) {
  const api = selectApiForModel(providerId, modelId);
  const headers = new Headers({ 'content-type': 'application/json' });
  if (api === 'anthropic-messages') {
    headers.set('x-api-key', apiKey);
    headers.set('anthropic-version', '2023-06-01');
    return {
      url: `${baseUrl}${providerId === 'anthropic' ? '/v1' : ''}/messages`,
      headers,
      body: { model: modelId, max_tokens: 1, stream: true, messages: [{ role: 'user', content: 'Reply OK' }] },
    };
  }
  if (api === 'google-generative-ai') {
    headers.set('x-goog-api-key', apiKey);
    return {
      url: `${baseUrl}/models/${encodeURIComponent(modelId)}:streamGenerateContent?alt=sse`,
      headers,
      body: { contents: [{ role: 'user', parts: [{ text: 'Reply OK' }] }], generationConfig: { maxOutputTokens: 1 } },
    };
  }
  headers.set('authorization', `Bearer ${apiKey}`);
  return {
    url: `${baseUrl}/chat/completions`,
    headers,
    body: { model: modelId, max_tokens: 1, stream: true, messages: [{ role: 'user', content: 'Reply OK' }] },
  };
}

async function probeModel(providerId: string, modelId: string, baseUrl: string, apiKey: string) {
  const probe = modelProbeRequest(providerId, modelId, baseUrl, apiKey);
  const startedAt = Date.now();
  const response = await fetch(probe.url, {
    method: 'POST',
    headers: probe.headers,
    body: JSON.stringify(probe.body),
    signal: AbortSignal.timeout(15_000),
  });
  if (!response.ok) {
    await response.body?.cancel().catch(() => undefined);
    return { ok: false, status: response.status, latencyMs: Date.now() - startedAt, error: `Provider returned HTTP ${response.status}` };
  }
  const reader = response.body?.getReader();
  if (reader) {
    await reader.read();
    await reader.cancel().catch(() => undefined);
  }
  return { ok: true, status: response.status, latencyMs: Date.now() - startedAt, modelId };
}

export const POST: APIRoute = async ({ request, params }) => {
  if (!(await requireAdmin(request))) return new Response('Forbidden', { status: 403 });
  const providerId = params.providerId;
  if (!providerId) return new Response('Missing providerId', { status: 400 });
  const input = await request.json().catch(() => null) as { modelId?: unknown } | null;
  const modelId = typeof input?.modelId === 'string' ? input.modelId.trim() : '';
  if (!modelId) return new Response('Missing modelId', { status: 400 });
  try {
    const { baseUrl, apiKey } = getUpstreamConfig(providerId);
    return Response.json(await probeModel(providerId, modelId, baseUrl, apiKey));
  } catch (err) {
    return Response.json({
      ok: false,
      error: err instanceof Error ? err.message : String(err),
    });
  }
};

export const GET: APIRoute = async ({ request, params }) => {
  if (!(await requireAdmin(request))) return new Response('Forbidden', { status: 403 });
  const providerId = params.providerId;
  if (!providerId) return new Response('Missing providerId', { status: 400 });

  let baseUrl: string;
  let apiKey: string;
  try {
    const cfg = getUpstreamConfig(providerId);
    baseUrl = cfg.baseUrl;
    apiKey = cfg.apiKey;
  } catch (err) {
    if (err instanceof UpstreamConfigError) {
      return Response.json({ ok: false, error: err.message }, { status: 200 });
    }
    throw err;
  }

  const probeUrl = providerId === 'anthropic'
    ? `${baseUrl}/v1/models?limit=1`
    : providerId === 'google'
      ? `${baseUrl}/models?pageSize=1`
      : `${baseUrl}/models`;
  const headers = new Headers();
  if (providerId === 'anthropic') {
    headers.set('x-api-key', apiKey);
    headers.set('anthropic-version', '2023-06-01');
  } else if (providerId === 'google') {
    headers.set('x-goog-api-key', apiKey);
  } else {
    headers.set('authorization', `Bearer ${apiKey}`);
  }
  const startedAt = Date.now();
  try {
    const resp = await fetch(probeUrl, {
      method: 'GET',
      headers,
      signal: AbortSignal.timeout(8000),
    });
    const latencyMs = Date.now() - startedAt;
    return Response.json({
      ok: resp.ok,
      status: resp.status,
      latencyMs,
      ...(resp.ok ? {} : { error: `Provider returned HTTP ${resp.status}` }),
    });
  } catch (err) {
    const latencyMs = Date.now() - startedAt;
    return Response.json({
      ok: false,
      error: err instanceof Error ? err.message : String(err),
      latencyMs,
    });
  }
};
