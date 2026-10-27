import { sql } from 'drizzle-orm';
import { db } from '../db/client';
import type { SummaryMonth } from './admin-summary';

export type ModelPerformanceRow = {
  model: string;
  provider: string | null;
  requests: number;
  timedRequests: number;
  ttftSamples: number;
  throughputSamples: number;
  meanTtftMs: number | null;
  meanTurnMs: number | null;
  meanTokensPerSecond: number | null;
};

export async function loadModelPerformance(
  month: SummaryMonth,
): Promise<ModelPerformanceRow[]> {
  const result = await db.execute(sql`
    select model, provider,
      count(*)::int as requests,
      count(latency_ms) filter (where latency_ms > 0)::int as "timedRequests",
      count(ttft_ms)::int as "ttftSamples",
      count(*) filter (where output_tokens > 0 and ttft_ms is not null
        and latency_ms > ttft_ms)::int as "throughputSamples",
      avg(ttft_ms)::float8 as "meanTtftMs",
      avg(latency_ms) filter (where latency_ms > 0)::float8 as "meanTurnMs",
      avg(output_tokens * 1000.0 / nullif(latency_ms - ttft_ms, 0))
        filter (where output_tokens > 0 and ttft_ms is not null
          and latency_ms > ttft_ms)::float8 as "meanTokensPerSecond"
    from usage
    where created_at >= ${month.from} and created_at < ${month.to}
      and status >= 200 and status < 300
    group by model, provider
    order by count(*) desc, model, provider
  `);
  return Array.isArray((result as { rows?: unknown }).rows)
    ? (result as unknown as { rows: ModelPerformanceRow[] }).rows
    : (result as unknown as ModelPerformanceRow[]);
}
