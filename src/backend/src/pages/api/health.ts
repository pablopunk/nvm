import type { APIRoute } from 'astro';
import { sql } from 'drizzle-orm';
import { db } from '../../db/client';
import { getUpstreamConfig } from '../../lib/upstream';
import { getActiveProvider } from '../../lib/settings';
import { compatibilityHeaders } from '../../lib/compatibility';
import { log } from '../../lib/log';

const VERSION = process.env.VERCEL_GIT_COMMIT_SHA?.slice(0, 7) ?? 'dev';
const UPSTREAM_TIMEOUT_MS = 4000;

async function checkDb(): Promise<boolean> {
  try {
    await db.execute(sql`select 1`);
    return true;
  } catch (err) {
    log.error('health_db_failed', { error: err });
    return false;
  }
}

async function checkUpstream(): Promise<boolean> {
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
      return res.status < 500;
    } finally {
      clearTimeout(timer);
    }
  } catch (err) {
    log.error('health_upstream_failed', { error: err });
    return false;
  }
}

export const GET: APIRoute = async () => {
  const [dbOk, upstreamOk] = await Promise.all([checkDb(), checkUpstream()]);
  const ok = dbOk && upstreamOk;
  return Response.json(
    { ok, db: dbOk, upstream: upstreamOk, version: VERSION },
    { status: ok ? 200 : 503, headers: compatibilityHeaders() },
  );
};
