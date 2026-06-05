import assert from 'node:assert/strict';
import { afterEach, test } from 'node:test';
import type { APIContext } from 'astro';
import { setDbForTests, resetDbForTests } from '../../db/client';
import { resetRateLimitOverridesForTests, setRateLimitOverridesForTests } from '../../lib/ratelimit';
import { POST as initiateDeviceAuth } from './auth/device/initiate';
import { POST as exchangeDeviceAuth } from './auth/device/exchange';
import { GET as getActiveModel } from './v1/active-model';
import { POST as postChatCompletion } from './v1/chat/completions';

type FakeDb = ReturnType<typeof createFakeDb>;
type MinimalAPIContext = Pick<APIContext, 'request' | 'url'>;

const originalFetch = globalThis.fetch;

function createChain(result: unknown, onValues?: (values: unknown) => void) {
  const promise = () => Promise.resolve(result);
  const chain = {
    from: () => chain,
    innerJoin: () => chain,
    where: () => chain,
    limit: () => promise(),
    set: () => chain,
    values: (values: unknown) => {
      onValues?.(values);
      return chain;
    },
    returning: () => promise(),
    then: (resolve: Parameters<Promise<unknown>['then']>[0], reject: Parameters<Promise<unknown>['then']>[1]) => promise().then(resolve, reject),
    catch: (reject: Parameters<Promise<unknown>['catch']>[0]) => promise().catch(reject),
  };
  return chain;
}

function createFakeDb(input: { selects?: unknown[]; inserts?: unknown[]; updates?: unknown[] } = {}) {
  const selects = [...(input.selects ?? [])];
  const inserts = [...(input.inserts ?? [])];
  const updates = [...(input.updates ?? [])];
  const insertedValues: unknown[] = [];
  const db = {
    insertedValues,
    select: () => createChain(selects.shift() ?? []),
    insert: () => createChain(inserts.shift() ?? [], (values) => insertedValues.push(values)),
    update: () => createChain(updates.shift() ?? []),
    transaction: async (callback: (tx: FakeDb) => Promise<void>) => callback(db as FakeDb),
  };
  return db;
}

function routeContext(request: Request, url = new URL(request.url)): MinimalAPIContext {
  return { request, url };
}

function installDb(db: FakeDb) {
  setDbForTests(db as any);
  return db;
}

function installModelsDevFetch() {
  globalThis.fetch = async (input: string | URL | Request) => {
    const url = String(input instanceof Request ? input.url : input);
    if (url === 'https://models.dev/api.json') {
      return Response.json({
        opencode: {
          models: {
            'gemini-3-flash': {
              id: 'gemini-3-flash',
              name: 'Gemini 3 Flash',
              cost: { input: 0.3, output: 2.5 },
              limit: { context: 100000, output: 8192 },
              modalities: { input: ['text'] },
            },
          },
        },
      });
    }
    throw new Error(`Unexpected fetch: ${url}`);
  };
}

function proxySelects(options: { free?: number; paid?: number; model?: string | null } = {}) {
  return [
    [{ user: { id: 'user_1', email: 'pablo@example.com', role: 'user' }, tokenId: 'token_1' }],
    [{ free: options.free ?? 10, paid: options.paid ?? 0 }],
    [],
    options.model === null ? [] : [{ value: options.model ?? 'gemini-3-flash' }],
  ];
}

function authorizedChatRequest(body: unknown = { model: 'placeholder', messages: [{ role: 'user', content: 'hello' }] }) {
  return new Request('https://api.nvm.fyi/api/v1/chat/completions', {
    method: 'POST',
    headers: {
      authorization: 'Bearer nvm_pat_test',
      'content-type': 'application/json',
      'x-nevermind-client': 'desktop',
      'x-nevermind-client-version': '0.6.2',
      'x-nevermind-api-version': '1',
    },
    body: JSON.stringify(body),
  });
}

afterEach(() => {
  resetDbForTests();
  resetRateLimitOverridesForTests();
  globalThis.fetch = originalFetch;
  delete process.env.OPENCODE_API_KEY;
  delete process.env.OPENCODE_BASE_URL;
  delete process.env.NEVERMIND_KILL_SWITCHES;
});

test('device auth initiate returns the desktop-v1 initiation contract', async () => {
  const db = installDb(createFakeDb());
  const request = new Request('https://api.nvm.fyi/api/auth/device/initiate', {
    method: 'POST',
    body: JSON.stringify({ label: ' Pablo Mac ' }),
  });

  const response = await initiateDeviceAuth(routeContext(request));
  const body = await response.json() as any;

  assert.equal(response.status, 200);
  assert.equal(typeof body.code, 'string');
  assert.equal(body.verifyUrl, `https://api.nvm.fyi/auth/device?code=${body.code}`);
  assert.match(body.expiresAt, /^\d{4}-\d{2}-\d{2}T/);
  assert.equal(body.pollIntervalMs, 2000);
  assert.equal((db.insertedValues[0] as any).deviceLabel, 'Pablo Mac');
});

test('device auth kill switch returns service-unavailable contract', async () => {
  process.env.NEVERMIND_KILL_SWITCHES = 'auth_device';
  const response = await initiateDeviceAuth(routeContext(new Request('https://api.nvm.fyi/api/auth/device/initiate', {
    method: 'POST',
    headers: { 'x-request-id': 'req_auth_disabled' },
    body: '{}',
  })));

  assert.equal(response.status, 503);
  assert.equal(response.headers.get('x-request-id'), 'req_auth_disabled');
  assert.deepEqual(await response.json(), {
    error: { type: 'service_unavailable', message: 'Device authorization is temporarily disabled.' },
  });
});

test('device auth exchange returns pending and missing-code contracts', async () => {
  const missingCode = await exchangeDeviceAuth(routeContext(new Request('https://api.nvm.fyi/api/auth/device/exchange', { method: 'POST', body: '{}' })));
  assert.equal(missingCode.status, 400);
  assert.equal(await missingCode.text(), 'Missing code');

  installDb(createFakeDb({ selects: [[{ code: 'device_code', approvedAt: null, consumedAt: null, userId: null }]] }));
  const pending = await exchangeDeviceAuth(routeContext(new Request('https://api.nvm.fyi/api/auth/device/exchange', {
    method: 'POST',
    body: JSON.stringify({ code: 'device_code' }),
  })));
  assert.equal(pending.status, 200);
  assert.deepEqual(await pending.json(), { status: 'pending' });
});

test('active-model route returns descriptor contract with compatibility headers', async () => {
  installModelsDevFetch();
  installDb(createFakeDb({ selects: proxySelects() }));
  const response = await getActiveModel(routeContext(new Request('https://api.nvm.fyi/api/v1/active-model', {
    headers: { authorization: 'Bearer nvm_pat_test', 'x-request-id': 'req_active_model' },
  })));
  const body = await response.json() as any;

  assert.equal(response.status, 200);
  assert.equal(response.headers.get('x-request-id'), 'req_active_model');
  assert.ok(response.headers.get('x-nevermind-backend-version'));
  assert.equal(body.id, 'gemini-3-flash');
  assert.equal(body.name, 'Gemini 3 Flash');
  assert.equal(body.provider, 'nevermind');
  assert.equal(body.api, 'google-generative-ai');
  assert.equal(body.baseUrl, 'https://api.nvm.fyi/api/v1');
});

test('proxy route kill switch returns service-unavailable contract', async () => {
  process.env.NEVERMIND_KILL_SWITCHES = 'ai_proxy';
  const response = await postChatCompletion(routeContext(authorizedChatRequest()));

  assert.equal(response.status, 503);
  assert.ok(response.headers.get('x-request-id'));
  assert.deepEqual(await response.json(), {
    error: { type: 'service_unavailable', message: 'AI proxy is temporarily disabled.' },
  });
});

test('proxy route returns stable auth, credits, model config, and prompt-size errors', async () => {
  const unauthorized = await postChatCompletion(routeContext(new Request('https://api.nvm.fyi/api/v1/chat/completions', { method: 'POST' })));
  assert.equal(unauthorized.status, 401);
  assert.equal(await unauthorized.text(), 'Unauthorized');
  assert.ok(unauthorized.headers.get('x-request-id'));

  installDb(createFakeDb({ selects: proxySelects({ free: 0, paid: 0 }) }));
  const noCredits = await postChatCompletion(routeContext(authorizedChatRequest()));
  assert.equal(noCredits.status, 402);
  assert.deepEqual(await noCredits.json(), {
    error: { type: 'insufficient_credits', message: 'No credits remaining', dashboard_url: 'https://nvm.fyi/dashboard' },
  });

  installDb(createFakeDb({ selects: proxySelects({ model: null }) }));
  const noModel = await postChatCompletion(routeContext(authorizedChatRequest()));
  assert.equal(noModel.status, 503);
  assert.deepEqual(await noModel.json(), {
    error: { type: 'model_not_configured', message: 'No active model configured. Admin must set one.' },
  });

  installModelsDevFetch();
  process.env.OPENCODE_API_KEY = 'upstream-key';
  installDb(createFakeDb({ selects: proxySelects({ free: 1000000 }) }));
  const promptTooLarge = await postChatCompletion(routeContext(authorizedChatRequest({ messages: [{ role: 'user', content: 'x'.repeat(400_004) }] })));
  assert.equal(promptTooLarge.status, 413);
  assert.deepEqual(await promptTooLarge.json(), {
    error: { type: 'prompt_too_large', message: 'Prompt exceeds 100000 input tokens' },
  });
});

test('proxy route returns the rate-limit contract', async () => {
  installModelsDevFetch();
  process.env.OPENCODE_API_KEY = 'upstream-key';
  setRateLimitOverridesForTests({ chat: async () => ({ ok: false, scope: 'chat:free:minute', retryAfterSec: 7 }) });
  installDb(createFakeDb({ selects: proxySelects() }));

  const response = await postChatCompletion(routeContext(authorizedChatRequest()));
  assert.equal(response.status, 429);
  assert.equal(response.headers.get('Retry-After'), '7');
  assert.deepEqual(await response.json(), {
    error: { type: 'rate_limited', message: 'Rate limit exceeded (chat:free:minute)', retry_after: 7, dashboard_url: 'https://nvm.fyi/dashboard' },
  });
});

test('proxy route preserves streaming responses and records stream usage', async () => {
  installModelsDevFetch();
  process.env.OPENCODE_API_KEY = 'upstream-key';
  process.env.OPENCODE_BASE_URL = 'https://upstream.example/v1';
  const db = installDb(createFakeDb({ selects: proxySelects({ free: 1000 }) }));
  let forwardedBody = '';
  globalThis.fetch = async (input: string | URL | Request, init?: RequestInit) => {
    const url = String(input instanceof Request ? input.url : input);
    if (url === 'https://models.dev/api.json') {
      return Response.json({ opencode: { models: { 'gemini-3-flash': { id: 'gemini-3-flash', cost: { input: 0.3, output: 2.5 } } } } });
    }
    assert.equal(url, 'https://upstream.example/v1/chat/completions');
    assert.equal(new Headers(init?.headers).get('authorization'), 'Bearer upstream-key');
    forwardedBody = String(init?.body);
    return new Response('data: {"usage":{"prompt_tokens":2,"completion_tokens":3}}\n\ndata: [DONE]\n\n', {
      status: 200,
      headers: { 'content-type': 'text/event-stream' },
    });
  };

  const response = await postChatCompletion(routeContext(authorizedChatRequest()));
  const text = await response.text();

  assert.equal(response.status, 200);
  assert.equal(response.headers.get('content-type'), 'text/event-stream');
  assert.match(response.headers.get('x-request-id') || '', /^[0-9a-f-]{36}$/);
  assert.equal(text, 'data: {"usage":{"prompt_tokens":2,"completion_tokens":3}}\n\ndata: [DONE]\n\n');
  assert.equal(JSON.parse(forwardedBody).model, 'gemini-3-flash');
  assert.equal(db.insertedValues.length, 2);
  assert.equal((db.insertedValues.at(-1) as any).inputTokens, 2);
  assert.equal((db.insertedValues.at(-1) as any).outputTokens, 3);
});
