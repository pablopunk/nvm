import type { APIRoute } from 'astro';
import { sql } from 'drizzle-orm';
import { db } from '../../../db/client';
import { requireAdmin } from '../../../lib/admin';
import {
  decodeAdminPageCursor,
  encodeAdminPageCursor,
  parseAdminPageSize,
  reportBounds,
  resolveAdminReportRange,
  usageCursorIdIsValid,
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

export const GET: APIRoute = async ({ request, url }) => {
  if (!(await requireAdmin(request)))
    return new Response('Forbidden', { status: 403 });
  const range = resolveAdminReportRange(
    url.searchParams.get('from'),
    url.searchParams.get('to'),
  );
  const pageSize = parseAdminPageSize(url.searchParams.get('limit'));
  if (!range || pageSize == null) {
    return Response.json(
      {
        error: {
          type: 'invalid_report_range',
          message:
            'Provide a valid UTC range (maximum 12 months) and a page size from 1 to 100.',
        },
      },
      { status: 400 },
    );
  }
  const bounds = reportBounds(range);
  const cursor = decodeAdminPageCursor(
    url.searchParams.get('cursor'),
    bounds,
    usageCursorIdIsValid,
  );
  if (cursor === undefined) {
    return Response.json(
      {
        error: {
          type: 'invalid_cursor',
          message:
            'The request cursor is invalid or belongs to another date range.',
        },
      },
      { status: 400 },
    );
  }

  const result = await db.execute(sql`
    select
      u.id::text as id,
      to_char(u.created_at at time zone 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"') as created_at,
      u.model,
      u.provider,
      u.modality,
      u.status,
      u.latency_ms,
      u.input_tokens,
      u.output_tokens,
      u.cached_input_tokens,
      u.cache_write_input_tokens,
      u.reasoning_tokens,
      u.audio_duration_ms,
      u.audio_token_counts_available,
      u.cost_credits,
      u.upstream_cost_microcents::text as provider_cost_microcents,
      u.upstream_cost_source as provider_cost_source,
      u.request_id,
      r.actual_free_credits,
      r.actual_paid_credits,
      r.actual_free_cost_microcents::text as free_attributed_cost_microcents,
      case when r.actual_free_cost_microcents is not null
        then (u.upstream_cost_microcents - r.actual_free_cost_microcents)::text
        else null end as paid_attributed_cost_microcents,
      (u.cost_credits > 0 and (r.request_id is null or r.actual_free_credits is null or r.actual_paid_credits is null or r.actual_free_cost_microcents is null)) as pool_attribution_unknown
    from usage u
    left join credit_reservations r on r.request_id = u.request_id and r.status = 'settled'
    where u.created_at >= ${range.from}
      and u.created_at < ${range.to}
      and (${cursor?.createdAt ?? null}::timestamptz is null
        or (u.created_at, u.id) < (${cursor?.createdAt ?? null}::timestamptz, ${cursor?.id ?? null}::bigint))
    order by u.created_at desc, u.id desc
    limit ${pageSize + 1}
  `);
  const rows = resultRows<Record<string, unknown>>(result);
  const hasMore = rows.length > pageSize;
  const page = hasMore ? rows.slice(0, pageSize) : rows;
  const last = page.at(-1);
  return Response.json({
    requests: page.map((row) => ({
      createdAt: row.created_at,
      model: row.model,
      provider: row.provider,
      modality: row.modality,
      status: row.status,
      latencyMs: row.latency_ms,
      inputTokens: row.input_tokens,
      outputTokens: row.output_tokens,
      cachedInputTokens: row.cached_input_tokens,
      cacheWriteInputTokens: row.cache_write_input_tokens,
      reasoningTokens: row.reasoning_tokens,
      audioDurationMs: row.audio_duration_ms,
      audioTokenCountsAvailable: row.audio_token_counts_available,
      credits: row.cost_credits,
      freeCredits: row.actual_free_credits ?? null,
      paidCredits: row.actual_paid_credits ?? null,
      providerCostMicrocents: row.provider_cost_microcents,
      providerCostSource: row.provider_cost_source,
      freeAttributedCostMicrocents: row.free_attributed_cost_microcents,
      paidAttributedCostMicrocents: row.paid_attributed_cost_microcents,
      poolAttributionUnknown: row.pool_attribution_unknown,
      requestId: row.request_id,
    })),
    range: bounds,
    pageSize,
    nextCursor:
      hasMore && last
        ? encodeAdminPageCursor({
            createdAt: String(last.created_at),
            id: String(last.id),
            bounds,
          })
        : null,
    consistency: 'eventually_consistent',
    paginationNote:
      'New usage may appear between pages. Refresh to restart; this list is not a point-in-time snapshot.',
  });
};
