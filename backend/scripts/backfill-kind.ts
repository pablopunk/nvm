import 'dotenv/config';
import { Pool, neonConfig } from '@neondatabase/serverless';
import { drizzle } from 'drizzle-orm/neon-serverless';
import { sql } from 'drizzle-orm';
import ws from 'ws';
import * as schema from '../src/db/schema';

neonConfig.webSocketConstructor = ws;
const pool = new Pool({ connectionString: process.env.DATABASE_URL });
const db = drizzle(pool, { schema });

const r = await db.execute(sql`
  update credit_ledger
  set kind = 'free'
  where reason in ('grant_signup', 'ai_usage')
`);
console.log('Backfilled rows:', r.rowCount);
await pool.end();
