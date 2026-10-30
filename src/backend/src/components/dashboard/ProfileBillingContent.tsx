import { useEffect, useState } from 'react';
import { dashboardFetch } from '../../lib/dashboard-fetch';
import { watchDashboard } from '../../lib/dashboard-refresh';
import { Button } from '../ui/button';

type Billing = {
  subscription: { status: string; currentPeriodEnd: string } | null;
  hasCustomer: boolean;
  catalog: { subscriptions: Array<{ tier: string; credits: number; priceId: string }>; topUps: Array<{ credits: number; priceId: string }> };
};

export function ProfileBillingContent({ initial, billingState }: { initial: Billing; billingState: string | null }) {
  const [billing, setBilling] = useState(initial);
  const [stale, setStale] = useState(false);
  useEffect(function watchBilling() {
    let active = true;
    const stop = watchDashboard(async () => {
      try {
        const value = await dashboardFetch<Billing>('/api/profile/billing');
        if (active) { setBilling(value); setStale(false); }
      } catch { if (active) setStale(true); }
    }, 300_000);
    return () => { active = false; stop(); };
  }, []);
  const { subscription, catalog, hasCustomer } = billing;
  const active = subscription && ['active', 'trialing', 'past_due'].includes(subscription.status);
  return <section className="stack">
    <div><h3>Billing</h3><p className="muted">{subscription ? `${subscription.status} · renews ${new Date(subscription.currentPeriodEnd).toLocaleDateString()}` : 'No active subscription.'}</p></div>
    {billingState === 'success' && <p className="status status--ok">Payment received. Credits may take a moment to appear and will update automatically.</p>}
    {billingState === 'canceled' && <p className="status">Checkout canceled. Nothing was charged.</p>}
    <div className="row">
      {!active && catalog.subscriptions.map((item) => <Button variant="default" className="billing-checkout" data-kind="subscription" data-price-id={item.priceId} key={item.priceId}>Subscribe {item.tier} · {item.credits} credits/mo</Button>)}
      {active && catalog.topUps.map((item) => <Button variant="outline" className="billing-checkout" data-kind="top_up" data-price-id={item.priceId} key={item.priceId}>Top up · {item.credits} credits</Button>)}
      {!active && catalog.topUps.length > 0 && <span className="muted">Top-ups unlock after subscribing to Pro.</span>}
      {hasCustomer && <Button variant={active ? 'default' : 'outline'} id="manage-billing" type="button">{active ? 'Manage subscription' : 'Manage billing'}</Button>}
      {catalog.subscriptions.length === 0 && catalog.topUps.length === 0 && <span className="muted">Billing is not configured yet.</span>}
    </div>
    <p id="billing-status" className="status" aria-live="polite"></p>
    {stale && <p className="status status--err" role="status">Could not refresh billing; showing the last information.</p>}
  </section>;
}
