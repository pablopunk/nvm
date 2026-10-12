import type { APIRoute } from 'astro';
import { sql } from 'drizzle-orm';
import { db } from '../../../db/client';
import { requireAdmin } from '../../../lib/admin';
import {
  parseAdminPageSize,
  reportBounds,
  resolveAdminReportRange,
} from '../../../lib/admin-reporting';
import { reconciliationCoverageComplete } from '../../../lib/financial-reconciliation';
import { getFreeSpendBudgetSnapshots } from '../../../lib/free-spend-cap';

type SqlRow = Record<string, string | number | boolean | null>;

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

function safeCount(value: unknown): number {
  const parsed = Number(value ?? 0);
  return Number.isSafeInteger(parsed) && parsed >= 0 ? parsed : 0;
}

function financialMetric(value: unknown, complete: boolean) {
  return {
    knownMinorUnits: String(value ?? 0),
    valueMinorUnits: complete ? String(value ?? 0) : null,
    complete,
  };
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

  const usageResult = await db.execute(sql`
    select
      count(*)::text as requests,
      count(*) filter (where status >= 400)::text as errors,
      coalesce(sum(input_tokens), 0)::text as input_tokens,
      coalesce(sum(output_tokens), 0)::text as output_tokens,
      coalesce(sum(cached_input_tokens), 0)::text as cached_input_tokens,
      coalesce(sum(cache_write_input_tokens), 0)::text as cache_write_input_tokens,
      coalesce(sum(reasoning_tokens), 0)::text as reasoning_tokens,
      coalesce(sum(audio_duration_ms), 0)::text as audio_duration_ms,
      coalesce(sum(cost_credits), 0)::text as cost_credits,
      coalesce(sum(upstream_cost_microcents) filter (where upstream_cost_source = 'provider_reported'), 0)::text as provider_reported_cost_microcents,
      coalesce(sum(upstream_cost_microcents) filter (where upstream_cost_source = 'catalog_estimate'), 0)::text as catalog_estimated_cost_microcents,
      count(*) filter (where upstream_cost_source is null or upstream_cost_source not in ('provider_reported', 'catalog_estimate', 'not_billed'))::text as unknown_cost_source_requests,
      coalesce(sum(r.actual_free_cost_microcents), 0)::text as free_attributed_cost_microcents,
      coalesce(sum(case when r.actual_free_cost_microcents is not null
        then u.upstream_cost_microcents - r.actual_free_cost_microcents else 0 end), 0)::text as paid_attributed_cost_microcents,
      count(*) filter (where u.cost_credits > 0 and (r.request_id is null or r.actual_free_credits is null or r.actual_paid_credits is null or r.actual_free_cost_microcents is null))::text as unknown_pool_requests,
      count(*) filter (where coalesce(r.actual_free_credits, 0) > 0 and coalesce(r.actual_paid_credits, 0) > 0)::text as mixed_pool_requests,
      count(*) filter (where u.upstream_cost_source = 'catalog_estimate' and u.cost_credits > 0)::text as estimated_cost_requests
    from usage u
    left join credit_reservations r on r.request_id = u.request_id and r.status = 'settled'
    where u.created_at >= ${range.from} and u.created_at < ${range.to}
  `);
  const creditResult = await db.execute(sql`
    select
      kind,
      coalesce(sum(case when delta > 0 and coalesce(ref_id, '') not like 'admin-adjustment:%' then delta else 0 end)
        filter (where created_at >= ${range.from} and created_at < ${range.to}), 0)::text as granted,
      coalesce(sum(case when reason = 'ai_usage' and delta < 0 then -delta else 0 end)
        filter (where created_at >= ${range.from} and created_at < ${range.to}), 0)::text as consumed,
      coalesce(sum(delta) filter (where coalesce(ref_id, '') like 'admin-adjustment:%' and created_at >= ${range.from} and created_at < ${range.to}), 0)::text as adjusted,
      coalesce(sum(delta), 0)::text as outstanding
    from credit_ledger
    group by kind
  `);
  const modelResult = await db.execute(sql`
    select
      u.model,
      u.provider,
      u.modality,
      u.upstream_cost_source,
      count(*)::text as requests,
      count(*) filter (where u.status >= 400)::text as errors,
      coalesce(sum(u.input_tokens), 0)::text as input_tokens,
      coalesce(sum(u.output_tokens), 0)::text as output_tokens,
      coalesce(sum(u.cached_input_tokens), 0)::text as cached_input_tokens,
      coalesce(sum(u.cache_write_input_tokens), 0)::text as cache_write_input_tokens,
      coalesce(sum(u.reasoning_tokens), 0)::text as reasoning_tokens,
      coalesce(sum(u.audio_duration_ms), 0)::text as audio_duration_ms,
      count(*) filter (where u.modality = 'audio')::text as audio_request_count,
      count(*) filter (where u.modality = 'audio' and u.audio_token_counts_available)::text as audio_token_known_count,
      coalesce(sum(u.input_tokens) filter (where u.modality = 'audio' and u.audio_token_counts_available), 0)::text as audio_input_tokens,
      coalesce(sum(u.output_tokens) filter (where u.modality = 'audio' and u.audio_token_counts_available), 0)::text as audio_output_tokens,
      coalesce(sum(u.cost_credits), 0)::text as cost_credits,
      coalesce(sum(u.upstream_cost_microcents), 0)::text as provider_cost_microcents,
      coalesce(sum(r.actual_free_cost_microcents), 0)::text as free_attributed_cost_microcents,
      coalesce(sum(case when r.actual_free_cost_microcents is not null
        then u.upstream_cost_microcents - r.actual_free_cost_microcents else 0 end), 0)::text as paid_attributed_cost_microcents,
      count(*) filter (where u.cost_credits > 0 and (r.request_id is null or r.actual_free_credits is null or r.actual_paid_credits is null or r.actual_free_cost_microcents is null))::text as unknown_pool_requests,
      count(*) filter (where u.upstream_cost_source is null or u.upstream_cost_source not in ('provider_reported', 'catalog_estimate', 'not_billed'))::text as unknown_cost_source_requests
    from usage u
    left join credit_reservations r on r.request_id = u.request_id and r.status = 'settled'
    where u.created_at >= ${range.from} and u.created_at < ${range.to}
    group by u.model, u.provider, u.modality, u.upstream_cost_source
    order by sum(u.upstream_cost_microcents) desc, count(*) desc
    limit ${pageSize}
  `);
  const financialResult = await db.execute(sql`
    select
      coalesce(currency, 'UNKNOWN') as currency,
      count(*)::text as transactions,
      count(*) filter (where not fee_known)::text as unknown_fee_rows,
      count(*) filter (where amount is null)::text as unknown_amount_rows,
      count(*) filter (where net is null)::text as unknown_net_rows,
      count(*) filter (where reporting_category is null)::text as unknown_category_rows,
      count(*) filter (where status is null)::text as unknown_status_rows,
      count(*) filter (where status = 'pending')::text as pending_rows,
      count(*) filter (where status = 'available')::text as available_rows,
      coalesce(sum(amount) filter (where classified_as = 'charge'), 0)::text as gross_captured_charges,
      coalesce(sum(net) filter (where classified_as = 'partial_capture_reversal'), 0)::text as partial_capture_reversal_movements,
      coalesce(sum(abs(amount)) filter (where classified_as = 'refund'), 0)::text as refunds,
      coalesce(sum(net) filter (where classified_as = 'dispute'), 0)::text as disputes,
      coalesce(sum(net) filter (where classified_as = 'dispute_reversal'), 0)::text as dispute_reversals,
      coalesce(sum(fee) filter (where classified_as in ('charge', 'refund', 'partial_capture_reversal')), 0)::text as payment_transaction_fees,
      count(*) filter (where classified_as in ('charge', 'refund', 'partial_capture_reversal') and not fee_known)::text as unknown_payment_fee_rows,
      coalesce(sum(net) filter (where classified_as = 'fee'), 0)::text as standalone_fee_movements,
      coalesce(sum(net) filter (where classified_as = 'tax'), 0)::text as stripe_tax_movements,
      coalesce(sum(net) filter (where classified_as in ('charge', 'refund', 'partial_capture_reversal', 'dispute', 'dispute_reversal', 'fee', 'tax')), 0)::text as selected_payment_category_movement,
      count(*) filter (where classified_as in ('charge', 'refund', 'partial_capture_reversal', 'dispute', 'dispute_reversal', 'fee', 'tax') and net is null)::text as selected_missing_net_rows,
      coalesce(sum(net) filter (where classified_as = 'charge_failure'), 0)::text as charge_failures,
      coalesce(sum(net) filter (where classified_as = 'refund_failure'), 0)::text as refund_failures,
      coalesce(sum(net) filter (where classified_as = 'other_adjustment'), 0)::text as other_adjustments,
      count(*) filter (where classified_as = 'unclassified')::text as unclassified_rows,
      coalesce(sum(net) filter (where classified_as = 'unclassified'), 0)::text as unclassified_known_net,
      coalesce(sum(net) filter (where classified_as = 'payout'), 0)::text as payouts,
      coalesce(sum(net) filter (where classified_as = 'transfer'), 0)::text as transfers,
      coalesce(sum(net) filter (where classified_as = 'reserve'), 0)::text as reserves
    from stripe_balance_transactions
    where created_at_stripe >= ${range.from} and created_at_stripe < ${range.to}
    group by coalesce(currency, 'UNKNOWN')
    order by coalesce(currency, 'UNKNOWN') asc
  `);
  const coverageResult = await db.execute(sql`
    select range_from::text as range_from, range_to::text as range_to
    from financial_reconciliation_jobs
    where state = 'complete'
      and range_to > ${range.from}
      and range_from < ${range.to}
    order by range_from asc
  `);
  const fieldCompletenessResult = await db.execute(sql`
    select
      count(*) filter (where not fee_known)::text as fee_unknown,
      count(*) filter (where amount is null)::text as amount_unknown,
      count(*) filter (where net is null)::text as net_unknown,
      count(*) filter (where currency is null)::text as currency_unknown,
      count(*) filter (where reporting_category is null)::text as category_unknown,
      count(*) filter (where status is null)::text as status_unknown
    from stripe_balance_transactions
    where created_at_stripe >= ${range.from} and created_at_stripe < ${range.to}
  `);
  const userCountResult = await db.execute(sql`
    select count(*)::text as created_in_range from users
    where created_at >= ${range.from} and created_at < ${range.to}
  `);
  const [
    capSnapshots,
    usageTotals,
    creditTotals,
    fieldCompleteness,
    userCount,
  ] = await Promise.all([
    getFreeSpendBudgetSnapshots(),
    Promise.resolve(resultRows<SqlRow>(usageResult)[0] ?? {}),
    Promise.resolve(resultRows<SqlRow>(creditResult)),
    Promise.resolve(resultRows<SqlRow>(fieldCompletenessResult)[0] ?? {}),
    Promise.resolve(resultRows<SqlRow>(userCountResult)[0] ?? {}),
  ]);
  const { config, current: budget, currentMonth } = capSnapshots;
  const coverageRows = resultRows<{ range_from: string; range_to: string }>(
    coverageResult,
  );
  const completedIntervals = coverageRows.map((row) => ({
    from: new Date(row.range_from),
    to: new Date(row.range_to),
  }));
  const intervalCoverageComplete = reconciliationCoverageComplete(
    completedIntervals,
    range,
  );
  const financialRows = resultRows<SqlRow>(financialResult);
  const categoryUnknown = safeCount(fieldCompleteness.category_unknown);
  const amountUnknown = safeCount(fieldCompleteness.amount_unknown);
  const netUnknown = safeCount(fieldCompleteness.net_unknown);
  const feeUnknown = safeCount(fieldCompleteness.fee_unknown);
  const currencyUnknown = safeCount(fieldCompleteness.currency_unknown);
  const selectedMissingNet = financialRows.reduce(
    (sum, row) => sum + safeCount(row.selected_missing_net_rows),
    0,
  );
  const categoriesComplete =
    intervalCoverageComplete && categoryUnknown === 0 && amountUnknown === 0;
  const selectedMovementComplete =
    intervalCoverageComplete &&
    categoryUnknown === 0 &&
    currencyUnknown === 0 &&
    selectedMissingNet === 0;
  const paymentRows = financialRows.map((row) => ({
    currency: row.currency,
    transactionCount: safeCount(row.transactions),
    status: {
      pending: safeCount(row.pending_rows),
      available: safeCount(row.available_rows),
      unknown: safeCount(row.unknown_status_rows),
    },
    grossCapturedCharges: financialMetric(
      row.gross_captured_charges,
      categoriesComplete && row.currency !== 'UNKNOWN',
    ),
    refunds: financialMetric(
      row.refunds,
      categoriesComplete && row.currency !== 'UNKNOWN',
    ),
    partialCaptureReversalMovements: financialMetric(
      row.partial_capture_reversal_movements,
      intervalCoverageComplete &&
        safeCount(row.unknown_category_rows) === 0 &&
        safeCount(row.unknown_net_rows) === 0 &&
        row.currency !== 'UNKNOWN',
    ),
    disputes: financialMetric(
      row.disputes,
      intervalCoverageComplete &&
        safeCount(row.unknown_category_rows) === 0 &&
        safeCount(row.unknown_net_rows) === 0 &&
        row.currency !== 'UNKNOWN',
    ),
    disputeReversals: financialMetric(
      row.dispute_reversals,
      intervalCoverageComplete &&
        safeCount(row.unknown_category_rows) === 0 &&
        safeCount(row.unknown_net_rows) === 0 &&
        row.currency !== 'UNKNOWN',
    ),
    paymentTransactionFees: financialMetric(
      row.payment_transaction_fees,
      intervalCoverageComplete &&
        safeCount(row.unknown_category_rows) === 0 &&
        safeCount(row.unknown_payment_fee_rows) === 0 &&
        row.currency !== 'UNKNOWN',
    ),
    unknownPaymentFeeRows: safeCount(row.unknown_payment_fee_rows),
    unknownFeeRows: safeCount(row.unknown_fee_rows),
    standaloneFeeMovements: financialMetric(
      row.standalone_fee_movements,
      intervalCoverageComplete &&
        safeCount(row.unknown_category_rows) === 0 &&
        safeCount(row.unknown_net_rows) === 0 &&
        row.currency !== 'UNKNOWN',
    ),
    stripeTaxMovements: financialMetric(
      row.stripe_tax_movements,
      intervalCoverageComplete &&
        safeCount(row.unknown_category_rows) === 0 &&
        safeCount(row.unknown_net_rows) === 0 &&
        row.currency !== 'UNKNOWN',
    ),
    selectedPaymentCategoryMovement: financialMetric(
      row.selected_payment_category_movement,
      selectedMovementComplete && row.currency !== 'UNKNOWN',
    ),
    chargeFailures: financialMetric(
      row.charge_failures,
      intervalCoverageComplete &&
        safeCount(row.unknown_category_rows) === 0 &&
        safeCount(row.unknown_net_rows) === 0 &&
        row.currency !== 'UNKNOWN',
    ),
    refundFailures: financialMetric(
      row.refund_failures,
      intervalCoverageComplete &&
        safeCount(row.unknown_category_rows) === 0 &&
        safeCount(row.unknown_net_rows) === 0 &&
        row.currency !== 'UNKNOWN',
    ),
    otherAdjustments: financialMetric(
      row.other_adjustments,
      intervalCoverageComplete &&
        safeCount(row.unknown_category_rows) === 0 &&
        safeCount(row.unknown_net_rows) === 0 &&
        row.currency !== 'UNKNOWN',
    ),
    unclassified: {
      rows: safeCount(row.unclassified_rows),
      knownNetMinorUnits: String(row.unclassified_known_net ?? 0),
    },
    payouts: financialMetric(
      row.payouts,
      intervalCoverageComplete &&
        safeCount(row.unknown_category_rows) === 0 &&
        safeCount(row.unknown_net_rows) === 0 &&
        row.currency !== 'UNKNOWN',
    ),
    transfers: financialMetric(
      row.transfers,
      intervalCoverageComplete &&
        safeCount(row.unknown_category_rows) === 0 &&
        safeCount(row.unknown_net_rows) === 0 &&
        row.currency !== 'UNKNOWN',
    ),
    reserves: financialMetric(
      row.reserves,
      intervalCoverageComplete &&
        safeCount(row.unknown_category_rows) === 0 &&
        safeCount(row.unknown_net_rows) === 0 &&
        row.currency !== 'UNKNOWN',
    ),
    unknowns: {
      amountRows: safeCount(row.unknown_amount_rows),
      netRows: safeCount(row.unknown_net_rows),
      categoryRows: safeCount(row.unknown_category_rows),
      currencyRows:
        row.currency === 'UNKNOWN' ? safeCount(row.transactions) : 0,
    },
    source: 'Stripe balance transactions',
  }));
  const usageTotalsJson = {
    requests: safeCount(usageTotals.requests),
    errors: safeCount(usageTotals.errors),
    inputTokens: safeCount(usageTotals.input_tokens),
    outputTokens: safeCount(usageTotals.output_tokens),
    cachedInputTokens: safeCount(usageTotals.cached_input_tokens),
    cacheWriteInputTokens: safeCount(usageTotals.cache_write_input_tokens),
    reasoningTokens: safeCount(usageTotals.reasoning_tokens),
    audioDurationMs: safeCount(usageTotals.audio_duration_ms),
    costCredits: safeCount(usageTotals.cost_credits),
    providerReportedCostMicrocents: String(
      usageTotals.provider_reported_cost_microcents ?? 0,
    ),
    catalogEstimatedCostMicrocents: String(
      usageTotals.catalog_estimated_cost_microcents ?? 0,
    ),
    trackedCostSource:
      'Stored customer-request usage; admin provider probes are excluded and their cost is unknown.',
    unknownCostSourceRequests: safeCount(
      usageTotals.unknown_cost_source_requests,
    ),
    freeSubsidyEstimate: {
      knownMicrocents: String(usageTotals.free_attributed_cost_microcents ?? 0),
      complete:
        safeCount(usageTotals.unknown_pool_requests) === 0 &&
        safeCount(usageTotals.unknown_cost_source_requests) === 0,
      unknownPoolRequests: safeCount(usageTotals.unknown_pool_requests),
      mixedPoolRequests: safeCount(usageTotals.mixed_pool_requests),
      catalogEstimatedRequests: safeCount(usageTotals.estimated_cost_requests),
      note: 'Provider cost attributed to free-pool credits. Catalog pricing and mixed-pool allocation are estimates; this is not total provider spend.',
    },
    paidAttributedCostMicrocents: String(
      usageTotals.paid_attributed_cost_microcents ?? 0,
    ),
  };
  const creditByKind = Object.fromEntries(
    creditTotals.map((row) => [
      String(row.kind),
      {
        granted: String(row.granted ?? 0),
        consumed: String(row.consumed ?? 0),
        adjusted: String(row.adjusted ?? 0),
        outstanding: String(row.outstanding ?? 0),
      },
    ]),
  );
  const modelRows = resultRows<SqlRow>(modelResult).map((row) => ({
    model: row.model,
    provider: row.provider,
    modality: row.modality,
    costSource: row.upstream_cost_source,
    requests: safeCount(row.requests),
    errors: safeCount(row.errors),
    inputTokens: safeCount(row.input_tokens),
    outputTokens: safeCount(row.output_tokens),
    credits: safeCount(row.cost_credits),
    providerCostMicrocents: String(row.provider_cost_microcents ?? 0),
    freeAttributedCostMicrocents: String(
      row.free_attributed_cost_microcents ?? 0,
    ),
    paidAttributedCostMicrocents: String(
      row.paid_attributed_cost_microcents ?? 0,
    ),
    unknownPoolRequests: safeCount(row.unknown_pool_requests),
    unknownCostSourceRequests: safeCount(row.unknown_cost_source_requests),
    cachedInputTokens: safeCount(row.cached_input_tokens),
    cacheWriteInputTokens: safeCount(row.cache_write_input_tokens),
    reasoningTokens: safeCount(row.reasoning_tokens),
    audioDurationMs: safeCount(row.audio_duration_ms),
    audioTokenCountsAvailable:
      safeCount(row.audio_request_count) ===
      safeCount(row.audio_token_known_count),
    audioInputTokens:
      safeCount(row.audio_request_count) > 0 &&
      safeCount(row.audio_request_count) ===
        safeCount(row.audio_token_known_count)
        ? safeCount(row.audio_input_tokens)
        : null,
    audioOutputTokens:
      safeCount(row.audio_request_count) > 0 &&
      safeCount(row.audio_request_count) ===
        safeCount(row.audio_token_known_count)
        ? safeCount(row.audio_output_tokens)
        : null,
  }));
  const bounds = reportBounds(range);

  return Response.json({
    range: bounds,
    users: { createdInRange: safeCount(userCount.created_in_range) },
    usage: usageTotalsJson,
    credits: {
      free: creditByKind.free ?? {
        granted: '0',
        consumed: '0',
        adjusted: '0',
        outstanding: '0',
      },
      paid: creditByKind.paid ?? {
        granted: '0',
        consumed: '0',
        adjusted: '0',
        outstanding: '0',
      },
      note: 'Credit counts are not Stripe cash, revenue, profit, or a GAAP liability.',
    },
    providerCostByModel: modelRows,
    pageSize,
    financials: {
      intervalCoverage: {
        complete: intervalCoverageComplete,
        from: bounds.from,
        to: bounds.to,
        completedIntervals: completedIntervals.map((interval) => ({
          from: interval.from.toISOString(),
          to: interval.to.toISOString(),
        })),
        note: intervalCoverageComplete
          ? 'Completed Stripe reconciliation intervals cover the selected UTC period.'
          : 'Stripe transaction history has interval gaps; affected totals are unavailable, not zero.',
      },
      fieldCompleteness: {
        feeUnknownRows: feeUnknown,
        amountUnknownRows: amountUnknown,
        netUnknownRows: netUnknown,
        currencyUnknownRows: currencyUnknown,
        categoryUnknownRows: categoryUnknown,
        statusUnknownRows: safeCount(fieldCompleteness.status_unknown),
        note: 'Missing fee detail does not invalidate Stripe-provided net. Missing amount/net/category/currency is reported separately.',
      },
      byCurrency: paymentRows,
      selectedMovementDefinition:
        'Signed net for charge, refund, partial capture reversal, dispute, dispute reversal, standalone fee, and Stripe tax categories. This is not net cash, settled cash, total Stripe balance movement, or sales revenue.',
    },
    freeSpendCap: {
      configuredCapMicrocents:
        config?.capMicrocents == null ? null : String(config.capMicrocents),
      currentMonth,
      current: budget
        ? {
            capMicrocents:
              budget.capMicrocents == null
                ? null
                : String(budget.capMicrocents),
            actualFreeCostMicrocents: String(budget.actualFreeCostMicrocents),
            reservedFreeCostMicrocents: String(
              budget.reservedFreeCostMicrocents,
            ),
            halted: budget.halted,
            baselineComplete: budget.baselineComplete,
          }
        : null,
      unconfigured: (budget?.capMicrocents ?? null) === null,
      warning:
        (budget?.capMicrocents ?? null) === null
          ? 'No aggregate cap configured. Aggregate free-provider subsidy remains unbounded until an owner sets a value.'
          : null,
    },
  });
};
