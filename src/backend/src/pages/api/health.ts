import type { APIRoute } from 'astro';
import { sql } from 'drizzle-orm';
import { db } from '../../db/client';
import { compatibilityHeaders } from '../../lib/compatibility';
import { backendCommitSha, backendShortVersion } from '../../lib/deployment-identity';
import { log } from '../../lib/log';
import { probeUpstreamReadiness } from '../../lib/upstream-readiness';

// `upstream` is chat liveness: each tiered chat route can be served by a provider that
// answers without a 5xx, so existing monitors and the cron ping are unaffected. Optional
// audio and decision routes never flip it. `upstreamReady` is the additive strict signal:
// every route the product exposes must be configured and served, and every provider they
// may call must return a successful model catalog and accept the configured key.
async function checkDb(): Promise<boolean> {
  try {
    await db.execute(sql`select 1`);
    return true;
  } catch (err) {
    log.error('health_db_failed', { error: err });
    return false;
  }
}

export const GET: APIRoute = async () => {
  const [dbOk, upstream] = await Promise.all([checkDb(), probeUpstreamReadiness()]);
  const ok = dbOk && upstream.upstream;
  return Response.json(
    {
      ok,
      db: dbOk,
      upstream: upstream.upstream,
      upstreamReady: upstream.upstreamReady,
      version: backendShortVersion(),
      commit: backendCommitSha(),
    },
    { status: ok ? 200 : 503, headers: compatibilityHeaders() },
  );
};
