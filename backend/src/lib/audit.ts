import { db } from '../db/client';
import { auditLog } from '../db/schema';

export type AuditAction =
  | 'model.changed'
  | 'provider.changed'
  | 'role.changed'
  | 'credits.granted';

export async function recordAudit(input: {
  actorUserId: string | null;
  action: AuditAction;
  targetType?: string;
  targetId?: string;
  meta?: Record<string, unknown>;
}) {
  try {
    await db.insert(auditLog).values({
      actorUserId: input.actorUserId,
      action: input.action,
      targetType: input.targetType ?? null,
      targetId: input.targetId ?? null,
      meta: input.meta ?? null,
    });
  } catch (err) {
    console.error('[audit] write failed', err);
  }
}
