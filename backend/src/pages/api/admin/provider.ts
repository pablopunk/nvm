import type { APIRoute } from 'astro';
import { requireAdmin } from '../../../lib/admin';
import { getActiveProvider, setActiveProvider, listKnownProviders } from '../../../lib/settings';
import { recordAudit } from '../../../lib/audit';

export const GET: APIRoute = async ({ request }) => {
  if (!(await requireAdmin(request))) return new Response('Forbidden', { status: 403 });
  return Response.json({ active: await getActiveProvider(), providers: listKnownProviders() });
};

export const PUT: APIRoute = async ({ request }) => {
  const actor = await requireAdmin(request);
  if (!actor) return new Response('Forbidden', { status: 403 });
  const body = (await request.json().catch(() => ({}))) as { provider?: string };
  if (!body.provider) return new Response('Missing provider', { status: 400 });
  try {
    await setActiveProvider(body.provider);
  } catch (err) {
    return new Response((err as Error).message, { status: 400 });
  }
  await recordAudit({
    actorUserId: actor.id,
    action: 'provider.changed',
    targetType: 'provider',
    targetId: body.provider,
  });
  return Response.json({ ok: true });
};
