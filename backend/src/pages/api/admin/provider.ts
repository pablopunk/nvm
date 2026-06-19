import type { APIRoute } from 'astro';
import { requireAdmin } from '../../../lib/admin';
import { requireSameOrigin } from '../../../lib/csrf';
import { listAllProviders, updateProvider } from '../../../lib/settings';
import { recordAudit } from '../../../lib/audit';

export const GET: APIRoute = async ({ request }) => {
  if (!(await requireAdmin(request))) return new Response('Forbidden', { status: 403 });
  const all = await listAllProviders();
  return Response.json({ providers: all });
};

export const PUT: APIRoute = async ({ request }) => {
  const originCheck = requireSameOrigin(request);
  if (originCheck) return originCheck;

  const actor = await requireAdmin(request);
  if (!actor) return new Response('Forbidden', { status: 403 });
  const body = (await request.json().catch(() => ({}))) as {
    id?: string;
    enabled?: boolean;
    priority?: number;
  };
  if (!body.id) return new Response('Missing id', { status: 400 });

  await updateProvider(body.id, {
    enabled: body.enabled,
    priority: body.priority,
  });

  await recordAudit({
    actorUserId: actor.id,
    action: 'provider.changed',
    targetType: 'provider',
    targetId: body.id,
    meta: { enabled: body.enabled, priority: body.priority },
  });

  return Response.json({ ok: true });
};
