import type { APIRoute } from 'astro';
import { requireAdmin } from '../../../../lib/admin';
import { getUpstreamConfig, UpstreamConfigError } from '../../../../lib/upstream';

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
