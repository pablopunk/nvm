import { eq } from 'drizzle-orm';
import { db } from '../db/client';
import { users } from '../db/schema';
import { env } from './env';
import { getSessionFromCookies } from './workos';

export type AppUser = typeof users.$inferSelect;

export async function getCurrentUser(
  request: Request,
): Promise<AppUser | null> {
  const session = await getSessionFromCookies(request.headers.get('cookie'));
  if (!session) return null;
  const [user] = await db
    .select()
    .from(users)
    .where(eq(users.workosUserId, session.user.id))
    .limit(1);
  return user ?? null;
}

export function isAdmin(user: AppUser | null | undefined): user is AppUser {
  return !!user && user.role === 'admin';
}

export function isBillingPriceApprover(
  user: Pick<AppUser, 'role' | 'workosUserId'> | null | undefined,
  configuredWorkosUserId = env('BILLING_PRICE_APPROVER_WORKOS_USER_ID')?.trim(),
): boolean {
  return Boolean(
    configuredWorkosUserId &&
      user?.role === 'admin' &&
      user.workosUserId === configuredWorkosUserId,
  );
}

export async function requireAdmin(request: Request): Promise<AppUser | null> {
  const user = await getCurrentUser(request);
  return isAdmin(user) ? user : null;
}
