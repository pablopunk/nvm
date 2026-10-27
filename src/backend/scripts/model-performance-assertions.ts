import assert from 'node:assert/strict';
import { db } from '../src/db/client';
import { usage } from '../src/db/schema';
import { loadModelPerformance } from '../src/lib/admin-model-performance';
import { resolveSummaryMonth } from '../src/lib/admin-summary';

export async function runModelPerformanceAssertions(userId: string) {
  const month = resolveSummaryMonth('2026-01');
  assert.ok(month);
  const common = {
    userId, model: 'performance-test', provider: 'integration',
    inputTokens: 10, outputTokens: 100, costCredits: 0, status: 200,
    createdAt: new Date('2026-01-15T00:00:00Z'),
  };
  await db.insert(usage).values([
    { ...common, latencyMs: 1200, ttftMs: 200 },
    { ...common, latencyMs: 2400, ttftMs: 400, outputTokens: 300 },
    { ...common, latencyMs: 600 },
    { ...common, latencyMs: 200, ttftMs: 200 },
    { ...common, status: 500, latencyMs: 99999, ttftMs: 99998 },
    { ...common, createdAt: month.to, latencyMs: 99999, ttftMs: 99998 },
  ]);
  const rows = await loadModelPerformance(month);
  const row = rows.find((item) => item.model === common.model);
  assert.ok(row);
  assert.equal(row.requests, 4);
  assert.equal(row.timedRequests, 4);
  assert.equal(row.ttftSamples, 3);
  assert.equal(row.throughputSamples, 2);
  assert.equal(row.meanTurnMs, 1100);
  assert.ok(Math.abs(row.meanTtftMs! - 800 / 3) < 0.001);
  assert.equal(row.meanTokensPerSecond, 125);
}
