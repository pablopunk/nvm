import type { APIRoute } from 'astro';
import { desc, eq } from 'drizzle-orm';
import { requireAdmin } from '../../../lib/admin';
import { db } from '../../../db/client';
import { auditLog, users } from '../../../db/schema';

export const GET: APIRoute = async ({ request, url }) => {
  if (!(await requireAdmin(request))) return new Response('Forbidden', { status: 403 });
  const limit = Math.min(200, Math.max(1, Number(url.searchParams.get('limit') ?? 50)));
  const rows = await db
    .select({
      id: auditLog.id,
      action: auditLog.action,
      targetType: auditLog.targetType,
      targetId: auditLog.targetId,
      meta: auditLog.meta,
      createdAt: auditLog.createdAt,
      actorEmail: users.email,
    })
    .from(auditLog)
    .leftJoin(users, eq(users.id, auditLog.actorUserId))
    .orderBy(desc(auditLog.createdAt))
    .limit(limit);
  return Response.json({ entries: rows });
};
