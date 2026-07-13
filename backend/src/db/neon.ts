import { neonConfig, Pool } from '@neondatabase/serverless';
import * as Sentry from '@sentry/astro';
import { drizzle } from 'drizzle-orm/neon-serverless';
import ws from 'ws';
import { log } from '../lib/log';
import * as schema from './schema';

neonConfig.webSocketConstructor = ws;

function isAdministrativeTermination(error: unknown) {
  return (
    error instanceof Error &&
    error.message.includes(
      'terminating connection due to administrator command',
    )
  );
}

export function createNeonDb(connectionString?: string) {
  const pool = new Pool({ connectionString });
  pool.on('error', (error: unknown) => {
    if (isAdministrativeTermination(error)) {
      log.warn('neon_idle_connection_terminated', { error });
      return;
    }
    log.error('neon_pool_error', { error });
    Sentry.captureException(error);
  });
  return { db: drizzle(pool, { schema }), pool };
}
