import type { APIRoute } from 'astro';
import { eq } from 'drizzle-orm';
import { db } from '../../../db/client';
import { subscriptions } from '../../../db/schema';
import { getCurrentUser } from '../../../lib/admin';
import { billingCatalog } from '../../../lib/billing';
import { dashboardRead } from '../../../lib/dashboard-read';

export const GET: APIRoute = async ({ request }) => {
  const user = await getCurrentUser(request);
  if (!user) return new Response('Unauthorized', { status: 401 });
  const [subscription] = await dashboardRead(() => db.select().from(subscriptions).where(eq(subscriptions.userId, user.id)).limit(1));
  return Response.json({
    subscription: subscription ? { status: subscription.status, currentPeriodEnd: subscription.currentPeriodEnd } : null,
    hasCustomer: Boolean(user.stripeCustomerId),
    catalog: billingCatalog(),
  }, { headers: { 'Cache-Control': 'private, no-store' } });
};
