import type { APIRoute } from 'astro';
import { eq } from 'drizzle-orm';
import { z } from 'zod';
import { db } from '../../../db/client';
import { users } from '../../../db/schema';
import { requireAdmin } from '../../../lib/admin';
import {
  adminMutationKeyFromRequest,
  adminMutationResponse,
  executeAdminMutation,
} from '../../../lib/admin-mutations';
import { recordAuditStrict } from '../../../lib/audit';
import { requireSameOrigin } from '../../../lib/csrf';
import { safeJsonBody } from '../../../lib/validation';

const adminsSchema = z
  .object({
    email: z.string().email().max(320).optional(),
    userId: z.string().uuid().optional(),
    role: z.enum(['admin', 'user']).default('admin'),
  })
  .strict()
  .refine((value) => Boolean(value.userId) !== Boolean(value.email), {
    message: 'Provide exactly one of userId or email.',
  });

export const POST: APIRoute = async ({ request }) => {
  const originCheck = requireSameOrigin(request);
  if (originCheck) return originCheck;
  const actor = await requireAdmin(request);
  if (!actor) return new Response('Forbidden', { status: 403 });
  const parsed = await safeJsonBody(request, adminsSchema);
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
      const targetQuery = tx.update(users).set({ role: parsed.data.role });
      const matched = parsed.data.userId
        ? await targetQuery
            .where(eq(users.id, parsed.data.userId))
            .returning({ id: users.id })
        : await targetQuery
            .where(eq(users.email, parsed.data.email!))
            .returning({ id: users.id });
      const target = matched[0];
      if (!target)
        return {
          status: 404,
          body: {
            error: { type: 'user_not_found', message: 'User not found.' },
          },
        };
      await recordAuditStrict(tx, {
        actorUserId: actor.id,
        action: 'role.changed',
        targetType: 'user',
        targetId: target.id,
        meta: {
          role: parsed.data.role,
          email: parsed.data.email ?? null,
          idempotencyKey,
        },
      });
      return { body: { ok: true, userId: target.id, role: parsed.data.role } };
    },
  });
  return adminMutationResponse(result);
};
