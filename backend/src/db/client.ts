import { Pool, neonConfig } from '@neondatabase/serverless';
import * as Sentry from '@sentry/astro';
import { drizzle } from 'drizzle-orm/neon-serverless';
import ws from 'ws';
import { log } from '../lib/log';
import * as schema from './schema';

neonConfig.webSocketConstructor = ws;

const pool = new Pool({ connectionString: import.meta.env.DATABASE_URL });

function isNeonAdministrativeTermination(error: unknown) {
  if (!(error instanceof Error)) return false;
  return error.message.includes('terminating connection due to administrator command');
}

pool.on('error', (error: unknown) => {
  if (isNeonAdministrativeTermination(error)) {
    log.warn('neon_idle_connection_terminated', { error });
    return;
  }

  log.error('neon_pool_error', { error });
  Sentry.captureException(error);
});

export const db = drizzle(pool, { schema });
