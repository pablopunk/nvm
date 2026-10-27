import { db } from '../db/client';
import { auditLog } from '../db/schema';

export type AuditExecutor = Pick<typeof db, 'insert'>;

export type AuditAction =
  | 'model.changed'
  | 'provider.changed'
  | 'role.changed'
  | 'credits.granted'
  | 'credits.adjusted'
  | 'route_breaker.reenabled'
  | 'free_spend_cap.changed'
  | 'waitlist.invite_queued'
  | 'waitlist.status_changed'
  | 'admin.mutation';

type AuditInput = {
  actorUserId: string | null;
  action: AuditAction;
  targetType?: string;
  targetId?: string;
  meta?: Record<string, unknown>;
};

/** Write audit data through the caller's transaction and let failures abort it. */
export async function recordAuditStrict(
  executor: AuditExecutor,
  input: AuditInput,
) {
  await executor.insert(auditLog).values({
    actorUserId: input.actorUserId,
    action: input.action,
    targetType: input.targetType ?? null,
    targetId: input.targetId ?? null,
    meta: input.meta ?? null,
  });
}

/** Strict non-transactional variant for mutations which own their transaction. */
export async function recordAuditStrictOnDb(input: AuditInput) {
  await recordAuditStrict(db, input);
}

/** Legacy best-effort API retained for non-transactional historical callers. */
export async function recordAudit(input: AuditInput) {
  try {
    await recordAuditStrictOnDb(input);
  } catch (err) {
    console.error('[audit] write failed', err);
  }
}
