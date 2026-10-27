import type { APIRoute } from 'astro';
import { and, eq, sql } from 'drizzle-orm';
import { z } from 'zod';
import { db } from '../../../db/client';
import { creditLedger, creditReservations, users } from '../../../db/schema';
import { requireAdmin } from '../../../lib/admin';
import {
  adminMutationKeyFromRequest,
  adminMutationResponse,
  executeAdminMutation,
} from '../../../lib/admin-mutations';
import { recordAuditStrict } from '../../../lib/audit';
import { requireSameOrigin } from '../../../lib/csrf';
import { safeJsonBody } from '../../../lib/validation';

const creditsSchema = z
  .object({
    email: z.string().trim().toLowerCase().email(),
    delta: z.number().int().min(-1_000_000).max(1_000_000),
    kind: z.enum(['free', 'paid']),
    reason: z.string().trim().min(1).max(500),
  })
  .strict();

export function isCreditDeltaAllowedUntilPlan006(delta: number): boolean {
  return (
    Number.isSafeInteger(delta) && delta !== 0 && Math.abs(delta) <= 1_000_000
  );
}

export const POST: APIRoute = async ({ request }) => {
  const originCheck = requireSameOrigin(request);
  if (originCheck) return originCheck;
  const actor = await requireAdmin(request);
  if (!actor) return new Response('Forbidden', { status: 403 });
  const parsed = await safeJsonBody(request, creditsSchema);
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
  if (!isCreditDeltaAllowedUntilPlan006(parsed.data.delta)) {
    return Response.json(
      {
        error: {
          type: 'invalid_adjustment',
          message:
            'Use a non-zero integer adjustment within the allowed range.',
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
      const [target] = await tx
        .select({ id: users.id })
        .from(users)
        .where(eq(users.email, parsed.data.email))
        .for('update');
      if (!target)
        return {
          status: 404,
          body: {
            error: { type: 'user_not_found', message: 'User not found.' },
          },
        };

      const userId = target.id;
      const [pool] = await tx
        .select({
          balance: sql<number>`coalesce(sum(${creditLedger.delta}), 0)::int`,
        })
        .from(creditLedger)
        .where(
          and(
            eq(creditLedger.userId, userId),
            eq(creditLedger.kind, parsed.data.kind),
          ),
        );
      const [pending] = await tx
        .select({
          reserved: sql<number>`coalesce(sum(coalesce(
            case when ${parsed.data.kind} = 'free' then ${creditReservations.reservedFreeCredits} else ${creditReservations.reservedPaidCredits} end,
            case when ${creditReservations.kind} = ${parsed.data.kind} then ${creditReservations.reservedCredits} else 0 end
          )), 0)::int`,
        })
        .from(creditReservations)
        .where(
          and(
            eq(creditReservations.userId, userId),
            eq(creditReservations.status, 'pending'),
          ),
        );
      const balance = Number(pool?.balance ?? 0);
      const reserved = Number(pending?.reserved ?? 0);
      if (
        parsed.data.delta < 0 &&
        (balance + parsed.data.delta < reserved ||
          balance + parsed.data.delta < 0)
      ) {
        return {
          status: 409,
          body: {
            error: {
              type: 'adjustment_invades_reserved_credits',
              message:
                'The adjustment cannot reduce this pool below its pending reservations or below zero.',
            },
          },
        };
      }

      const reference = `admin-adjustment:${idempotencyKey}`;
      await tx.insert(creditLedger).values({
        userId,
        delta: parsed.data.delta,
        kind: parsed.data.kind,
        reason: parsed.data.reason,
        refId: reference,
      });
      await recordAuditStrict(tx, {
        actorUserId: actor.id,
        action: parsed.data.delta >= 0 ? 'credits.granted' : 'credits.adjusted',
        targetType: 'user',
        targetId: userId,
        meta: {
          delta: parsed.data.delta,
          kind: parsed.data.kind,
          reason: parsed.data.reason,
          idempotencyKey,
        },
      });
      return {
        body: {
          ok: true,
          email: parsed.data.email,
          kind: parsed.data.kind,
          balance: balance + parsed.data.delta,
          pendingReserved: reserved,
        },
      };
    },
  });
  return adminMutationResponse(result);
};
