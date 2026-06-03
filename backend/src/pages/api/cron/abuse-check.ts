import type { APIRoute } from 'astro';
import { sql } from 'drizzle-orm';
import * as Sentry from '@sentry/astro';
import { db } from '../../../db/client';
import { isAuthorizedCron } from '../../../lib/cron-auth';
import { log } from '../../../lib/log';

const SPIKE_MULTIPLIER = 5;
const MIN_DAILY_SPEND = 50;

type Row = {
  email: string;
  user_id: string;
  spend_24h: string;
  median_30d: string;
};

export const GET: APIRoute = async ({ request }) => {
  if (!isAuthorizedCron(request)) return new Response('Forbidden', { status: 403 });

  const result = await db.execute<Row>(sql`
    with windows as (
      select
        u.id as user_id,
        u.email,
        sum(case when l.created_at >= now() - interval '24 hours' then -l.delta else 0 end) as spend_24h,
        percentile_cont(0.5) within group (
          order by daily.spent
        ) as median_30d
      from users u
      join credit_ledger l on l.user_id = u.id
      left join lateral (
        select date_trunc('day', created_at) as day, sum(-delta) as spent
        from credit_ledger
        where user_id = u.id
          and reason = 'ai_usage'
          and created_at >= now() - interval '30 days'
        group by 1
      ) daily on true
      where l.reason = 'ai_usage'
      group by u.id, u.email
    )
    select user_id, email, spend_24h::text, coalesce(median_30d, 0)::text as median_30d
    from windows
    where spend_24h >= ${MIN_DAILY_SPEND}
      and (median_30d is null or spend_24h > median_30d * ${SPIKE_MULTIPLIER})
  `);

  const rows = ((result as any).rows ?? result) as Row[];
  const flagged = rows.map((r) => ({
    email: r.email,
    userId: r.user_id,
    spend24h: Number(r.spend_24h),
    median30d: Number(r.median_30d),
  }));

  log.info('abuse_check_complete', { flagged_count: flagged.length });

  for (const f of flagged) {
    Sentry.captureMessage('abuse_credit_spike', {
      level: 'warning',
      tags: { kind: 'abuse_check', user_email: f.email },
      extra: { user_id: f.userId, spend_24h: f.spend24h, median_30d: f.median30d, multiplier: SPIKE_MULTIPLIER },
    });
  }

  return Response.json({ flagged });
};
