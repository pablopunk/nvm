import type { APIRoute } from 'astro';
import * as Sentry from '@sentry/astro';
import { isAuthorizedCron } from '../../../lib/cron-auth';
import { log } from '../../../lib/log';
import {
  pricingRefreshSucceeded,
  refreshPricingSnapshots,
} from '../../../lib/pricing-refresh';
import { openRouterRouteModelIds } from '../../../lib/pricing-route-targets';

// The only path that fetches prices from upstream catalogs. Inference reads the
// durable snapshots this writes and never reaches the network itself.
// Every failure reaches Sentry under the `pricing_refresh` tag: a refused catalog as a
// message, and a crash (such as an unreadable route configuration) as an exception.
export const GET: APIRoute = async ({ request }) => {
  if (!isAuthorizedCron(request)) return new Response('Unauthorized', { status: 401 });
  try {
    const report = await refreshPricingSnapshots(await openRouterRouteModelIds());
    const ok = pricingRefreshSucceeded(report);
    log.info('pricing_refresh', { ok, ...report });
    if (!ok) {
      Sentry.captureMessage('pricing_refresh_failed', {
        level: 'error',
        tags: { kind: 'pricing_refresh' },
        extra: { ...report },
      });
    }
    return Response.json({ ok, ...report }, { status: ok ? 200 : 503 });
  } catch (error) {
    log.error('pricing_refresh_crashed', { error: String(error) });
    Sentry.captureException(error, { tags: { kind: 'pricing_refresh' } });
    return Response.json({ ok: false, error: 'pricing_refresh_crashed' }, { status: 503 });
  }
};
