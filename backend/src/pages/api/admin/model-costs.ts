import type { APIRoute } from 'astro';
import { and, eq, asc } from 'drizzle-orm';
import { requireAdmin } from '../../../lib/admin';
import { db } from '../../../db/client';
import { modelCosts } from '../../../db/schema';

export const GET: APIRoute = async ({ request }) => {
  if (!(await requireAdmin(request))) return new Response('Forbidden', { status: 403 });
  const rows = await db.select().from(modelCosts).orderBy(asc(modelCosts.provider), asc(modelCosts.modelId));
  return Response.json({ rows });
};

export const PUT: APIRoute = async ({ request }) => {
  if (!(await requireAdmin(request))) return new Response('Forbidden', { status: 403 });
  const body = (await request.json().catch(() => ({}))) as {
    provider?: string;
    modelId?: string;
    inputUsdPerMtok?: number;
    outputUsdPerMtok?: number;
  };
  if (!body.provider || !body.modelId) return new Response('Missing provider/modelId', { status: 400 });
  if (typeof body.inputUsdPerMtok !== 'number' || typeof body.outputUsdPerMtok !== 'number') {
    return new Response('Missing prices', { status: 400 });
  }
  await db
    .insert(modelCosts)
    .values({
      provider: body.provider,
      modelId: body.modelId,
      inputUsdPerMtok: String(body.inputUsdPerMtok),
      outputUsdPerMtok: String(body.outputUsdPerMtok),
    })
    .onConflictDoUpdate({
      target: [modelCosts.provider, modelCosts.modelId],
      set: {
        inputUsdPerMtok: String(body.inputUsdPerMtok),
        outputUsdPerMtok: String(body.outputUsdPerMtok),
      },
    });
  return Response.json({ ok: true });
};

export const DELETE: APIRoute = async ({ request }) => {
  if (!(await requireAdmin(request))) return new Response('Forbidden', { status: 403 });
  const url = new URL(request.url);
  const provider = url.searchParams.get('provider');
  const modelId = url.searchParams.get('modelId');
  if (!provider || !modelId) return new Response('Missing provider/modelId', { status: 400 });
  await db.delete(modelCosts).where(and(eq(modelCosts.provider, provider), eq(modelCosts.modelId, modelId)));
  return Response.json({ ok: true });
};
