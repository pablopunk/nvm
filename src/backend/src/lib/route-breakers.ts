import { and, eq } from 'drizzle-orm';
import { db } from '../db/client';
import { routeBreakers } from '../db/schema';

export type RouteCandidate = { providerId: string; modelId: string };

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

export async function healthyRouteCandidates(
  routeSlot: string,
  candidates: RouteCandidate[],
  isTripped?: (slot: string, provider: string, modelId: string) => Promise<boolean>,
): Promise<RouteCandidate[]> {
  if (!isTripped) {
    const breakers = await db.select({ provider: routeBreakers.provider, modelId: routeBreakers.modelId })
      .from(routeBreakers).where(eq(routeBreakers.routeSlot, routeSlot));
    const tripped = new Set(breakers.map((breaker) => `${breaker.provider}\0${breaker.modelId}`));
    return candidates.filter((candidate) => !tripped.has(`${candidate.providerId}\0${candidate.modelId}`));
  }
  const healthy: RouteCandidate[] = [];
  for (const candidate of candidates) {
    if (!(await isTripped(routeSlot, candidate.providerId, candidate.modelId))) healthy.push(candidate);
  }
  return healthy;
}
