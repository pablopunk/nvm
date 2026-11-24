import type { APIRoute } from 'astro';
import { isAuthorizedCron } from '../../../lib/cron-auth';
import { captureBackendException } from '../../../lib/error-tracking';
import { log } from '../../../lib/log';
import { refreshModelCapabilities } from '../../../lib/model-capabilities/refresh';

export const config = { maxDuration: 300 };

const PROBE_BUDGET_MS = 240_000;

export const GET: APIRoute = async ({ request }) => {
  if (!isAuthorizedCron(request)) return new Response('Unauthorized', { status: 401 });
  try {
    const report = await refreshModelCapabilities(Date.now() + PROBE_BUDGET_MS);
    log.info('model_capability_refresh', report);
    return Response.json({ ok: true, ...report });
  } catch (error) {
    log.error('model_capability_refresh_crashed', { error: String(error) });
    captureBackendException(error, { kind: 'model_capability_refresh' });
    return Response.json({ ok: false, error: 'model_capability_refresh_crashed' }, { status: 503 });
  }
};
