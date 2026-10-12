import assert from 'node:assert/strict';
import { afterEach, test } from 'node:test';
import { resetDbForTests, setDbForTests } from '../../../db/client';
import {
  freeSpendCapConfig,
  freeSpendMonthlyBudgets,
  users,
} from '../../../db/schema';
import {
  createPreviewSessionToken,
  PREVIEW_SESSION_COOKIE,
} from '../../../lib/workos';
import { GET as getRequests } from './requests';
import { GET as getStats } from './stats';
import { GET as getTimeseries } from './timeseries';
import { GET as getUsers } from './users';

const environmentKeys = [
  'VERCEL_ENV',
  'PREVIEW_SESSION_KEY',
  'WORKOS_API_KEY',
  'WORKOS_CLIENT_ID',
  'WORKOS_COOKIE_PASSWORD',
] as const;
const previousEnvironment = Object.fromEntries(
  environmentKeys.map((key) => [key, process.env[key]]),
);

afterEach(() => {
  resetDbForTests();
  for (const key of environmentKeys) {
    const value = previousEnvironment[key];
    if (value === undefined) delete process.env[key];
    else process.env[key] = value;
  }
});

function databaseTimestamp(date: Date, micros: string): string {
  if (!/^\d{6}$/.test(micros))
    throw new Error('Expected six fractional digits');
  return date.toISOString().replace(/\.\d{3}Z$/, `.${micros}Z`);
}

function installAdminDatabase(results: unknown[][]) {
  const executeCalls: unknown[] = [];
  const admin = { id: 'admin-id', workosUserId: 'admin-workos', role: 'admin' };
  const now = new Date();
  const currentBudget = {
    monthKey: `${now.getUTCFullYear()}-${String(now.getUTCMonth() + 1).padStart(2, '0')}`,
    capMicrocents: null,
    actualFreeCostMicrocents: 0,
    reservedFreeCostMicrocents: 0,
    halted: false,
    baselineComplete: true,
    updatedAt: now,
  };
  let database: any;
  database = {
    select() {
      let table: unknown;
      function selectedRows() {
        if (table === users) return [admin];
        if (table === freeSpendCapConfig)
          return [
            {
              id: 'global',
              capMicrocents: null,
              updatedBy: null,
              updatedAt: now,
            },
          ];
        if (table === freeSpendMonthlyBudgets) return [currentBudget];
        return [];
      }
      const chain = {
        from(value: unknown) {
          table = value;
          return chain;
        },
        where() {
          return chain;
        },
        limit() {
          return Promise.resolve(selectedRows());
        },
        for() {
          return Promise.resolve(selectedRows());
        },
      };
      return chain;
    },
    async execute(query: unknown) {
      executeCalls.push(query);
      return results.shift() ?? [];
    },
    async transaction(callback: (tx: any) => Promise<unknown>) {
      return callback(database);
    },
  };
  setDbForTests(database);
  return executeCalls;
}

async function adminRequestCookie() {
  process.env.VERCEL_ENV = 'preview';
  process.env.PREVIEW_SESSION_KEY =
    'preview-session-key-for-report-route-tests';
  process.env.WORKOS_API_KEY = 'sk_test_preview';
  process.env.WORKOS_CLIENT_ID = 'client_test_preview';
  process.env.WORKOS_COOKIE_PASSWORD = 'production-session-key';
  const token = await createPreviewSessionToken({
    id: 'admin-workos',
    email: 'admin@example.test',
  });
  assert.ok(token);
  return `${PREVIEW_SESSION_COOKIE}=${token}`;
}

test('admin request log keeps microsecond cursors, fixed bounds, and safe operational fields', async () => {
  const cookie = await adminRequestCookie();
  const to = new Date(Math.floor(Date.now() / 1000) * 1000);
  const from = new Date(to.getTime() - 24 * 60 * 60 * 1000);
  const fromBound = from.toISOString();
  const toBound = to.toISOString();
  const firstTimestamp = databaseTimestamp(
    new Date(to.getTime() - 1_000),
    '123456',
  );
  const secondTimestamp = databaseTimestamp(
    new Date(to.getTime() - 2_000),
    '654321',
  );
  const rows = [
    {
      id: '12',
      created_at: firstTimestamp,
      model: 'model-a',
      provider: 'provider-a',
      modality: 'text',
      status: 200,
      latency_ms: 25,
      input_tokens: 10,
      output_tokens: 5,
      cached_input_tokens: 0,
      cache_write_input_tokens: 0,
      reasoning_tokens: 0,
      audio_duration_ms: 0,
      audio_token_counts_available: false,
      cost_credits: 1,
      provider_cost_microcents: '123',
      provider_cost_source: 'provider_reported',
      request_id: 'request-a',
      actual_free_credits: 1,
      actual_paid_credits: 0,
      free_attributed_cost_microcents: '123',
      paid_attributed_cost_microcents: '0',
      pool_attribution_unknown: false,
      prompt: 'must never be returned',
    },
    {
      id: '11',
      created_at: secondTimestamp,
      model: 'model-b',
      provider: 'provider-b',
      modality: 'text',
      status: 200,
      latency_ms: 30,
      input_tokens: 12,
      output_tokens: 4,
      cached_input_tokens: 0,
      cache_write_input_tokens: 0,
      reasoning_tokens: 0,
      audio_duration_ms: 0,
      audio_token_counts_available: false,
      cost_credits: 1,
      provider_cost_microcents: '100',
      provider_cost_source: 'catalog_estimate',
      request_id: 'request-b',
      actual_free_credits: 1,
      actual_paid_credits: 0,
      free_attributed_cost_microcents: '100',
      paid_attributed_cost_microcents: '0',
      pool_attribution_unknown: false,
    },
  ];
  const executeCalls = installAdminDatabase([rows, [rows[1]]]);
  const base = `https://www.nvm.fyi/api/admin/requests?from=${encodeURIComponent(fromBound)}&to=${encodeURIComponent(toBound)}&limit=1`;
  const firstRequest = new Request(base, { headers: { cookie } });
  const firstResponse = await getRequests({
    request: firstRequest,
    url: new URL(firstRequest.url),
  } as any);
  const firstBody = (await firstResponse.json()) as any;

  assert.equal(firstResponse.status, 200);
  assert.deepEqual(firstBody.range, { from: fromBound, to: toBound });
  assert.equal(firstBody.requests[0].createdAt, firstTimestamp);
  assert.equal(firstBody.requests[0].providerCostMicrocents, '123');
  assert.equal(firstBody.consistency, 'eventually_consistent');
  assert.equal('prompt' in firstBody.requests[0], false);
  assert.ok(firstBody.nextCursor);

  const nextUrl = new URL(base);
  nextUrl.searchParams.set('cursor', firstBody.nextCursor);
  const nextRequest = new Request(nextUrl, { headers: { cookie } });
  const nextResponse = await getRequests({
    request: nextRequest,
    url: nextUrl,
  } as any);
  const nextBody = (await nextResponse.json()) as any;
  assert.equal(
    nextResponse.status,
    200,
    'a returned cursor is accepted for the same fixed range',
  );
  assert.equal(nextBody.requests[0].createdAt, secondTimestamp);

  nextUrl.searchParams.set('to', new Date(to.getTime() - 1_000).toISOString());
  const changedRangeRequest = new Request(nextUrl, { headers: { cookie } });
  const changedRangeResponse = await getRequests({
    request: changedRangeRequest,
    url: nextUrl,
  } as any);
  assert.equal(
    changedRangeResponse.status,
    400,
    'cursors cannot be replayed against different report bounds',
  );
  assert.equal(
    executeCalls.length,
    2,
    'invalid cursor requests do not query usage data',
  );
});

test('admin timeseries rejects invalid bounds and returns the bounded series contract', async () => {
  const cookie = await adminRequestCookie();
  const executeCalls = installAdminDatabase([[], []]);
  const invalidUrl = new URL(
    'https://www.nvm.fyi/api/admin/timeseries?days=367',
  );
  const invalidRequest = new Request(invalidUrl, { headers: { cookie } });
  const invalidResponse = await getTimeseries({
    request: invalidRequest,
    url: invalidUrl,
  } as any);
  assert.equal(invalidResponse.status, 400);
  assert.equal(executeCalls.length, 0);

  const now = new Date(Math.floor(Date.now() / 1000) * 1000);
  const from = new Date(now.getTime() - 24 * 60 * 60 * 1000).toISOString();
  const to = now.toISOString();
  const url = new URL(
    `https://www.nvm.fyi/api/admin/timeseries?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`,
  );
  const request = new Request(url, { headers: { cookie } });
  const response = await getTimeseries({ request, url } as any);
  assert.equal(response.status, 200);
  assert.deepEqual(await response.json(), {
    range: { from, to },
    series: [],
    byModelProvider: [],
  });
  assert.equal(
    executeCalls.length,
    2,
    'valid time-series reads execute the daily and model/provider queries',
  );
});

test('admin user search is exact-email scoped and keyset pages preserve report bounds', async () => {
  const cookie = await adminRequestCookie();
  const now = new Date(Math.floor(Date.now() / 1000) * 1000);
  const from = new Date(now.getTime() - 24 * 60 * 60 * 1000).toISOString();
  const to = now.toISOString();
  const rows = [
    {
      id: '11111111-1111-4111-8111-111111111111',
      email: 'admin@example.test',
      plan: 'free',
      role: 'user',
      created_at: databaseTimestamp(new Date(now.getTime() - 1_000), '111222'),
      balance: 500,
      free_balance: 500,
      paid_balance: 0,
      requests: 2,
      spent: 10,
      last_model: 'model-a',
      last_used_at: databaseTimestamp(
        new Date(now.getTime() - 2_000),
        '333444',
      ),
    },
    {
      id: '22222222-2222-4222-8222-222222222222',
      email: 'other@example.test',
      plan: 'free',
      role: 'user',
      created_at: databaseTimestamp(new Date(now.getTime() - 3_000), '555666'),
      balance: 500,
      free_balance: 500,
      paid_balance: 0,
      requests: 0,
      spent: 0,
      last_model: null,
      last_used_at: null,
    },
  ];
  const exactEmailCalls = installAdminDatabase([[rows[0]]]);
  const exactEmailUrl = new URL(
    `https://www.nvm.fyi/api/admin/users?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}&limit=1&email=ADMIN%40EXAMPLE.TEST`,
  );
  const exactEmailRequest = new Request(exactEmailUrl, { headers: { cookie } });
  const exactEmailResponse = await getUsers({
    request: exactEmailRequest,
    url: exactEmailUrl,
  } as any);
  const exactEmailBody = (await exactEmailResponse.json()) as any;
  assert.equal(exactEmailResponse.status, 200);
  assert.equal(exactEmailBody.searchedEmail, 'admin@example.test');
  assert.equal(exactEmailBody.searchScope, 'exact_email_all_history');
  assert.equal(exactEmailBody.users[0].createdAt, rows[0].created_at);
  assert.equal(
    exactEmailBody.nextCursor,
    null,
    'an exact-email lookup returns only its matching account',
  );
  assert.equal(exactEmailCalls.length, 1);

  const listCalls = installAdminDatabase([rows, [rows[1]]]);
  const listUrl = new URL(
    `https://www.nvm.fyi/api/admin/users?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}&limit=1`,
  );
  const listRequest = new Request(listUrl, { headers: { cookie } });
  const listResponse = await getUsers({
    request: listRequest,
    url: listUrl,
  } as any);
  const listBody = (await listResponse.json()) as any;
  assert.equal(listResponse.status, 200);
  assert.ok(
    listBody.nextCursor,
    'a bounded user list exposes a keyset cursor when another row exists',
  );
  assert.equal(listBody.users[0].createdAt, rows[0].created_at);

  listUrl.searchParams.set('cursor', listBody.nextCursor);
  const nextRequest = new Request(listUrl, { headers: { cookie } });
  const nextResponse = await getUsers({
    request: nextRequest,
    url: listUrl,
  } as any);
  assert.equal(nextResponse.status, 200);
  assert.deepEqual(((await nextResponse.json()) as any).range, { from, to });
  assert.equal(listCalls.length, 2);
});

test('admin stats keeps cost, cash coverage, and unset-cap status explicit', async () => {
  const cookie = await adminRequestCookie();
  const usage = {
    requests: '0',
    errors: '0',
    input_tokens: '0',
    output_tokens: '0',
    cached_input_tokens: '0',
    cache_write_input_tokens: '0',
    reasoning_tokens: '0',
    audio_duration_ms: '0',
    cost_credits: '0',
    provider_reported_cost_microcents: '0',
    catalog_estimated_cost_microcents: '0',
    unknown_cost_source_requests: '0',
    free_attributed_cost_microcents: '0',
    paid_attributed_cost_microcents: '0',
    unknown_pool_requests: '0',
    mixed_pool_requests: '0',
    estimated_cost_requests: '0',
  };
  const fieldCompleteness = {
    fee_unknown: '0',
    amount_unknown: '0',
    net_unknown: '0',
    currency_unknown: '0',
    category_unknown: '0',
    status_unknown: '0',
  };
  const executeCalls = installAdminDatabase([
    [usage],
    [
      {
        kind: 'free',
        granted: '500',
        consumed: '0',
        adjusted: '0',
        outstanding: '500',
      },
      {
        kind: 'paid',
        granted: '0',
        consumed: '0',
        adjusted: '0',
        outstanding: '0',
      },
    ],
    [],
    [],
    [],
    [fieldCompleteness],
    [{ created_in_range: '0' }],
  ]);
  const url = new URL('https://www.nvm.fyi/api/admin/stats');
  const request = new Request(url, { headers: { cookie } });
  const response = await getStats({ request, url } as any);
  const body = (await response.json()) as any;

  assert.equal(response.status, 200);
  assert.equal(body.usage.freeSubsidyEstimate.complete, true);
  assert.equal(body.financials.intervalCoverage.complete, false);
  assert.equal(body.freeSpendCap.unconfigured, true);
  assert.match(body.freeSpendCap.warning, /remains unbounded/);
  assert.equal('profit' in body, false);
  assert.equal(executeCalls.length, 7);
});
