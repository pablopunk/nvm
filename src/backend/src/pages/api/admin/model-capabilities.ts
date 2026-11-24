import type { APIRoute } from 'astro';
import { requireAdmin } from '../../../lib/admin';
import { requireSameOrigin } from '../../../lib/csrf';
import { log } from '../../../lib/log';
import { refreshModelCapabilities } from '../../../lib/model-capabilities/refresh';

export const config = { maxDuration: 60 };

const ADMIN_PROBE_BUDGET_MS = 45_000;

export const POST: APIRoute = async ({ request }) => {
  const originCheck = requireSameOrigin(request);
  if (originCheck) return originCheck;
  if (!(await requireAdmin(request))) return new Response('Forbidden', { status: 403 });
  const report = await refreshModelCapabilities(Date.now() + ADMIN_PROBE_BUDGET_MS);
  log.info('model_capability_refresh', { trigger: 'admin', ...report });
  return Response.json(report);
};
