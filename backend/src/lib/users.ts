import { eq, sql } from 'drizzle-orm';
import { db } from '../db/client';
import { users, creditLedger } from '../db/schema';

const FREE_SIGNUP_GRANT = 100;

export async function upsertUserWithFreeGrant(input: {
  workosUserId: string;
  email: string;
}) {
  return db.transaction(async (tx) => {
    const existing = await tx
      .select()
      .from(users)
      .where(eq(users.workosUserId, input.workosUserId))
      .limit(1);
    if (existing[0]) return existing[0];

    const [created] = await tx
      .insert(users)
      .values({ workosUserId: input.workosUserId, email: input.email })
      .returning();

    await tx.insert(creditLedger).values({
      userId: created.id,
      delta: FREE_SIGNUP_GRANT,
      reason: 'grant_signup',
    });

    return created;
  });
}

export async function getBalance(userId: string): Promise<number> {
  const rows = await db
    .select({ balance: sql<number>`coalesce(sum(${creditLedger.delta}), 0)::int` })
    .from(creditLedger)
    .where(eq(creditLedger.userId, userId));
  return rows[0]?.balance ?? 0;
}
