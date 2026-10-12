import type { APIRoute } from 'astro';
import { desc } from 'drizzle-orm';
import { z } from 'zod';
import { db } from '../../../../db/client';
import { financialReconciliationJobs } from '../../../../db/schema';
import { requireAdmin } from '../../../../lib/admin';
import { BillingConfigError, stripeClient } from '../../../../lib/billing';
import { requireSameOrigin } from '../../../../lib/csrf';
import {
  beginFinancialReconciliation,
  normalizeStripeBalanceTransaction,
  persistFinancialReconciliationPage,
  type ReconciliationRange,
  readStripeBalanceTransactionPage,
  resolveUtcReconciliationRange,
} from '../../../../lib/financial-reconciliation';
import { safeJsonBody } from '../../../../lib/validation';

const boundSchema = z.string().min(1).max(40);
const reconcileSchema = z
  .object({
    jobId: z.string().uuid().optional(),
    from: boundSchema.optional(),
    to: boundSchema.optional(),
  })
  .strict()
  .refine(
    (value) =>
      (!value.from && !value.to) || (Boolean(value.from) && Boolean(value.to)),
    {
      message: 'Provide both from and to when specifying a date range.',
    },
  );

function jobResponse(job: {
  id: string;
  state: string;
  cursor: string | null;
  importedCount: number;
  updatedCount: number;
  unknownCount: number;
  rangeFrom: Date;
  rangeTo: Date;
  leaseExpiresAt: Date | null;
}) {
  return {
    jobId: job.id,
    status: job.state,
    cursor: job.cursor,
    counts: {
      imported: job.importedCount,
      updated: job.updatedCount,
      unknown: job.unknownCount,
    },
    range: { from: job.rangeFrom.toISOString(), to: job.rangeTo.toISOString() },
    retryAfter: job.leaseExpiresAt?.toISOString() ?? null,
  };
}

export const GET: APIRoute = async ({ request }) => {
  if (!(await requireAdmin(request)))
    return new Response('Forbidden', { status: 403 });
  const jobs = await db
    .select({
      id: financialReconciliationJobs.id,
      rangeFrom: financialReconciliationJobs.rangeFrom,
      rangeTo: financialReconciliationJobs.rangeTo,
      state: financialReconciliationJobs.state,
      importedCount: financialReconciliationJobs.importedCount,
      updatedCount: financialReconciliationJobs.updatedCount,
      unknownCount: financialReconciliationJobs.unknownCount,
      cursor: financialReconciliationJobs.cursor,
      leaseExpiresAt: financialReconciliationJobs.leaseExpiresAt,
      updatedAt: financialReconciliationJobs.updatedAt,
    })
    .from(financialReconciliationJobs)
    .orderBy(desc(financialReconciliationJobs.updatedAt))
    .limit(100);
  return Response.json({
    jobs,
    source: 'Durable local reconciliation state; no Stripe reads were made.',
  });
};

export const POST: APIRoute = async ({ request }) => {
  const deadlineAt = Date.now() + 8000;
  const originCheck = requireSameOrigin(request);
  if (originCheck) return originCheck;
  const actor = await requireAdmin(request);
  if (!actor) return new Response('Forbidden', { status: 403 });

  const parsed = await safeJsonBody(request, reconcileSchema);
  if (!parsed.ok) return Response.json(parsed.error, { status: 400 });

  let range: ReconciliationRange | undefined;
  if (parsed.data.from || parsed.data.to || !parsed.data.jobId) {
    const resolvedRange = resolveUtcReconciliationRange(
      parsed.data.from,
      parsed.data.to,
    );
    if (!resolvedRange) {
      return Response.json(
        {
          error: {
            type: 'invalid_date_range',
            message:
              'Use a valid UTC [from, to) range no longer than one year and ending no later than now.',
          },
        },
        { status: 400 },
      );
    }
    range = resolvedRange;
  }

  let begun;
  try {
    begun = await beginFinancialReconciliation({
      actorUserId: actor.id,
      jobId: parsed.data.jobId,
      range,
      deadlineAt,
    });
  } catch (error) {
    console.error(
      '[financial-reconciliation] job claim failed',
      error instanceof Error ? error.name : 'unknown',
    );
    return Response.json(
      {
        error: {
          type: 'reconciliation_unavailable',
          message: 'Could not claim the reconciliation job.',
        },
      },
      { status: 503 },
    );
  }

  if (!begun) {
    return Response.json(
      {
        error: {
          type: 'reconciliation_not_found',
          message: 'The reconciliation job was not found for that range.',
        },
      },
      { status: parsed.data.jobId ? 404 : 409 },
    );
  }
  if (begun.status !== 'claimed') {
    if (begun.status === 'complete')
      return Response.json(jobResponse(begun.job));
    return Response.json(
      {
        ...jobResponse(begun.job),
        ...(begun.status === 'deferred'
          ? {
              message:
                'The invocation budget is too short to safely start a Stripe page; retry this job.',
            }
          : {}),
      },
      { status: 202 },
    );
  }

  try {
    const stripe = stripeClient();
    const page = await readStripeBalanceTransactionPage(stripe, {
      range: { from: begun.job.rangeFrom, to: begun.job.rangeTo },
      cursor: begun.job.cursor,
    });

    if (Date.now() > deadlineAt - 3000) {
      return Response.json(
        {
          ...jobResponse(begun.job),
          status: 'processing',
          message:
            'The page was read but could not be committed within this invocation; retry after the lease expires.',
        },
        { status: 202 },
      );
    }

    const transactions = page.data.map(normalizeStripeBalanceTransaction);
    if (transactions.some((transaction) => transaction === null)) {
      throw new Error(
        'Stripe returned a balance transaction without a stable ID',
      );
    }
    const normalized = transactions.filter(
      (transaction) => transaction !== null,
    );
    const cursorAfter = page.data.at(-1)?.id ?? null;
    const saved = await persistFinancialReconciliationPage({
      actorUserId: actor.id,
      jobId: begun.job.id,
      leaseVersion: begun.leaseVersion,
      deadlineAt,
      cursorBefore: begun.job.cursor,
      cursorAfter,
      hasMore: page.has_more,
      transactions: normalized,
    });

    if (saved.status === 'stale') {
      return Response.json(
        {
          ...jobResponse(begun.job),
          status: 'processing',
          message: 'A newer invocation owns this job.',
        },
        { status: 202 },
      );
    }
    return Response.json({
      ...jobResponse(saved.job),
      page: {
        imported: saved.importedCount,
        updated: saved.updatedCount,
        unknown: saved.unknownCount,
      },
    });
  } catch (error) {
    const configError = error instanceof BillingConfigError;
    console.error(
      '[financial-reconciliation] page failed',
      error instanceof Error ? error.name : 'unknown',
    );
    return Response.json(
      {
        jobId: begun.job.id,
        status: 'retryable',
        error: {
          type: configError
            ? 'stripe_unavailable'
            : 'reconciliation_page_failed',
          message:
            'The page was not committed. Retry this job after its lease expires.',
        },
      },
      { status: 503 },
    );
  }
};
