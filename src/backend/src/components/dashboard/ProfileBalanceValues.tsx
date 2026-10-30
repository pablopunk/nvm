import { useEffect, useState } from 'react';
import { TextMorph } from 'torph/react';
import { dashboardFetch } from '../../lib/dashboard-fetch';
import { watchDashboard } from '../../lib/dashboard-refresh';

type Balances = { total: number; free: number; paid: number };

export function ProfileBalanceValues({ initial }: { initial: Balances }) {
  const [balances, setBalances] = useState(initial);
  const [stale, setStale] = useState(false);
  useEffect(function watchBalances() {
    let active = true;
    const stop = watchDashboard(async () => {
      try {
        const value = await dashboardFetch<Balances>('/api/profile/balances');
        if (active) { setBalances(value); setStale(false); }
      } catch { if (active) setStale(true); }
    });
    return () => { active = false; stop(); };
  }, []);
  return <>
    <div className="stat-grid profile-stats">
      {([['Credits', balances.total], ['Free', balances.free], ['Paid', balances.paid]] as const).map(([label, value]) => (
        <div className="stat" key={label}><div className="v"><TextMorph duration={180} scale={false}>{value}</TextMorph></div><div className="l">{label}</div></div>
      ))}
    </div>
    {stale && <span className="status status--err" role="status">Could not refresh credits; showing the last balance.</span>}
  </>;
}
