import type { APIRoute } from 'astro';
import { captureBackendAlert } from '../../../lib/error-tracking';
import { isAuthorizedCron } from '../../../lib/cron-auth';
import { log } from '../../../lib/log';

export const GET: APIRoute = async ({ request, url }) => {
  if (!isAuthorizedCron(request)) return new Response('Forbidden', { status: 403 });
  const origin = `${url.protocol}//${url.host}`;
  const res = await fetch(`${origin}/api/health`).catch(() => null);
  const body = res ? await res.json().catch(() => null) : null;
  const ok = Boolean(res?.ok && body?.ok);
  log.info('health_check_ping', { ok, status: res?.status, body });
  if (!ok) {
    captureBackendAlert('health_check_failed', {
      kind: 'health_check',
      status: res?.status ?? 'no-response',
      body,
    });
  }
  return Response.json({ ok, status: res?.status, body });
};
