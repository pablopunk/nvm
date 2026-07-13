import * as Sentry from '@sentry/astro';
import { drizzle } from 'drizzle-orm/node-postgres';
import { Pool } from 'pg';
import { log } from '../lib/log';
import * as schema from './schema';

export function createPostgresDb(connectionString?: string) {
  const pool = new Pool({ connectionString });
  pool.on('error', (error: unknown) => {
    log.error('postgres_pool_error', { error });
    Sentry.captureException(error);
  });
  return { db: drizzle(pool, { schema }), pool };
}
