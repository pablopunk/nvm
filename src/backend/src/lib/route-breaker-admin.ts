import { and, eq } from 'drizzle-orm';
import { db } from '../db/client';
import { auditLog, routeBreakers } from '../db/schema';

export async function reenableRouteBreaker(input: {
  actorUserId: string;
  routeSlot: string;
  provider: string;
  modelId: string;
}): Promise<void> {
  await db.transaction(async (tx) => {
    await tx.delete(routeBreakers).where(and(
      eq(routeBreakers.routeSlot, input.routeSlot),
      eq(routeBreakers.provider, input.provider),
      eq(routeBreakers.modelId, input.modelId),
    ));
    await tx.insert(auditLog).values({
      actorUserId: input.actorUserId,
      action: 'route_breaker.reenabled',
      targetType: 'route',
      targetId: `${input.routeSlot}:${input.provider}:${input.modelId}`,
      meta: { routeSlot: input.routeSlot, provider: input.provider, modelId: input.modelId },
    });
  });
}
