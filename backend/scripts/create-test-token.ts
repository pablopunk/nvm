import 'dotenv/config';
import { Pool, neonConfig } from '@neondatabase/serverless';
import { drizzle } from 'drizzle-orm/neon-serverless';
import { eq } from 'drizzle-orm';
import { randomBytes, createHash } from 'node:crypto';
import ws from 'ws';
import * as schema from '../src/db/schema';

neonConfig.webSocketConstructor = ws;
const pool = new Pool({ connectionString: process.env.DATABASE_URL });
const db = drizzle(pool, { schema });

const email = process.argv[2];
if (!email) {
  console.error('usage: tsx scripts/create-test-token.ts <email>');
  process.exit(1);
}

const [user] = await db.select().from(schema.users).where(eq(schema.users.email, email)).limit(1);
if (!user) {
  console.error(`No user found for ${email}`);
  process.exit(1);
}

const secret = randomBytes(32).toString('base64url');
const token = `nvm_pat_${secret}`;
const hash = createHash('sha256').update(token).digest('hex');
const prefix = token.slice(0, 'nvm_pat_'.length + 6);

await db.insert(schema.apiTokens).values({
  userId: user.id,
  tokenHash: hash,
  prefix,
  name: 'test-script',
});

console.log(token);
await pool.end();
