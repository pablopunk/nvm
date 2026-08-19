import type { APIRoute } from 'astro';
import { sql } from 'drizzle-orm';
import { requireAdmin } from '../../../lib/admin';
import { db } from '../../../db/client';

export const GET: APIRoute = async ({ request }) => {
  if (!(await requireAdmin(request))) return new Response('Forbidden', { status: 403 });

  const { rows } = await db.execute<{
    id: string;
    email: string;
    plan: string;
    role: string;
    created_at: string;
    balance: number;
    requests: number;
    spent: number;
    last_model: string | null;
    last_used_at: string | null;
  }>(sql`
    select
      u.id,
      u.email,
      u.plan,
      u.role,
      u.created_at,
      coalesce((select sum(delta) from credit_ledger where user_id = u.id), 0)::int as balance,
      coalesce((select count(*) from usage where user_id = u.id), 0)::int as requests,
      coalesce((select sum(cost_credits) from usage where user_id = u.id), 0)::int as spent,
      (select model from usage where user_id = u.id order by created_at desc limit 1) as last_model,
      (select created_at from usage where user_id = u.id order by created_at desc limit 1) as last_used_at
    from users u
    order by u.created_at desc
  `);

  return Response.json({
    users: rows.map((r) => ({
      id: r.id,
      email: r.email,
      plan: r.plan,
      role: r.role,
      createdAt: r.created_at,
      balance: r.balance,
      requests: r.requests,
      spent: r.spent,
      lastModel: r.last_model,
      lastUsedAt: r.last_used_at,
    })),
  });
};
