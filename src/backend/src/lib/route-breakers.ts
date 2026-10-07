import { and, eq } from 'drizzle-orm';
import { db } from '../db/client';
import { routeBreakers } from '../db/schema';

export async function isRouteTripped(
  routeSlot: string,
  provider: string,
  modelId: string,
): Promise<boolean> {
  const [breaker] = await db
    .select({ provider: routeBreakers.provider })
    .from(routeBreakers)
    .where(
      and(
        eq(routeBreakers.routeSlot, routeSlot),
        eq(routeBreakers.provider, provider),
        eq(routeBreakers.modelId, modelId),
      ),
    )
    .limit(1);
  return Boolean(breaker);
}
