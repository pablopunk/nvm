import type { APIRoute } from 'astro';
import { sql } from 'drizzle-orm';
import { db } from '../../db/client';
import { getUpstreamConfig } from '../../lib/upstream';
import { getActiveProvider } from '../../lib/settings';
import { compatibilityHeaders } from '../../lib/compatibility';
import { backendCommitSha, backendShortVersion } from '../../lib/deployment-identity';
import { log } from '../../lib/log';

const UPSTREAM_TIMEOUT_MS = 4000;

// `upstream` keeps its released meaning (reachable, not 5xx) so existing monitors and
// the cron ping are unaffected. `upstreamReady` is the additive strict signal: the
// provider must return a successful model catalog. Some catalogs are public, so
// this does not establish that the configured credential is accepted.
type UpstreamProbe = { upstream: boolean; upstreamReady: boolean };

const UPSTREAM_UNREACHABLE: UpstreamProbe = { upstream: false, upstreamReady: false };

async function checkDb(): Promise<boolean> {
  try {
    await db.execute(sql`select 1`);
    return true;
  } catch (err) {
    log.error('health_db_failed', { error: err });
    return false;
  }
}

function isModelCatalog(body: unknown): boolean {
  return typeof body === 'object' && body !== null && Array.isArray((body as { data?: unknown }).data);
}

async function readsModelCatalog(res: Response): Promise<boolean> {
  try {
    return isModelCatalog(await res.json());
  } catch {
    return false;
  }
}

async function probeUpstream(): Promise<UpstreamProbe> {
  try {
    const provider = await getActiveProvider();
    const { baseUrl, apiKey } = getUpstreamConfig(provider);
    const ctl = new AbortController();
    const timer = setTimeout(() => ctl.abort(), UPSTREAM_TIMEOUT_MS);
    try {
      const res = await fetch(`${baseUrl}/models`, {
        method: 'GET',
        headers: { authorization: `Bearer ${apiKey}` },
        signal: ctl.signal,
      });
      const upstream = res.status < 500;
      if (!res.ok) {
        await res.body?.cancel();
        log.warn('health_upstream_not_ready', { status: res.status });
        return { upstream, upstreamReady: false };
      }
      const upstreamReady = await readsModelCatalog(res);
      if (!upstreamReady) log.warn('health_upstream_catalog_invalid', { status: res.status });
      return { upstream, upstreamReady };
    } finally {
      clearTimeout(timer);
    }
  } catch (err) {
    log.error('health_upstream_failed', { error: err });
    return UPSTREAM_UNREACHABLE;
  }
}

export const GET: APIRoute = async () => {
  const [dbOk, upstream] = await Promise.all([checkDb(), probeUpstream()]);
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
