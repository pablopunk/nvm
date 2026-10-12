import type { APIRoute } from 'astro';
import { sql } from 'drizzle-orm';
import { db } from '../../../db/client';
import { requireAdmin } from '../../../lib/admin';
import {
  reportBounds,
  resolveAdminReportRange,
} from '../../../lib/admin-reporting';

function resultRows<T>(result: unknown): T[] {
  if (Array.isArray(result)) return result as T[];
  if (
    result &&
    typeof result === 'object' &&
    Array.isArray((result as { rows?: unknown }).rows)
  ) {
    return (result as { rows: T[] }).rows;
  }
  return [];
}

function resolveTimeseriesRange(url: URL) {
  const from = url.searchParams.get('from');
  const to = url.searchParams.get('to');
  if (from || to) return resolveAdminReportRange(from, to);
  const daysValue = url.searchParams.get('days');
  if (daysValue == null) return resolveAdminReportRange(null, null);
  if (!/^\d+$/.test(daysValue)) return null;
  const days = Number(daysValue);
  if (!Number.isInteger(days) || days < 1 || days > 366) return null;
  const now = new Date(Math.floor(Date.now() / 1000) * 1000);
  return {
    from: new Date(now.getTime() - days * 24 * 60 * 60 * 1000),
    to: now,
  };
}

export const GET: APIRoute = async ({ request, url }) => {
  if (!(await requireAdmin(request)))
    return new Response('Forbidden', { status: 403 });
  const range = resolveTimeseriesRange(url);
  if (!range) {
    return Response.json(
      {
        error: {
          type: 'invalid_report_range',
          message:
            'Provide a valid UTC range (maximum 12 months), or days from 1 to 366.',
        },
      },
      { status: 400 },
    );
  }
  const seriesResult = await db.execute(sql`
    with days as (
      select generate_series(
        date_trunc('day', ${range.from}::timestamptz at time zone 'UTC'),
        date_trunc('day', (${range.to}::timestamptz - interval '1 microsecond') at time zone 'UTC'),
        interval '1 day'
      ) as day
    )
    select
      to_char(days.day, 'YYYY-MM-DD') as day,
      count(u.id)::text as requests,
      count(*) filter (where u.status >= 400)::text as errors,
      coalesce(sum(u.cost_credits), 0)::text as credits,
      coalesce(sum(u.input_tokens), 0)::text as input_tokens,
      coalesce(sum(u.output_tokens), 0)::text as output_tokens,
      coalesce(sum(u.cached_input_tokens), 0)::text as cached_input_tokens,
      coalesce(sum(u.cache_write_input_tokens), 0)::text as cache_write_input_tokens,
      coalesce(sum(u.reasoning_tokens), 0)::text as reasoning_tokens,
      coalesce(sum(u.audio_duration_ms), 0)::text as audio_duration_ms,
      coalesce(sum(u.upstream_cost_microcents) filter (where u.upstream_cost_source = 'provider_reported'), 0)::text as provider_reported_cost_microcents,
      coalesce(sum(u.upstream_cost_microcents) filter (where u.upstream_cost_source = 'catalog_estimate'), 0)::text as estimated_cost_microcents,
      coalesce(sum(r.actual_free_cost_microcents), 0)::text as free_attributed_cost_microcents,
      coalesce(sum(case when r.actual_free_cost_microcents is not null
        then u.upstream_cost_microcents - r.actual_free_cost_microcents else 0 end), 0)::text as paid_attributed_cost_microcents,
      count(*) filter (where u.cost_credits > 0 and (r.request_id is null or r.actual_free_credits is null or r.actual_paid_credits is null or r.actual_free_cost_microcents is null))::text as unknown_pool_requests,
      count(*) filter (where u.modality = 'audio')::text as audio_request_count,
      count(*) filter (where u.modality = 'audio' and u.audio_token_counts_available)::text as audio_token_known_count,
      coalesce(sum(u.input_tokens) filter (where u.modality = 'audio' and u.audio_token_counts_available), 0)::text as audio_input_tokens,
      coalesce(sum(u.output_tokens) filter (where u.modality = 'audio' and u.audio_token_counts_available), 0)::text as audio_output_tokens,
      percentile_cont(0.5) within group (order by u.latency_ms) filter (where u.latency_ms is not null)::text as p50,
      percentile_cont(0.95) within group (order by u.latency_ms) filter (where u.latency_ms is not null)::text as p95
    from days
    left join usage u on u.created_at >= ${range.from}
      and u.created_at < ${range.to}
      and u.created_at >= days.day at time zone 'UTC'
      and u.created_at < (days.day + interval '1 day') at time zone 'UTC'
    left join credit_reservations r on r.request_id = u.request_id and r.status = 'settled'
    group by days.day
    order by days.day asc
  `);
  const modelResult = await db.execute(sql`
    select
      u.model,
      u.provider,
      u.modality,
      count(*)::text as requests,
      coalesce(sum(u.upstream_cost_microcents) filter (where u.upstream_cost_source = 'provider_reported'), 0)::text as provider_reported_cost_microcents,
      coalesce(sum(u.upstream_cost_microcents) filter (where u.upstream_cost_source = 'catalog_estimate'), 0)::text as estimated_cost_microcents,
      coalesce(sum(r.actual_free_cost_microcents), 0)::text as free_attributed_cost_microcents,
      coalesce(sum(case when r.actual_free_cost_microcents is not null
        then u.upstream_cost_microcents - r.actual_free_cost_microcents else 0 end), 0)::text as paid_attributed_cost_microcents,
      count(*) filter (where u.cost_credits > 0 and (r.request_id is null or r.actual_free_credits is null or r.actual_paid_credits is null or r.actual_free_cost_microcents is null))::text as unknown_pool_requests,
       coalesce(sum(u.input_tokens), 0)::text as input_tokens,
       coalesce(sum(u.output_tokens), 0)::text as output_tokens,
       coalesce(sum(u.cached_input_tokens), 0)::text as cached_input_tokens,
       coalesce(sum(u.cache_write_input_tokens), 0)::text as cache_write_input_tokens,
       coalesce(sum(u.reasoning_tokens), 0)::text as reasoning_tokens,
       coalesce(sum(u.audio_duration_ms), 0)::text as audio_duration_ms,
       count(*) filter (where u.modality = 'audio')::text as audio_request_count,
       count(*) filter (where u.modality = 'audio' and u.audio_token_counts_available)::text as audio_token_known_count,
       coalesce(sum(u.input_tokens) filter (where u.modality = 'audio' and u.audio_token_counts_available), 0)::text as audio_input_tokens,
       coalesce(sum(u.output_tokens) filter (where u.modality = 'audio' and u.audio_token_counts_available), 0)::text as audio_output_tokens
    from usage u
    left join credit_reservations r on r.request_id = u.request_id and r.status = 'settled'
    where u.created_at >= ${range.from}
      and u.created_at < ${range.to}
    group by u.model, u.provider, u.modality
    order by sum(u.upstream_cost_microcents) desc, count(*) desc
    limit 100
  `);
  const rangeBounds = reportBounds(range);
  const series = resultRows<Record<string, string | null>>(seriesResult).map(
    (row) => {
      const audioAvailable =
        Number(row.audio_request_count) === Number(row.audio_token_known_count);
      return {
        day: row.day,
        requests: Number(row.requests),
        errors: Number(row.errors),
        credits: Number(row.credits),
        tokens: {
          input: Number(row.input_tokens),
          output: Number(row.output_tokens),
          cachedInput: Number(row.cached_input_tokens),
          cacheWriteInput: Number(row.cache_write_input_tokens),
          reasoning: Number(row.reasoning_tokens),
        },
        audioDurationMs: Number(row.audio_duration_ms),
        audioTokens: {
          available: audioAvailable,
          input:
            audioAvailable && Number(row.audio_request_count) > 0
              ? Number(row.audio_input_tokens)
              : null,
          output:
            audioAvailable && Number(row.audio_request_count) > 0
              ? Number(row.audio_output_tokens)
              : null,
        },
        providerCostMicrocents: {
          providerReported: row.provider_reported_cost_microcents,
          catalogEstimated: row.estimated_cost_microcents,
          freeAttributed: row.free_attributed_cost_microcents,
          paidAttributed: row.paid_attributed_cost_microcents,
          unknownPoolRequests: Number(row.unknown_pool_requests),
        },
        p50: row.p50 == null ? null : Number(row.p50),
        p95: row.p95 == null ? null : Number(row.p95),
      };
    },
  );
  const byModelProvider = resultRows<Record<string, string | null>>(
    modelResult,
  ).map((row) => {
    const audioRequestCount = Number(row.audio_request_count);
    const audioTokensKnownCount = Number(row.audio_token_known_count);
    const audioTokenCountsAvailable =
      audioRequestCount === audioTokensKnownCount;
    return {
      model: row.model,
      provider: row.provider,
      modality: row.modality,
      requests: Number(row.requests),
      providerReportedCostMicrocents: row.provider_reported_cost_microcents,
      catalogEstimatedCostMicrocents: row.estimated_cost_microcents,
      freeAttributedCostMicrocents: row.free_attributed_cost_microcents,
      paidAttributedCostMicrocents: row.paid_attributed_cost_microcents,
      unknownPoolRequests: Number(row.unknown_pool_requests),
      inputTokens: Number(row.input_tokens),
      outputTokens: Number(row.output_tokens),
      cachedInputTokens: Number(row.cached_input_tokens),
      cacheWriteInputTokens: Number(row.cache_write_input_tokens),
      reasoningTokens: Number(row.reasoning_tokens),
      audioDurationMs: Number(row.audio_duration_ms),
      audioTokenCountsAvailable,
      audioInputTokens:
        audioRequestCount > 0 && audioTokenCountsAvailable
          ? Number(row.audio_input_tokens)
          : null,
      audioOutputTokens:
        audioRequestCount > 0 && audioTokenCountsAvailable
          ? Number(row.audio_output_tokens)
          : null,
    };
  });

  return Response.json({ range: rangeBounds, series, byModelProvider });
};
