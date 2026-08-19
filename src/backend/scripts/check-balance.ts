import 'dotenv/config';
import { Pool, neonConfig } from '@neondatabase/serverless';
import { drizzle } from 'drizzle-orm/neon-serverless';
import { eq, desc, sql } from 'drizzle-orm';
import ws from 'ws';
import * as schema from '../src/db/schema';

neonConfig.webSocketConstructor = ws;
const pool = new Pool({ connectionString: process.env.DATABASE_URL });
const db = drizzle(pool, { schema });

const email = process.argv[2];
const [user] = await db.select().from(schema.users).where(eq(schema.users.email, email)).limit(1);
const [bal] = await db
  .select({ b: sql<number>`coalesce(sum(${schema.creditLedger.delta}), 0)::int` })
  .from(schema.creditLedger)
  .where(eq(schema.creditLedger.userId, user.id));
const ledger = await db
  .select()
  .from(schema.creditLedger)
  .where(eq(schema.creditLedger.userId, user.id))
  .orderBy(desc(schema.creditLedger.createdAt))
  .limit(10);
const usageRows = await db
  .select()
  .from(schema.usage)
  .where(eq(schema.usage.userId, user.id))
  .orderBy(desc(schema.usage.createdAt))
  .limit(10);

console.log('balance:', bal.b);
console.log('recent ledger:', ledger);
console.log('recent usage:', usageRows);
await pool.end();
