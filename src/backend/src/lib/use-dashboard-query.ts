import { useEffect, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { dashboardCacheRestored, dashboardQueryClient } from './dashboard-query-client';

type DashboardQueryOptions = { refetchInterval?: number; paused?: boolean };

/** Cached data paints at once; a background fetch then replaces it in place. */
export function useDashboardQuery<T>(url: string, { refetchInterval, paused = false }: DashboardQueryOptions = {}) {
  const [cacheRestored, setCacheRestored] = useState(false);
  useEffect(function waitForCacheRestore() {
    let active = true;
    void dashboardCacheRestored().then(() => { if (active) setCacheRestored(true); });
    return () => { active = false; };
  }, []);
  return useQuery<T>({
    queryKey: [url],
    enabled: cacheRestored && !paused,
    ...(refetchInterval ? { refetchInterval } : {}),
  }, dashboardQueryClient());
}
