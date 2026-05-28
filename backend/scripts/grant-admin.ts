import 'dotenv/config';
import { Pool, neonConfig } from '@neondatabase/serverless';
import { drizzle } from 'drizzle-orm/neon-serverless';
import { eq } from 'drizzle-orm';
import ws from 'ws';
import * as schema from '../src/db/schema';

neonConfig.webSocketConstructor = ws;
const pool = new Pool({ connectionString: process.env.DATABASE_URL });
const db = drizzle(pool, { schema });

const email = process.argv[2];
if (!email) {
  console.error('Usage: pnpm tsx scripts/grant-admin.ts <email>');
  process.exit(1);
}

const result = await db
  .update(schema.users)
  .set({ role: 'admin' })
  .where(eq(schema.users.email, email))
  .returning({ id: schema.users.id, email: schema.users.email, role: schema.users.role });

if (!result.length) console.error('User not found:', email);
else console.log('Granted admin:', result[0]);

await pool.end();
