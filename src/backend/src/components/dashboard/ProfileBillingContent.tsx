import { useDashboardQuery } from '../../lib/use-dashboard-query';
import { DashboardQueryError } from './DashboardQueryError';
import { MAX_TOP_UP_QUANTITY } from '../../lib/billing-limits';
import { Button } from '../ui/button';

type Billing = {
  subscription: { status: string; currentPeriodEnd: string } | null;
  hasCustomer: boolean;
  catalog: { subscriptions: Array<{ tier: string; credits: number; priceId: string }>; topUps: Array<{ credits: number; priceId: string }> };
};

const BILLING_REFRESH_MS = 300_000;

function ProfileBillingSkeleton() {
  return <div aria-busy="true" aria-label="Loading billing"><span className="dashboard-skeleton" /><span className="dashboard-skeleton" /><span className="dashboard-skeleton" /></div>;
}

export function ProfileBillingContent({ billingState }: { billingState: string | null }) {
  const query = useDashboardQuery<Billing>('/api/profile/billing', { refetchInterval: BILLING_REFRESH_MS });
  if (!query.data) return query.isError ? <DashboardQueryError what="billing" retry={query.refetch} /> : <ProfileBillingSkeleton />;
  const { subscription, catalog, hasCustomer } = query.data;
  const stale = query.isError;
  const active = subscription && ['active', 'trialing', 'past_due'].includes(subscription.status);
  return <section className="stack">
    <div><h3>Billing</h3><p className="muted">{subscription ? `${subscription.status} · renews ${new Date(subscription.currentPeriodEnd).toLocaleDateString()}` : 'No active subscription.'}</p></div>
    {billingState === 'success' && <p className="status status--ok">Payment received. Credits may take a moment to appear and will update automatically.</p>}
    {billingState === 'canceled' && <p className="status">Checkout canceled. Nothing was charged.</p>}
    <div className="row">
      {!active && catalog.subscriptions.map((item) => <Button variant="default" className="billing-checkout" data-kind="subscription" data-price-id={item.priceId} key={item.priceId}>Subscribe {item.tier} · {item.credits} credits/mo</Button>)}
      {active && catalog.topUps.flatMap((item) => Array.from({ length: MAX_TOP_UP_QUANTITY }, (_, index) => index + 1).map((quantity) => <Button variant="outline" className="billing-checkout" data-kind="top_up" data-price-id={item.priceId} data-quantity={quantity} key={`${item.priceId}:${quantity}`}>Top up · {(item.credits * quantity).toLocaleString('en')} credits</Button>))}
      {!active && catalog.topUps.length > 0 && <span className="muted">Top-ups unlock after subscribing to Pro.</span>}
      {hasCustomer && <Button variant={active ? 'default' : 'outline'} id="manage-billing" type="button">{active ? 'Manage subscription' : 'Manage billing'}</Button>}
      {catalog.subscriptions.length === 0 && catalog.topUps.length === 0 && <span className="muted">Billing is not configured yet.</span>}
    </div>
    <p id="billing-status" className="status" aria-live="polite"></p>
    {stale && <p className="status status--err" role="status">Could not refresh billing; showing the last information.</p>}
  </section>;
}
