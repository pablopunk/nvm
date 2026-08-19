import type { APIRoute } from 'astro';
import { sql } from 'drizzle-orm';
import { requireAdmin } from '../../../lib/admin';
import { db } from '../../../db/client';

export const GET: APIRoute = async ({ request, url }) => {
  if (!(await requireAdmin(request))) return new Response('Forbidden', { status: 403 });
  const days = Math.min(90, Math.max(1, Number(url.searchParams.get('days') ?? 30)));

  const rows = await db.execute<{
    day: string;
    requests: string;
    errors: string;
    credits: string;
    p50: string | null;
    p95: string | null;
  }>(sql`
    select
      to_char(date_trunc('day', created_at), 'YYYY-MM-DD') as day,
      count(*)::text as requests,
      sum(case when status is null or status >= 400 then 1 else 0 end)::text as errors,
      sum(cost_credits)::text as credits,
      percentile_cont(0.5) within group (order by latency_ms) filter (where latency_ms is not null)::text as p50,
      percentile_cont(0.95) within group (order by latency_ms) filter (where latency_ms is not null)::text as p95
    from usage
    where created_at >= now() - (${days}::int * interval '1 day')
    group by day
    order by day asc
  `);

  const points = (rows as any).rows ?? rows;
  const series = (points as any[]).map((r) => ({
    day: r.day,
    requests: Number(r.requests ?? 0),
    errors: Number(r.errors ?? 0),
    credits: Number(r.credits ?? 0),
    p50: r.p50 != null ? Number(r.p50) : null,
    p95: r.p95 != null ? Number(r.p95) : null,
  }));
  return Response.json({ days, series });
};
