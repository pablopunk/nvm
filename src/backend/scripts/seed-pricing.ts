import 'dotenv/config';
import { closeDefaultDb } from '../src/db/client';
import { verifyLivePricingReadiness } from '../src/lib/pricing-readiness';
import { pricingRefreshSucceeded, refreshPricingSnapshots } from '../src/lib/pricing-refresh';
import { openRouterRouteModelIds } from '../src/lib/pricing-route-targets';

// The deploy gate for pricing. It refreshes the durable price snapshots, then requires
// every configured route to admit a current price from what they hold. A failed refresh
// is reported but does not fail the gate while the kept snapshots are still current, so
// an upstream outage cannot block a deploy whose prices are already verified. Requests
// never run this script; the workflow runs it before a deploy takes traffic.
// Without a database the snapshot store is in memory, so a gate would verify only what it
// just wrote. Refuse to run instead of reporting ready.
if (!process.env.DATABASE_URL)
  throw new Error('DATABASE_URL is required to seed and verify pricing snapshots');
const refresh = await refreshPricingSnapshots(await openRouterRouteModelIds());
const readiness = await verifyLivePricingReadiness();
console.log(
  JSON.stringify(
    { refreshed: pricingRefreshSucceeded(refresh), refresh, ...readiness },
    null,
    2,
  ),
);
await closeDefaultDb();
process.exitCode = readiness.ready ? 0 : 1;
