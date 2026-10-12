import { and, eq } from 'drizzle-orm';
import { db } from '../db/client';
import { routeBreakers } from '../db/schema';
import { recordAuditStrict } from './audit';

export async function reenableRouteBreaker(
  input: {
    actorUserId: string;
    routeSlot: string;
    provider: string;
    modelId: string;
  },
  executor?: Parameters<Parameters<typeof db.transaction>[0]>[0],
): Promise<void> {
  if (!executor) {
    await db.transaction((tx) => reenableRouteBreaker(input, tx));
    return;
  }
  await executor
    .delete(routeBreakers)
    .where(
      and(
        eq(routeBreakers.routeSlot, input.routeSlot),
        eq(routeBreakers.provider, input.provider),
        eq(routeBreakers.modelId, input.modelId),
      ),
    );
  await recordAuditStrict(executor, {
    actorUserId: input.actorUserId,
    action: 'route_breaker.reenabled',
    targetType: 'route',
    targetId: `${input.routeSlot}:${input.provider}:${input.modelId}`,
    meta: {
      routeSlot: input.routeSlot,
      provider: input.provider,
      modelId: input.modelId,
    },
  });
}
