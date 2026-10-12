import { createHash } from 'node:crypto';
import { and, eq } from 'drizzle-orm';
import { db } from '../db/client';
import { adminMutationResults } from '../db/schema';

type MutationTransaction = Parameters<Parameters<typeof db.transaction>[0]>[0];

export type AdminMutationReply = {
  status?: number;
  body: unknown;
};

export type AdminMutationResult = {
  status: number;
  body: unknown;
  replayed: boolean;
  conflict: boolean;
};

function canonicalize(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(canonicalize);
  if (value && typeof value === 'object') {
    return Object.fromEntries(
      Object.entries(value as Record<string, unknown>)
        .sort(([left], [right]) => left.localeCompare(right))
        .map(([key, child]) => [key, canonicalize(child)]),
    );
  }
  return value;
}

export function adminMutationPayloadHash(payload: unknown): string {
  const serialized = JSON.stringify(canonicalize(payload)) ?? 'null';
  return createHash('sha256').update(serialized).digest('hex');
}

export function adminMutationKeyFromRequest(request: Request): string | null {
  const key = request.headers.get('idempotency-key');
  if (!key || key.length > 36) return null;
  return /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(
    key,
  )
    ? key.toLowerCase()
    : null;
}

export function adminMutationError(
  status: number,
  type: string,
  message: string,
): AdminMutationResult {
  return {
    status,
    body: { error: { type, message } },
    replayed: false,
    conflict: false,
  };
}

export function adminMutationResponse(result: AdminMutationResult): Response {
  return Response.json(result.body, {
    status: result.status,
    headers: result.replayed ? { 'Idempotency-Replayed': 'true' } : undefined,
  });
}

export async function lookupAdminMutationResult(input: {
  actorUserId: string;
  idempotencyKey: string;
  payload: unknown;
}): Promise<AdminMutationResult | null> {
  const [saved] = await db
    .select()
    .from(adminMutationResults)
    .where(
      and(
        eq(adminMutationResults.actorUserId, input.actorUserId),
        eq(adminMutationResults.idempotencyKey, input.idempotencyKey),
      ),
    )
    .limit(1);
  if (!saved) return null;
  if (saved.payloadHash !== adminMutationPayloadHash(input.payload)) {
    return {
      status: 409,
      body: {
        error: {
          type: 'idempotency_conflict',
          message:
            'This Idempotency-Key was already used with a different request.',
        },
      },
      replayed: false,
      conflict: true,
    };
  }
  if (saved.responseStatus == null || saved.responseJson == null) {
    return {
      status: 409,
      body: {
        error: {
          type: 'mutation_in_progress',
          message:
            'The original admin action is still being committed. Retry with the same key.',
        },
      },
      replayed: false,
      conflict: true,
    };
  }
  return {
    status: saved.responseStatus,
    body: saved.responseJson,
    replayed: true,
    conflict: false,
  };
}

export async function executeAdminMutation(input: {
  actorUserId: string;
  idempotencyKey: string;
  payload: unknown;
  execute: (transaction: MutationTransaction) => Promise<AdminMutationReply>;
}): Promise<AdminMutationResult> {
  const payloadHash = adminMutationPayloadHash(input.payload);

  return db.transaction(async (tx) => {
    const [inserted] = await tx
      .insert(adminMutationResults)
      .values({
        actorUserId: input.actorUserId,
        idempotencyKey: input.idempotencyKey,
        payloadHash,
      })
      .onConflictDoNothing()
      .returning({ id: adminMutationResults.id });

    const [saved] = await tx
      .select()
      .from(adminMutationResults)
      .where(
        and(
          eq(adminMutationResults.actorUserId, input.actorUserId),
          eq(adminMutationResults.idempotencyKey, input.idempotencyKey),
        ),
      )
      .for('update');
    if (!saved)
      throw new Error('Admin mutation idempotency record disappeared');
    if (saved.payloadHash !== payloadHash) {
      return {
        status: 409,
        body: {
          error: {
            type: 'idempotency_conflict',
            message:
              'This Idempotency-Key was already used with a different request.',
          },
        },
        replayed: false,
        conflict: true,
      };
    }

    if (!inserted) {
      if (saved.responseStatus == null || saved.responseJson == null) {
        return {
          status: 409,
          body: {
            error: {
              type: 'mutation_in_progress',
              message:
                'The original admin action is still being committed. Retry with the same key.',
            },
          },
          replayed: false,
          conflict: true,
        };
      }
      return {
        status: saved.responseStatus,
        body: saved.responseJson,
        replayed: true,
        conflict: false,
      };
    }

    const reply = await input.execute(tx);
    const status = reply.status ?? 200;
    await tx
      .update(adminMutationResults)
      .set({ responseStatus: status, responseJson: reply.body })
      .where(eq(adminMutationResults.id, saved.id));
    return { status, body: reply.body, replayed: false, conflict: false };
  });
}
