import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { sql } from 'drizzle-orm';
import type { drizzle } from 'drizzle-orm/node-postgres';
import { migrate } from 'drizzle-orm/node-postgres/migrator';
import { createPostgresDb } from '../src/db/postgres';

const MIGRATIONS_DIR = path.resolve(import.meta.dirname, '../drizzle');
const BASELINE_TAG = '0000_remarkable_meltdown';
const BASELINE_WHEN = 1779990985460;

export async function markBaselineAppliedIfPreexistingDb(
  database: ReturnType<typeof drizzle>,
) {
  await database.execute(sql`CREATE SCHEMA IF NOT EXISTS "drizzle"`);
  await database.execute(sql`
    CREATE TABLE IF NOT EXISTS "drizzle"."__drizzle_migrations" (
      id SERIAL PRIMARY KEY,
      hash text NOT NULL,
      created_at bigint
    )
  `);
  const { rows: existing } = await database.execute<{ count: string }>(
    sql`SELECT count(*)::text as count FROM "drizzle"."__drizzle_migrations"`,
  );
  if (Number(existing[0]?.count ?? '0') > 0) return;
  const { rows: pre } = await database.execute<{ exists: boolean }>(
    sql`SELECT to_regclass('public.users') IS NOT NULL AS exists`,
  );
  if (!pre[0]?.exists) return;
  const baselineSql = fs.readFileSync(
    path.join(MIGRATIONS_DIR, `${BASELINE_TAG}.sql`),
  );
  const hash = crypto.createHash('sha256').update(baselineSql).digest('hex');
  await database.execute(sql`
    INSERT INTO "drizzle"."__drizzle_migrations" (hash, created_at)
    VALUES (${hash}, ${BASELINE_WHEN})
  `);
}

export async function runPostgresMigrations(connectionString: string) {
  const { db, pool } = createPostgresDb(connectionString);
  try {
    await markBaselineAppliedIfPreexistingDb(db);
    await migrate(db, { migrationsFolder: MIGRATIONS_DIR });
  } finally {
    await pool.end();
  }
}

if (import.meta.url === `file://${process.argv[1]}`) {
  const connectionString = process.env.DATABASE_URL;
  if (!connectionString)
    throw new Error('DATABASE_URL is required for Postgres migrations');
  await runPostgresMigrations(connectionString);
}
