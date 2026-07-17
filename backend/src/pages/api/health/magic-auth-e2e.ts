import { timingSafeEqual } from 'node:crypto';
import { Redis } from '@upstash/redis';
import type { APIRoute } from 'astro';
import { eq } from 'drizzle-orm';
import { db } from '../../../db/client';
import { appSettings } from '../../../db/schema';
import { env } from '../../../lib/env';

export const MAGIC_AUTH_ENVIRONMENT_SETTING = 'magic_auth_e2e_environment_id';

function sameSecret(actual: string | null, expected: string | undefined) {
  if (!actual?.startsWith('Bearer ') || !expected) return false;
  const supplied = Buffer.from(actual.slice('Bearer '.length));
  const configured = Buffer.from(expected);
  return supplied.length === configured.length && timingSafeEqual(supplied, configured);
}

export const GET: APIRoute = async ({ request }) => {
  if (!sameSecret(request.headers.get('authorization'), env('NVM_MAGIC_AUTH_PROBE_SECRET'))) {
    return new Response('Not found', { status: 404 });
  }
  const environmentId = env('NVM_MAGIC_AUTH_ENVIRONMENT_ID');
  const namespace = env('GATEWAY_STATE_NAMESPACE');
  const redisUrl = env('GATEWAY_STATE_REDIS_URL');
  const redisToken = env('GATEWAY_STATE_REDIS_TOKEN');
  if (
    env('NVM_MAGIC_AUTH_WORKOS_ENV') !== 'staging' ||
    !env('WORKOS_API_KEY')?.startsWith('sk_test_') ||
    !environmentId ||
    !/^nvm:magic-auth-e2e:[a-z0-9_-]+:v\d+$/i.test(namespace ?? '') ||
    !redisUrl ||
    !redisToken
  ) {
    return Response.json({ ok: false, reason: 'deployment_configuration' }, { status: 503 });
  }
  try {
    const [databaseMarker] = await db
      .select({ value: appSettings.value })
      .from(appSettings)
      .where(eq(appSettings.key, MAGIC_AUTH_ENVIRONMENT_SETTING))
      .limit(1);
    const redisMarker = await new Redis({ url: redisUrl, token: redisToken }).get<string>(`${namespace}:environment`);
    if (databaseMarker?.value !== environmentId || redisMarker !== environmentId) {
      return Response.json({ ok: false, reason: 'datastore_isolation' }, { status: 503 });
    }
    return Response.json({ ok: true, environmentId, namespace });
  } catch {
    return Response.json({ ok: false, reason: 'datastore_unavailable' }, { status: 503 });
  }
};
