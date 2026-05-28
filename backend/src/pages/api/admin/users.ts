import type { APIRoute } from 'astro';
import { sql } from 'drizzle-orm';
import { requireAdmin } from '../../../lib/admin';
import { db } from '../../../db/client';
import { users, usage, creditLedger } from '../../../db/schema';

export const GET: APIRoute = async ({ request }) => {
  if (!(await requireAdmin(request))) return new Response('Forbidden', { status: 403 });

  const rows = await db
    .select({
      id: users.id,
      email: users.email,
      plan: users.plan,
      role: users.role,
      createdAt: users.createdAt,
      balance: sql<number>`coalesce((select sum(${creditLedger.delta}) from ${creditLedger} where ${creditLedger.userId} = ${users.id}), 0)::int`,
      requests: sql<number>`coalesce((select count(*) from ${usage} where ${usage.userId} = ${users.id}), 0)::int`,
      spent: sql<number>`coalesce((select sum(${usage.costCredits}) from ${usage} where ${usage.userId} = ${users.id}), 0)::int`,
      lastModel: sql<string | null>`(select ${usage.model} from ${usage} where ${usage.userId} = ${users.id} order by ${usage.createdAt} desc limit 1)`,
      lastUsedAt: sql<string | null>`(select ${usage.createdAt} from ${usage} where ${usage.userId} = ${users.id} order by ${usage.createdAt} desc limit 1)`,
    })
    .from(users)
    .orderBy(sql`${users.createdAt} desc`);

  return Response.json({ users: rows });
};
