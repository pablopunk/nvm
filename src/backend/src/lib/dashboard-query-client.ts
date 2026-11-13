import { QueryClient, type QueryFunctionContext } from '@tanstack/query-core';
import { persistQueryClient } from '@tanstack/query-persist-client-core';
import { createSyncStoragePersister } from '@tanstack/query-sync-storage-persister';
import { dashboardFetch } from './dashboard-fetch';

export const DASHBOARD_CACHE_STORAGE_KEY = 'nvm-dashboard-cache';
const DASHBOARD_CACHE_MAX_AGE_MS = 24 * 60 * 60 * 1000;
export const DASHBOARD_REFRESH_MS = 60_000;

export type DashboardCacheStorage = 'local' | 'session';
type DashboardCacheConfig = { scope: string; storage: DashboardCacheStorage };

let client: QueryClient | undefined;
let restored: Promise<void> = Promise.resolve();

function readDashboardCacheConfig(): DashboardCacheConfig | null {
  const content = document.querySelector<HTMLMetaElement>('meta[name="dashboard-cache"]')?.content;
  if (!content) return null;
  try {
    const config = JSON.parse(content) as Partial<DashboardCacheConfig>;
    if (typeof config.scope !== 'string' || (config.storage !== 'local' && config.storage !== 'session')) return null;
    return { scope: config.scope, storage: config.storage };
  } catch {
    return null;
  }
}

function browserStorage(kind: DashboardCacheStorage): Storage | undefined {
  try {
    return kind === 'local' ? window.localStorage : window.sessionStorage;
  } catch {
    return undefined;
  }
}

function fetchDashboardQuery({ queryKey }: QueryFunctionContext) {
  return dashboardFetch(String(queryKey[0]));
}

function persistDashboardCache(queryClient: QueryClient): Promise<void> {
  const config = readDashboardCacheConfig();
  const storage = config && browserStorage(config.storage);
  if (!config || !storage) return Promise.resolve();
  const [, restore] = persistQueryClient({
    queryClient,
    persister: createSyncStoragePersister({ storage, key: DASHBOARD_CACHE_STORAGE_KEY }),
    maxAge: DASHBOARD_CACHE_MAX_AGE_MS,
    buster: config.scope,
  });
  return restore.catch(() => undefined);
}

/** One client per page, shared by every island and script, restored from the per-user browser cache. */
export function dashboardQueryClient(): QueryClient {
  if (client) return client;
  client = new QueryClient({
    defaultOptions: {
      queries: {
        queryFn: fetchDashboardQuery,
        retry: false,
        gcTime: DASHBOARD_CACHE_MAX_AGE_MS,
        refetchInterval: DASHBOARD_REFRESH_MS,
      },
    },
  });
  restored = persistDashboardCache(client);
  return client;
}

/** Resolves once cached data is in the client, so the first paint shows it instead of a skeleton. */
export function dashboardCacheRestored(): Promise<QueryClient> {
  const queryClient = dashboardQueryClient();
  return restored.then(() => queryClient);
}
