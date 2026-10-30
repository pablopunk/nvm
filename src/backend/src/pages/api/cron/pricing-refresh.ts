import type { APIRoute } from 'astro';
import { isAuthorizedCron } from '../../../lib/cron-auth';
import {
  captureBackendAlert,
  captureBackendException,
} from '../../../lib/error-tracking';
import { log } from '../../../lib/log';
import {
  pricingRefreshSucceeded,
  refreshPricingSnapshots,
} from '../../../lib/pricing-refresh';
import { openRouterRouteModelIds } from '../../../lib/pricing-route-targets';

// The only path that fetches prices from upstream catalogs. Inference reads the
// durable snapshots this writes and never reaches the network itself.
// Every failure reaches PostHog under the `pricing_refresh` kind: a refused catalog as a
// `backend_alert` event, and a crash (such as an unreadable route configuration) as an exception.
export const GET: APIRoute = async ({ request }) => {
  if (!isAuthorizedCron(request)) return new Response('Unauthorized', { status: 401 });
  try {
    const report = await refreshPricingSnapshots(await openRouterRouteModelIds());
    const ok = pricingRefreshSucceeded(report);
    log.info('pricing_refresh', { ok, ...report });
    if (!ok) {
      captureBackendAlert('pricing_refresh_failed', {
        kind: 'pricing_refresh',
        ...report,
      });
    }
    return Response.json({ ok, ...report }, { status: ok ? 200 : 503 });
  } catch (error) {
    log.error('pricing_refresh_crashed', { error: String(error) });
    captureBackendException(error, { kind: 'pricing_refresh' });
    return Response.json({ ok: false, error: 'pricing_refresh_crashed' }, { status: 503 });
  }
};
