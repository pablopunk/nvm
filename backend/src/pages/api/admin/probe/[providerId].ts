import type { APIRoute } from 'astro';
import { requireAdmin } from '../../../lib/admin';
import { getUpstreamConfig, UpstreamConfigError } from '../../../lib/upstream';

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

  const probeUrl = `${baseUrl}/models`;
  const startedAt = Date.now();
  try {
    const resp = await fetch(probeUrl, {
      method: 'GET',
      headers: { authorization: `Bearer ${apiKey}` },
      signal: AbortSignal.timeout(8000),
    });
    const latencyMs = Date.now() - startedAt;
    const body = await resp.text().catch(() => '');
    return Response.json({
      ok: resp.ok || resp.status < 500,
      status: resp.status,
      latencyMs,
      body: body.slice(0, 500),
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
