import type { APIRoute } from 'astro';
import { sql } from 'drizzle-orm';
import { requireAdmin } from '../../../lib/admin';
import { db } from '../../../db/client';
import { users, usage, creditLedger } from '../../../db/schema';

export const GET: APIRoute = async ({ request }) => {
  if (!(await requireAdmin(request))) return new Response('Forbidden', { status: 403 });

  const [userCount] = await db.select({ n: sql<number>`count(*)::int` }).from(users);
  const [usageTotals] = await db
    .select({
      requests: sql<number>`count(*)::int`,
      inputTokens: sql<number>`coalesce(sum(${usage.inputTokens}),0)::int`,
      outputTokens: sql<number>`coalesce(sum(${usage.outputTokens}),0)::int`,
      costCredits: sql<number>`coalesce(sum(${usage.costCredits}),0)::int`,
    })
    .from(usage);
  const [credit] = await db
    .select({
      granted: sql<number>`coalesce(sum(case when ${creditLedger.delta} > 0 then ${creditLedger.delta} else 0 end),0)::int`,
      spent: sql<number>`coalesce(sum(case when ${creditLedger.delta} < 0 then -${creditLedger.delta} else 0 end),0)::int`,
    })
    .from(creditLedger);

  const byModel = await db
    .select({
      model: usage.model,
      requests: sql<number>`count(*)::int`,
      inputTokens: sql<number>`sum(${usage.inputTokens})::int`,
      outputTokens: sql<number>`sum(${usage.outputTokens})::int`,
      costCredits: sql<number>`sum(${usage.costCredits})::int`,
    })
    .from(usage)
    .groupBy(usage.model)
    .orderBy(sql`count(*) desc`);

  return Response.json({
    users: userCount.n,
    usage: usageTotals,
    credits: credit,
    byModel,
  });
};
