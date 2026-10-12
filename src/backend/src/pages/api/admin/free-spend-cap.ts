import type { APIRoute } from 'astro';
import { and, eq, sql } from 'drizzle-orm';
import { z } from 'zod';
import { db } from '../../../db/client';
import {
  creditReservations,
  freeSpendCapConfig,
  freeSpendMonthlyBudgets,
} from '../../../db/schema';
import { requireAdmin } from '../../../lib/admin';
import {
  adminMutationKeyFromRequest,
  adminMutationResponse,
  executeAdminMutation,
} from '../../../lib/admin-mutations';
import { recordAuditStrict } from '../../../lib/audit';
import { requireSameOrigin } from '../../../lib/csrf';
import {
  ensureFreeSpendMonthlyBudget,
  getFreeSpendBudgetSnapshots,
  nextUtcMonth,
  utcMonthKey,
} from '../../../lib/free-spend-cap';
import { safeJsonBody } from '../../../lib/validation';

const capSchema = z
  .object({
    capMicrocents: z
      .number()
      .int()
      .min(0)
      .max(Number.MAX_SAFE_INTEGER)
      .nullable(),
    effective: z.enum(['current', 'next_month']),
  })
  .strict();

function capState(
  budget: typeof freeSpendMonthlyBudgets.$inferSelect | undefined,
) {
  if (!budget) return null;
  return {
    month: budget.monthKey,
    capMicrocents: budget.capMicrocents,
    actualFreeCostMicrocents: budget.actualFreeCostMicrocents,
    reservedFreeCostMicrocents: budget.reservedFreeCostMicrocents,
    remainingMicrocents:
      budget.capMicrocents == null
        ? null
        : Math.max(
            0,
            budget.capMicrocents -
              budget.actualFreeCostMicrocents -
              budget.reservedFreeCostMicrocents,
          ),
    halted: budget.halted,
    baselineComplete: budget.baselineComplete,
  };
}

export const GET: APIRoute = async ({ request }) => {
  if (!(await requireAdmin(request)))
    return new Response('Forbidden', { status: 403 });
  const {
    config,
    current,
    next: upcoming,
    currentMonth,
    nextMonth,
  } = await getFreeSpendBudgetSnapshots();
  return Response.json({
    configuredCapMicrocents: config?.capMicrocents ?? null,
    current: capState(current),
    nextMonth: capState(upcoming),
    currentMonth,
    nextMonthKey: nextMonth,
    unset: (current?.capMicrocents ?? null) === null,
  });
};

export const POST: APIRoute = async ({ request }) => {
  const originCheck = requireSameOrigin(request);
  if (originCheck) return originCheck;
  const actor = await requireAdmin(request);
  if (!actor) return new Response('Forbidden', { status: 403 });
  const parsed = await safeJsonBody(request, capSchema);
  if (!parsed.ok) return Response.json(parsed.error, { status: 400 });
  const idempotencyKey = adminMutationKeyFromRequest(request);
  if (!idempotencyKey) {
    return Response.json(
      {
        error: {
          type: 'idempotency_key_required',
          message: 'Provide a UUID Idempotency-Key.',
        },
      },
      { status: 400 },
    );
  }

  const result = await executeAdminMutation({
    actorUserId: actor.id,
    idempotencyKey,
    payload: parsed.data,
    execute: async (tx) => {
      const [config] = await tx
        .select()
        .from(freeSpendCapConfig)
        .where(eq(freeSpendCapConfig.id, 'global'))
        .for('update');
      if (!config)
        throw new Error(
          'Global free-spend cap configuration is not initialized',
        );

      const now = new Date();
      const currentMonth = utcMonthKey(now);
      const nextMonth = utcMonthKey(nextUtcMonth(now));
      const current = await ensureFreeSpendMonthlyBudget(
        tx,
        currentMonth,
        config.capMicrocents,
      );
      let affectedBudget: typeof current;
      const previousCap = config.capMicrocents;

      if (parsed.data.effective === 'current') {
        const [pendingLegacy] = await tx
          .select({ count: sql<number>`count(*)::int` })
          .from(creditReservations)
          .where(
            and(
              eq(creditReservations.status, 'pending'),
              sql`(${creditReservations.admissionMonth} is null or ${creditReservations.reservedFreeCostMicrocents} is null)`,
            ),
          );
        const heldTotal =
          current.actualFreeCostMicrocents + current.reservedFreeCostMicrocents;
        if (parsed.data.capMicrocents != null && !current.baselineComplete) {
          return {
            status: 409,
            body: {
              error: {
                type: 'free_spend_baseline_incomplete',
                message:
                  'Current-month cap cannot be activated because some usage lacks verifiable free-pool cost attribution. Schedule the cap for next month instead.',
              },
            },
          };
        }
        if (
          parsed.data.capMicrocents != null &&
          Number(pendingLegacy?.count ?? 0) > 0
        ) {
          return {
            status: 409,
            body: {
              error: {
                type: 'legacy_reservations_pending',
                message:
                  'Current-month cap cannot be activated while pending requests lack an admission month or cost hold. Schedule the cap for next month instead.',
              },
            },
          };
        }
        if (
          parsed.data.capMicrocents != null &&
          parsed.data.capMicrocents < heldTotal
        ) {
          return {
            status: 409,
            body: {
              error: {
                type: 'cap_below_committed_and_reserved',
                message:
                  'The cap cannot be lower than this month’s committed plus reserved free-provider cost.',
              },
            },
          };
        }
        affectedBudget = current;
        await tx
          .update(freeSpendMonthlyBudgets)
          .set({
            capMicrocents: parsed.data.capMicrocents,
            halted:
              parsed.data.capMicrocents != null &&
              (current.actualFreeCostMicrocents >= parsed.data.capMicrocents ||
                heldTotal > parsed.data.capMicrocents),
            updatedAt: now,
          })
          .where(eq(freeSpendMonthlyBudgets.monthKey, currentMonth));
      } else {
        const nextBudget = await ensureFreeSpendMonthlyBudget(
          tx,
          nextMonth,
          parsed.data.capMicrocents,
        );
        affectedBudget = nextBudget;
        await tx
          .update(freeSpendMonthlyBudgets)
          .set({
            capMicrocents: parsed.data.capMicrocents,
            halted:
              parsed.data.capMicrocents != null &&
              (!nextBudget.baselineComplete ||
                nextBudget.actualFreeCostMicrocents >=
                  parsed.data.capMicrocents ||
                nextBudget.actualFreeCostMicrocents +
                  nextBudget.reservedFreeCostMicrocents >
                  parsed.data.capMicrocents),
            updatedAt: now,
          })
          .where(eq(freeSpendMonthlyBudgets.monthKey, nextMonth));
      }

      await tx
        .update(freeSpendCapConfig)
        .set({
          capMicrocents: parsed.data.capMicrocents,
          updatedBy: actor.id,
          updatedAt: now,
        })
        .where(eq(freeSpendCapConfig.id, 'global'));
      await recordAuditStrict(tx, {
        actorUserId: actor.id,
        action: 'free_spend_cap.changed',
        targetType: 'free_spend_cap',
        targetId:
          parsed.data.effective === 'current' ? currentMonth : nextMonth,
        meta: {
          effective: parsed.data.effective,
          previousCapMicrocents: previousCap,
          capMicrocents: parsed.data.capMicrocents,
          month: parsed.data.effective === 'current' ? currentMonth : nextMonth,
        },
      });
      const [updated] = await tx
        .select()
        .from(freeSpendMonthlyBudgets)
        .where(eq(freeSpendMonthlyBudgets.monthKey, affectedBudget.monthKey))
        .limit(1);
      const [updatedCurrent] = await tx
        .select()
        .from(freeSpendMonthlyBudgets)
        .where(eq(freeSpendMonthlyBudgets.monthKey, currentMonth))
        .limit(1);
      return {
        body: {
          ok: true,
          effective: parsed.data.effective,
          configuredCapMicrocents: parsed.data.capMicrocents,
          current: capState(updatedCurrent),
          affected: capState(updated),
        },
      };
    },
  });
  if (result.conflict && result.status === 409)
    return adminMutationResponse(result);
  return adminMutationResponse(result);
};
