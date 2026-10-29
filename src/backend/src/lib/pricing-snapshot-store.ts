import { eq, sql } from 'drizzle-orm';
import { pricingSnapshots } from '../db/schema';
import { env } from './env';
import { log } from './log';

const DAY_MS = 24 * 60 * 60 * 1000;

/**
 * The scheduled refresh runs daily. A validation inside this window is current and
 * admissible for every tier. Its slack covers one missed or late daily run.
 */
export const SNAPSHOT_CURRENT_MAX_AGE_MS = 36 * 60 * 60 * 1000;
/**
 * Hard bound on a durable last-known-valid price. Past it the price is refused for
 * every tier, so a long refresh outage never admits an arbitrarily old rate.
 */
export const SNAPSHOT_MAX_AGE_MS = 7 * DAY_MS;

export type PriceValidity = 'current' | 'stale';
export type ValidatedSnapshot<T> = { value: T; validatedAt: number };

export type SnapshotLookup<T> =
  | { status: 'current'; value: T }
  | { status: 'stale'; value: T }
  | { status: 'expired' }
  | { status: 'unverified' };

export function snapshotLookup<T>(
  entry: ValidatedSnapshot<T> | undefined,
  now = Date.now(),
): SnapshotLookup<T> {
  if (!entry) return { status: 'unverified' };
  const age = now - entry.validatedAt;
  if (age < SNAPSHOT_CURRENT_MAX_AGE_MS)
    return { status: 'current', value: entry.value };
  if (age < SNAPSHOT_MAX_AGE_MS) return { status: 'stale', value: entry.value };
  return { status: 'expired' };
}

/**
 * A validated price snapshot keyed by its catalog section or provider model.
 * `payload` is untrusted once read back: every reader re-validates its shape.
 */
export type PricingSnapshotRecord = {
  key: string;
  validatedAt: number;
  payload: unknown;
};

export type PricingSnapshotStore = {
  load(key: string): Promise<PricingSnapshotRecord | null>;
  save(record: PricingSnapshotRecord): Promise<void>;
  expire?(expiredAt: number): void;
};

/**
 * The Postgres store is absent without a database URL, so an unconfigured
 * environment keeps snapshots in memory only and never reaches for a connection.
 */
function databasePricingSnapshotStore(): PricingSnapshotStore {
  return {
    async load(key) {
      if (!env('DATABASE_URL')) return null;
      const { getDb } = await import('../db/client');
      const [row] = await getDb()
        .select()
        .from(pricingSnapshots)
        .where(eq(pricingSnapshots.key, key))
        .limit(1);
      return row
        ? { key, validatedAt: Number(row.validatedAt), payload: row.payload }
        : null;
    },
    async save(record) {
      if (!env('DATABASE_URL')) return;
      const { getDb } = await import('../db/client');
      // Only a strictly newer validation may replace a row, so a slow refresh
      // that finishes late cannot roll a durable snapshot backwards.
      await getDb()
        .insert(pricingSnapshots)
        .values({
          key: record.key,
          validatedAt: record.validatedAt,
          payload: record.payload,
        })
        .onConflictDoUpdate({
          target: pricingSnapshots.key,
          set: { validatedAt: record.validatedAt, payload: record.payload },
          setWhere: sql`${pricingSnapshots.validatedAt} < ${record.validatedAt}`,
        });
    },
  };
}

export function memoryPricingSnapshotStore(): PricingSnapshotStore {
  const records = new Map<string, PricingSnapshotRecord>();
  return {
    async load(key) {
      return records.get(key) ?? null;
    },
    async save(record) {
      const current = records.get(record.key);
      if (!current || current.validatedAt < record.validatedAt)
        records.set(record.key, record);
    },
    expire(expiredAt) {
      for (const [key, record] of records)
        records.set(key, { ...record, validatedAt: Math.min(record.validatedAt, expiredAt) });
    },
  };
}

let activeStore: PricingSnapshotStore = databasePricingSnapshotStore();

export function activePricingSnapshotStore(): PricingSnapshotStore {
  return activeStore;
}

/** A catalog that keeps its own validated copy, so tests age and clear it with the store. */
export type DurableCacheForTests = { expire(expiredAt: number): void; reset(): void };
const durableCachesForTests = new Set<DurableCacheForTests>();

export function registerDurableCacheForTests(cache: DurableCacheForTests) {
  durableCachesForTests.add(cache);
}

export function resetPricingSnapshotStoreForTests() {
  activeStore = memoryPricingSnapshotStore();
  for (const cache of durableCachesForTests) cache.reset();
}

export function expirePricingSnapshotStoreForTests(expiredAt: number) {
  activeStore.expire?.(expiredAt);
  for (const cache of durableCachesForTests) cache.expire(expiredAt);
}

export async function loadSnapshotRecord(
  key: string,
): Promise<PricingSnapshotRecord | null> {
  try {
    return await activeStore.load(key);
  } catch (error) {
    log.warn('pricing_snapshot_load_failed', { key, error: String(error) });
    return null;
  }
}

/**
 * Persists one validated snapshot. Returns false when the write failed, so the
 * refresh that produced it reports failure instead of claiming a durable seed.
 */
export async function saveSnapshotRecord(
  record: PricingSnapshotRecord,
): Promise<boolean> {
  try {
    await activeStore.save(record);
    return true;
  } catch (error) {
    log.warn('pricing_snapshot_save_failed', {
      key: record.key,
      error: String(error),
    });
    return false;
  }
}
