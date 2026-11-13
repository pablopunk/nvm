import assert from 'node:assert/strict';
import { test } from 'node:test';

function memoryStorage(): Storage {
  const values = new Map<string, string>();
  return {
    get length() { return values.size; },
    clear() { values.clear(); },
    getItem(key) { return values.get(key) ?? null; },
    key(index) { return [...values.keys()][index] ?? null; },
    removeItem(key) { values.delete(key); },
    setItem(key, value) { values.set(key, value); },
  };
}

function persistedCache(scope: string, url: string, data: unknown) {
  return JSON.stringify({
    buster: scope,
    timestamp: Date.now(),
    clientState: {
      mutations: [],
      queries: [{ queryKey: [url], queryHash: JSON.stringify([url]), state: { data, dataUpdateCount: 1, dataUpdatedAt: Date.now(), error: null, errorUpdateCount: 0, errorUpdatedAt: 0, fetchFailureCount: 0, fetchFailureReason: null, fetchMeta: null, isInvalidated: false, status: 'success', fetchStatus: 'idle' } }],
    },
  });
}

test('dashboard query client restores the signed-in user cache before first paint', async () => {
  const storage = memoryStorage();
  storage.setItem('nvm-dashboard-cache', persistedCache('user-1', '/api/profile/balances', { total: 7, free: 5, paid: 2 }));
  Object.assign(globalThis, {
    window: { localStorage: storage, addEventListener() {}, removeEventListener() {} },
    document: { querySelector: () => ({ content: JSON.stringify({ scope: 'user-1', storage: 'local' }) }) },
  });
  const { dashboardCacheRestored } = await import('./dashboard-query-client');
  const client = await dashboardCacheRestored();
  assert.deepEqual(client.getQueryData(['/api/profile/balances']), { total: 7, free: 5, paid: 2 });
  client.clear();
});
