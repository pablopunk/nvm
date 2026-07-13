import assert from 'node:assert/strict';
import crypto from 'node:crypto';
import { and, eq, isNull } from 'drizzle-orm';
import { Pool } from 'pg';
import type { Database } from '../src/db/client';
import { createPostgresDb } from '../src/db/postgres';
import {
  apiTokens,
  creditLedger,
  deviceCodes,
  emailOutbox,
  emailSuppressions,
  invites,
  requestDedup,
  stripeEvents,
  users,
} from '../src/db/schema';
import { createInvite, createInviteIntent, readInviteIntentCookie } from '../src/lib/waitlist';
import { createUserFromInviteIntent, InviteRequiredError } from '../src/lib/users';
import { leaseOutbox, processProviderEvent } from '../src/lib/email';
import { runPostgresMigrations } from './migrate-postgres';

if (process.env.NVM_DB_DRIVER !== 'postgres')
  throw new Error('Postgres integration requires NVM_DB_DRIVER=postgres');

const adminUrl =
  process.env.NVM_TEST_ADMIN_DATABASE_URL ||
  (() => {
    throw new Error('NVM_TEST_ADMIN_DATABASE_URL is required');
  })();
const productionUrl = process.env.DATABASE_URL;
const admin = new URL(adminUrl);
if (!['localhost', '127.0.0.1', '::1', 'postgres'].includes(admin.hostname))
  throw new Error(
    `Test admin host is not local/service-scoped: ${admin.hostname}`,
  );
if (productionUrl && new URL(productionUrl).toString() === admin.toString())
  throw new Error('Test admin URL must not equal DATABASE_URL');

const worker = (process.env.TEST_WORKER_INDEX || '0').replace(
  /[^a-zA-Z0-9_]/g,
  '_',
);
const databaseName = `nvm_test_${worker}_${crypto.randomBytes(6).toString('hex')}`;
if (!/^nvm_test_[a-zA-Z0-9_]+$/.test(databaseName))
  throw new Error('Unsafe test database name');

function quotedIdentifier(value: string) {
  return `"${value.replaceAll('"', '""')}"`;
}

function databaseUrl(name: string) {
  const url = new URL(adminUrl);
  url.pathname = `/${name}`;
  return url.toString();
}

async function fingerprint(pool: Pool) {
  const { rows } = await pool.query(
    `SELECT table_schema, table_name, column_name, data_type, is_nullable
       FROM information_schema.columns
      WHERE table_schema = 'public'
      ORDER BY table_schema, table_name, ordinal_position`,
  );
  return crypto.createHash('sha256').update(JSON.stringify(rows)).digest('hex');
}

async function migrationMetadata(pool: Pool) {
  const { rows } = await pool.query(
    'SELECT hash, created_at FROM "drizzle"."__drizzle_migrations" ORDER BY id',
  );
  return rows;
}

async function expectUniqueViolation(action: () => Promise<unknown>) {
  await assert.rejects(action, (error: unknown) => {
    if (!error || typeof error !== 'object') return false;
    const record = error as { code?: unknown; cause?: { code?: unknown } };
    return record.code === '23505' || record.cause?.code === '23505';
  });
}

async function runAssertions() {
  process.env.DATABASE_URL = databaseUrl(databaseName);
  const { withTestDb, closeDefaultDb } = await import('../src/db/client');
  const { db, pool } = createPostgresDb(databaseUrl(databaseName));
  try {
    await withTestDb(db as unknown as Database, async () => {
      const [user] = await db
        .insert(users)
        .values({
          workosUserId: `integration-${databaseName}`,
          email: `${databaseName}@example.test`,
        })
        .returning();
      assert.ok(user);

      const tokenHash = `hash-${databaseName}`;
      await db.insert(apiTokens).values({
        userId: user.id,
        tokenHash,
        prefix: 'nvm_pat_test',
        name: 'integration',
      });
      await expectUniqueViolation(() =>
        db.insert(apiTokens).values({
          userId: user.id,
          tokenHash,
          prefix: 'nvm_pat_test',
          name: 'duplicate',
        }),
      );
      const tokenRows = await db
        .select()
        .from(apiTokens)
        .where(eq(apiTokens.tokenHash, tokenHash));
      assert.equal(
        tokenRows.length,
        1,
        'token_hash uniqueness must leave one row',
      );

      const duplicateEvent = `evt-${databaseName}`;
      const firstEvent = await db
        .insert(stripeEvents)
        .values({ eventId: duplicateEvent, type: 'integration.test' })
        .onConflictDoNothing()
        .returning();
      const secondEvent = await db
        .insert(stripeEvents)
        .values({ eventId: duplicateEvent, type: 'integration.test' })
        .onConflictDoNothing()
        .returning();
      assert.equal(firstEvent.length, 1);
      assert.equal(
        secondEvent.length,
        0,
        'stripe event idempotency must ignore duplicate event_id',
      );
      const rollbackEvent = `evt-rollback-${databaseName}`;
      await assert.rejects(
        db.transaction(async (tx) => {
          await tx
            .insert(stripeEvents)
            .values({ eventId: rollbackEvent, type: 'integration.rollback' });
          await tx.insert(creditLedger).values({
            userId: user.id,
            delta: 100,
            reason: 'integration.rollback',
            refId: rollbackEvent,
          });
          throw new Error('forced integration rollback');
        }),
      );
      assert.equal(
        (
          await db
            .select()
            .from(stripeEvents)
            .where(eq(stripeEvents.eventId, rollbackEvent))
        ).length,
        0,
      );
      assert.equal(
        (
          await db
            .select()
            .from(creditLedger)
            .where(eq(creditLedger.refId, rollbackEvent))
        ).length,
        0,
      );

      const dedupKey = `request-${databaseName}`;
      await assert.rejects(
        db.transaction(async (tx) => {
          await tx.insert(requestDedup).values({
            userId: user.id,
            idempotencyKey: dedupKey,
            requestId: `req-${databaseName}`,
          });
          throw new Error('forced request-dedup rollback');
        }),
      );
      assert.equal(
        (
          await db
            .select()
            .from(requestDedup)
            .where(
              and(
                eq(requestDedup.userId, user.id),
                eq(requestDedup.idempotencyKey, dedupKey),
              ),
            )
        ).length,
        0,
      );

      const code = `device-${databaseName}`;
      await db.insert(deviceCodes).values({
        code,
        userId: user.id,
        deviceLabel: 'integration',
        approvedAt: new Date(),
        expiresAt: new Date(Date.now() + 60_000),
      });
      const exchanges = await Promise.all(
        [1, 2].map(() =>
          db.transaction(async (tx) => {
            const updated = await tx
              .update(deviceCodes)
              .set({ consumedAt: new Date() })
              .where(
                and(eq(deviceCodes.code, code), isNull(deviceCodes.consumedAt)),
              )
              .returning({ code: deviceCodes.code });
            return updated.length === 1;
          }),
        ),
      );
      assert.deepEqual(
        exchanges.sort(),
        [false, true],
        'concurrent device exchange must have one winner',
      );

      const concurrentEmail = `concurrent-${databaseName}@example.test`;
      const concurrentInvites = await Promise.all(
        [1, 2].map(() => createInvite({ email: concurrentEmail })),
      );
      assert.equal(
        (await db.select().from(invites).where(eq(invites.email, concurrentEmail))).length,
        1,
        'concurrent issuance must leave one active invite',
      );
      assert.equal(
        concurrentInvites.filter((item) => !item.existing).length,
        1,
        'concurrent issuance must have one creator',
      );

      const leaseInvite = await createInvite({ email: `lease-${databaseName}@example.test` });
      assert.ok(leaseInvite.invite);
      const [leaseRow] = await db
        .select()
        .from(emailOutbox)
        .where(eq(emailOutbox.inviteId, leaseInvite.invite.id));
      assert.ok(leaseRow);
      const expiredLease = new Date(Date.now() - 60_000);
      await db.update(emailOutbox).set({
        status: 'sending',
        availableAt: expiredLease,
        leaseExpiresAt: expiredLease,
        leaseOwner: 'crashed-worker',
      }).where(eq(emailOutbox.id, leaseRow.id));
      const reclaimed = await leaseOutbox(10, crypto.randomUUID());
      assert.equal(reclaimed.some((row) => row.id === leaseRow.id), true, 'expired sending lease must be reclaimable');
      assert.ok(reclaimed.find((row) => row.id === leaseRow.id)?.leaseOwner);

      for (const [index, eventType] of ['email.bounced', 'email.complained'].entries()) {
        const recipient = `suppressed-${index}-${databaseName}@example.test`;
        const issued = await createInvite({ email: recipient });
        const [providerRow] = await db.select().from(emailOutbox).where(eq(emailOutbox.inviteId, issued.invite.id));
        const [laterRow] = await db.insert(emailOutbox).values({
          inviteId: issued.invite.id,
          recipient,
          tokenCiphertext: 'test-token',
          idempotencyKey: `later/${databaseName}/${index}`,
        }).returning();
        await db.update(emailOutbox).set({ providerMessageId: `provider-${databaseName}-${index}` }).where(eq(emailOutbox.id, providerRow.id));
        await processProviderEvent({
          id: `provider-event-${databaseName}-${index}`,
          type: eventType,
          messageId: `provider-${databaseName}-${index}`,
          raw: JSON.stringify({ type: eventType, data: { email_id: `provider-${databaseName}-${index}` } }),
        });
        assert.equal((await db.select().from(emailSuppressions).where(eq(emailSuppressions.email, recipient))).length, 1);
        assert.equal((await db.select().from(emailOutbox).where(eq(emailOutbox.id, laterRow.id)))[0]?.status, 'cancelled');
      }

      const redemptionEmail = `redemption-${databaseName}@example.test`;
      const redemptionInvite = await createInvite({ email: redemptionEmail });
      const intent = await createInviteIntent(redemptionInvite.token!);
      assert.ok(intent);
      const cookie = readInviteIntentCookie(intent.cookie);
      assert.ok(cookie);
      const redeemedUser = await createUserFromInviteIntent({
        intentId: cookie.id,
        nonce: cookie.nonce,
        workosUserId: `redemption-${databaseName}`,
        email: redemptionEmail,
      });
      assert.equal(redeemedUser.email, redemptionEmail);
      assert.equal((await db.select().from(invites).where(eq(invites.id, redemptionInvite.invite.id)))[0]?.status, 'redeemed');
      assert.equal((await db.select().from(creditLedger).where(eq(creditLedger.userId, redeemedUser.id))).length, 1);
      await assert.rejects(
        () => createUserFromInviteIntent({ intentId: cookie.id, nonce: cookie.nonce, workosUserId: `second-${databaseName}`, email: redemptionEmail }),
        (error: unknown) => error instanceof InviteRequiredError,
        'invite intent must be one-time',
      );
    });
  } finally {
    await pool.end();
    await closeDefaultDb();
  }
}

const adminPool = new Pool({ connectionString: adminUrl });
const targetUrl = databaseUrl(databaseName);
let targetPool: Pool | undefined;
let cleanupError: unknown;
let cleanupVerified = false;
let result:
  | {
      databaseName: string;
      firstFingerprint: string;
      migrationCount: number;
      assertions: string;
    }
  | undefined;
try {
  await adminPool.query(`CREATE DATABASE ${quotedIdentifier(databaseName)}`);
  await runPostgresMigrations(targetUrl);
  targetPool = new Pool({ connectionString: targetUrl });
  const firstFingerprint = await fingerprint(targetPool);
  const firstMetadata = await migrationMetadata(targetPool);
  await targetPool.end();
  targetPool = undefined;
  await runPostgresMigrations(targetUrl);
  targetPool = new Pool({ connectionString: targetUrl });
  assert.equal(
    await fingerprint(targetPool),
    firstFingerprint,
    'second migration changed schema fingerprint',
  );
  assert.deepEqual(
    await migrationMetadata(targetPool),
    firstMetadata,
    'second migration changed migration metadata',
  );
  await targetPool.end();
  targetPool = undefined;
  await runAssertions();
  result = {
    databaseName,
    firstFingerprint,
    migrationCount: firstMetadata.length,
    assertions: 'passed',
  };
} finally {
  await targetPool?.end().catch(() => {});
  await adminPool
    .query(
      `DROP DATABASE IF EXISTS ${quotedIdentifier(databaseName)} WITH (FORCE)`,
    )
    .catch((error) => {
      cleanupError = error;
    });
  if (!cleanupError) {
    const { rows } = await adminPool.query(
      'SELECT 1 FROM pg_database WHERE datname = $1',
      [databaseName],
    );
    cleanupVerified = rows.length === 0;
    if (!cleanupVerified)
      cleanupError = new Error(
        `Test database still exists after cleanup: ${databaseName}`,
      );
  }
  await adminPool.end().catch(() => {});
}
if (cleanupError) throw cleanupError;
if (!result) throw new Error('Postgres integration did not produce a result');
console.log(
  JSON.stringify({
    ...result,
    cleanup: cleanupVerified ? 'verified' : 'failed',
  }),
);
