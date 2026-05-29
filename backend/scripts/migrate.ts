import 'dotenv/config';
import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { Pool, neonConfig } from '@neondatabase/serverless';
import { drizzle } from 'drizzle-orm/neon-serverless';
import { migrate } from 'drizzle-orm/neon-serverless/migrator';
import { sql } from 'drizzle-orm';
import ws from 'ws';

neonConfig.webSocketConstructor = ws;

const MIGRATIONS_DIR = path.resolve(import.meta.dirname, '../drizzle');
const BASELINE_TAG = '0000_remarkable_meltdown';
const BASELINE_WHEN = 1779990985460;

async function markBaselineAppliedIfPreexistingDb(db: ReturnType<typeof drizzle>) {
  await db.execute(sql`CREATE SCHEMA IF NOT EXISTS "drizzle"`);
  await db.execute(sql`
    CREATE TABLE IF NOT EXISTS "drizzle"."__drizzle_migrations" (
      id SERIAL PRIMARY KEY,
      hash text NOT NULL,
      created_at bigint
    )
  `);
  const { rows: existing } = await db.execute<{ count: string }>(
    sql`SELECT count(*)::text as count FROM "drizzle"."__drizzle_migrations"`,
  );
  if (Number(existing[0]?.count ?? '0') > 0) return;

  const { rows: pre } = await db.execute<{ exists: boolean }>(
    sql`SELECT to_regclass('public.users') IS NOT NULL AS exists`,
  );
  if (!pre[0]?.exists) return;

  const baselineSql = fs.readFileSync(path.join(MIGRATIONS_DIR, `${BASELINE_TAG}.sql`)).toString();
  const hash = crypto.createHash('sha256').update(baselineSql).digest('hex');
  await db.execute(sql`
    INSERT INTO "drizzle"."__drizzle_migrations" (hash, created_at)
    VALUES (${hash}, ${BASELINE_WHEN})
  `);
  console.log(`[migrate] marked baseline ${BASELINE_TAG} as already applied`);
}

const pool = new Pool({ connectionString: process.env.DATABASE_URL });
const db = drizzle(pool);

await markBaselineAppliedIfPreexistingDb(db);
await migrate(db, { migrationsFolder: MIGRATIONS_DIR });
console.log('[migrate] done');
await pool.end();
