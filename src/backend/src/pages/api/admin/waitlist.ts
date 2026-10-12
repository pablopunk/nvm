import type { APIRoute } from 'astro';
import { eq, inArray } from 'drizzle-orm';
import { z } from 'zod';
import { db } from '../../../db/client';
import { waitlistEntries } from '../../../db/schema';
import { requireAdmin } from '../../../lib/admin';
import {
  adminMutationKeyFromRequest,
  adminMutationResponse,
  executeAdminMutation,
} from '../../../lib/admin-mutations';
import { recordAuditStrict } from '../../../lib/audit';
import { requireSameOrigin } from '../../../lib/csrf';
import { env } from '../../../lib/env';
import { safeJsonBody } from '../../../lib/validation';
import { createInvite, listWaitlist } from '../../../lib/waitlist';

const configuredBulkCap = Number(env('INVITE_BULK_CAP') || 50);
const bulkCap =
  Number.isInteger(configuredBulkCap) && configuredBulkCap > 0
    ? Math.min(100, configuredBulkCap)
    : 50;
const inviteSchema = z.union([
  z
    .object({ entryIds: z.array(z.string().uuid()).min(1).max(bulkCap) })
    .strict(),
  z
    .object({
      email: z.string().email().max(320),
      waitlistEntryId: z.string().uuid().optional(),
    })
    .strict(),
]);
const waitlistUpdateSchema = z
  .object({
    id: z.string().uuid(),
    status: z.enum(['pending', 'declined', 'blocked', 'suppressed']),
    note: z.string().trim().max(500).optional(),
  })
  .strict();

export const GET: APIRoute = async ({ request, url }) => {
  if (!(await requireAdmin(request)))
    return new Response('Forbidden', { status: 403 });
  return Response.json({
    entries: await listWaitlist(url.searchParams.get('status') ?? undefined),
  });
};

export const POST: APIRoute = async ({ request }) => {
  const originCheck = requireSameOrigin(request);
  if (originCheck) return originCheck;
  const actor = await requireAdmin(request);
  if (!actor) return new Response('Forbidden', { status: 403 });
  const parsed = await safeJsonBody(request, inviteSchema);
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
      if ('entryIds' in parsed.data) {
        const entries = await tx
          .select()
          .from(waitlistEntries)
          .where(inArray(waitlistEntries.id, parsed.data.entryIds))
          .for('update');
        let created = 0;
        let skipped = parsed.data.entryIds.length - entries.length;
        for (const entry of entries) {
          if (entry.status !== 'pending') {
            skipped += 1;
            continue;
          }
          const invite = await createInvite(
            {
              email: entry.email,
              waitlistEntryId: entry.id,
              createdBy: actor.id,
            },
            tx,
          );
          if (invite.existing) {
            skipped += 1;
            continue;
          }
          created += 1;
          await recordAuditStrict(tx, {
            actorUserId: actor.id,
            action: 'waitlist.invite_queued',
            targetType: 'waitlist_entry',
            targetId: entry.id,
            meta: {
              email: entry.email,
              inviteId: invite.invite.id,
              idempotencyKey,
            },
          });
        }
        return { body: { created, skipped } };
      }

      const invite = await createInvite(
        {
          email: parsed.data.email,
          waitlistEntryId: parsed.data.waitlistEntryId,
          createdBy: actor.id,
        },
        tx,
      );
      if (!invite.existing) {
        await recordAuditStrict(tx, {
          actorUserId: actor.id,
          action: 'waitlist.invite_queued',
          targetType: parsed.data.waitlistEntryId ? 'waitlist_entry' : 'invite',
          targetId: parsed.data.waitlistEntryId ?? invite.invite.id,
          meta: {
            email: invite.invite.email,
            inviteId: invite.invite.id,
            idempotencyKey,
          },
        });
      }
      return {
        status: invite.existing ? 200 : 201,
        body: { invite: invite.invite, queued: !invite.existing },
      };
    },
  });
  return adminMutationResponse(result);
};

export const PUT: APIRoute = async ({ request }) => {
  const originCheck = requireSameOrigin(request);
  if (originCheck) return originCheck;
  const actor = await requireAdmin(request);
  if (!actor) return new Response('Forbidden', { status: 403 });
  const parsed = await safeJsonBody(request, waitlistUpdateSchema);
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
      const [entry] = await tx
        .update(waitlistEntries)
        .set({
          status: parsed.data.status,
          reviewNote: parsed.data.note,
          reviewedAt: new Date(),
          reviewerId: actor.id,
        })
        .where(eq(waitlistEntries.id, parsed.data.id))
        .returning();
      if (!entry)
        return {
          status: 404,
          body: {
            error: {
              type: 'waitlist_entry_not_found',
              message: 'Waitlist entry not found.',
            },
          },
        };
      await recordAuditStrict(tx, {
        actorUserId: actor.id,
        action: 'waitlist.status_changed',
        targetType: 'waitlist_entry',
        targetId: entry.id,
        meta: {
          status: parsed.data.status,
          note: parsed.data.note ?? null,
          idempotencyKey,
        },
      });
      return { body: { entry } };
    },
  });
  return adminMutationResponse(result);
};
