import { TextMorph } from 'torph/react';
import { useDashboardQuery } from '../../lib/use-dashboard-query';
import { DashboardQueryError } from './DashboardQueryError';

type Balances = { total: number; free: number; paid: number };

const BALANCE_LABELS = ['Credits', 'Free', 'Paid'] as const;

function balanceValues(balances: Balances) {
  return [balances.total, balances.free, balances.paid];
}

export function ProfileBalanceValues() {
  const query = useDashboardQuery<Balances>('/api/profile/balances');
  if (!query.data && query.isError) return <DashboardQueryError what="credits" retry={query.refetch} />;
  const values = query.data ? balanceValues(query.data) : null;
  return <>
    <div className="stat-grid profile-stats" aria-busy={values ? undefined : 'true'}>
      {BALANCE_LABELS.map((label, index) => (
        <div className="stat" key={label}>
          {values
            ? <><div className="v"><TextMorph duration={180} scale={false}>{values[index]}</TextMorph></div><div className="l">{label}</div></>
            : <span className="dashboard-skeleton" />}
        </div>
      ))}
    </div>
    {query.data && query.isError && <span className="status status--err" role="status">Could not refresh credits; showing the last balance.</span>}
  </>;
}
