import { and, eq, isNotNull, isNull } from 'drizzle-orm';
import { db, type Database } from '../db/client';
import { deviceCodes, users } from '../db/schema';
import { createApiToken } from './tokens';

export type DeviceExchangeResult =
  | { status: 'expired' }
  | { status: 'pending' }
  | { status: 'consumed' }
  | { status: 'missing_user' }
  | {
      status: 'ok';
      token: string;
      user: { email: string; role: string };
    };

type DeviceAuthTransaction = Parameters<
  Parameters<Database['transaction']>[0]
>[0];

type DeviceExchangeDependencies = {
  createTokenInTransaction?: (
    transaction: DeviceAuthTransaction,
    userId: string,
    name: string,
  ) => Promise<{ token: string }>;
  now?: () => Date;
};

export async function exchangeApprovedDeviceCode(
  code: string,
  dependencies: DeviceExchangeDependencies = {},
): Promise<DeviceExchangeResult> {
  const createTokenInTransaction =
    dependencies.createTokenInTransaction ??
    ((transaction, userId, name) =>
      createApiToken(userId, name, transaction));

  return db.transaction(async (transaction) => {
    const [deviceCode] = await transaction
      .select()
      .from(deviceCodes)
      .where(eq(deviceCodes.code, code))
      .limit(1)
      .for('update');
    const now = dependencies.now?.() ?? new Date();

    if (!deviceCode || deviceCode.expiresAt <= now) {
      return { status: 'expired' };
    }
    if (deviceCode.consumedAt) return { status: 'consumed' };
    if (!deviceCode.approvedAt) return { status: 'pending' };
    if (!deviceCode.userId) return { status: 'missing_user' };

    const [user] = await transaction
      .select()
      .from(users)
      .where(eq(users.id, deviceCode.userId))
      .limit(1);
    if (!user) return { status: 'missing_user' };

    const consumed = await transaction
      .update(deviceCodes)
      .set({ consumedAt: now })
      .where(
        and(
          eq(deviceCodes.code, code),
          isNull(deviceCodes.consumedAt),
          isNotNull(deviceCodes.approvedAt),
        ),
      )
      .returning({ code: deviceCodes.code });
    if (consumed.length === 0) return { status: 'consumed' };

    const created = await createTokenInTransaction(
      transaction,
      user.id,
      deviceCode.deviceLabel,
    );
    return {
      status: 'ok',
      token: created.token,
      user: { email: user.email, role: user.role },
    };
  });
}
