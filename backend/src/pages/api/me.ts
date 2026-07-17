import type { APIRoute } from 'astro';
import { eq, desc } from 'drizzle-orm';
import { getSessionFromCookies } from '../../lib/workos';
import { db } from '../../db/client';
import { users, usage, subscriptions, creditLedger, apiTokens, deviceCodes } from '../../db/schema';
import { ensureMonthlyFreeCredits, getBalance } from '../../lib/users';
import { recordAudit } from '../../lib/audit';
import { log } from '../../lib/log';
import { setStripeForTests } from '../../lib/billing';

type MeDependencies = {
  getSession: typeof getSessionFromCookies;
  getUser: (workosUserId: string) => Promise<any>;
  ensureCredits: typeof ensureMonthlyFreeCredits;
  getBalance: typeof getBalance;
  getRecentUsage: (userId: string) => Promise<any[]>;
};

export function createMeHandler(overrides: Partial<MeDependencies> = {}): APIRoute {
  const dependencies: MeDependencies = {
    getSession: getSessionFromCookies,
    getUser: async (workosUserId) => (await db.select().from(users).where(eq(users.workosUserId, workosUserId)).limit(1))[0],
    ensureCredits: ensureMonthlyFreeCredits,
    getBalance,
    getRecentUsage: (userId) => db.select().from(usage).where(eq(usage.userId, userId)).orderBy(desc(usage.createdAt)).limit(10),
    ...overrides,
  };

  return async ({ request }) => {
  const session = await dependencies.getSession(request.headers.get('cookie'));
  if (!session) return new Response('Unauthorized', { status: 401 });

  const user = await dependencies.getUser(session.user.id);
  if (!user) return new Response('Unknown user', { status: 404 });
  await dependencies.ensureCredits(user.id);

  const [balance, recentUsage] = await Promise.all([
    dependencies.getBalance(user.id),
    dependencies.getRecentUsage(user.id),
  ]);

  return Response.json({
    email: user.email,
    plan: user.plan,
    balance,
    recentUsage,
  });
  };
}

export const GET = createMeHandler();

function stripeClientForDelete() {
  const key = process.env.STRIPE_SECRET_KEY;
  if (!key) {
    try {
      const Stripe = require('stripe');
      return new Stripe.default(key || 'sk_test_placeholder');
    } catch {
      return null;
    }
  }
  try {
    const Stripe = require('stripe');
    return new Stripe.default(key);
  } catch {
    return null;
  }
}

export const DELETE: APIRoute = async ({ request }) => {
  const session = await getSessionFromCookies(request.headers.get('cookie'));
  if (!session) return new Response('Unauthorized', { status: 401 });

  const [user] = await db
    .select()
    .from(users)
    .where(eq(users.workosUserId, session.user.id))
    .limit(1);
  if (!user) return new Response('Unknown user', { status: 404 });

  const userId = user.id;
  const email = user.email;

  const [sub] = await db
    .select()
    .from(subscriptions)
    .where(eq(subscriptions.userId, userId))
    .limit(1);

  if (sub) {
    const stripe = stripeClientForDelete();
    if (stripe) {
      try {
        await stripe.subscriptions.cancel(sub.stripeSubId);
        log.info('account_delete_stripe_subscription_canceled', {
          user_id: userId,
          stripe_sub_id: sub.stripeSubId,
        });
      } catch (err) {
        log.warn('account_delete_stripe_cancel_failed', {
          user_id: userId,
          error: (err as Error).message,
        });
      }
    }
  }

  if (user.stripeCustomerId) {
    const stripe = stripeClientForDelete();
    if (stripe) {
      try {
        await stripe.customers.update(user.stripeCustomerId, {
          email: '',
          name: '',
          phone: '',
          description: '[Account deleted — PII scrubbed per GDPR request]',
          address: null,
          shipping: null,
          metadata: {},
        });
        log.info('account_delete_stripe_customer_scrubbed', {
          user_id: userId,
          stripe_customer_id: user.stripeCustomerId,
        });
      } catch (err) {
        log.warn('account_delete_stripe_scrub_failed', {
          user_id: userId,
          error: (err as Error).message,
        });
      }
    }
    /*
      HUMAN-REVIEW: Stripe data handling decision per Plan-Reviewer.
      choice: retain-and-scrub — cancel subscription, blank PII fields,
      keep customer id for tax/accounting. Pending human legal review.
    */
  }

  await db.delete(users).where(eq(users.id, userId));

  await recordAudit({
    actorUserId: userId,
    action: 'model.changed',
    targetType: 'user',
    targetId: userId,
    meta: {
      action: 'account_deleted',
      email,
      stripeCustomerId: user.stripeCustomerId,
      subscriptionCanceled: Boolean(sub),
    },
  });

  log.info('account_deleted', { user_id: userId, email });

  return Response.json({ ok: true });
};
