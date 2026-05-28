import { Pool, neonConfig } from '@neondatabase/serverless';
import { drizzle } from 'drizzle-orm/neon-serverless';
import ws from 'ws';
import * as schema from './schema';

neonConfig.webSocketConstructor = ws;

const pool = new Pool({ connectionString: import.meta.env.DATABASE_URL });
export const db = drizzle(pool, { schema });
