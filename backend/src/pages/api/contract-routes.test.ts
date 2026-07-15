import assert from 'node:assert/strict';
import { afterEach, test } from 'node:test';
import type { APIContext } from 'astro';
import { setDbForTests, resetDbForTests } from '../../db/client';
import { resetRateLimitOverridesForTests, setRateLimitOverridesForTests } from '../../lib/ratelimit';
import { resetPricingCacheForTests } from '../../lib/pricing';
import { POST as initiateDeviceAuth } from './auth/device/initiate';
import { POST as exchangeDeviceAuth } from './auth/device/exchange';
import { GET as getActiveModel } from './v1/active-model';
import { POST as postChatCompletion } from './v1/chat/completions';
import { POST as postGoogleModel } from './v1/models/[...path]';

type FakeDb = {
  insertedValues: unknown[];
  select: () => ReturnType<typeof createChain>;
  insert: () => ReturnType<typeof createChain>;
  update: () => ReturnType<typeof createChain>;
  transaction: (callback: (tx: FakeDb) => Promise<void>) => Promise<void>;
};

const originalFetch = globalThis.fetch;

function createChain(result: unknown, onValues?: (values: unknown) => void) {
  const promise = () => Promise.resolve(result);
  const chain = {
    from: () => chain,
    innerJoin: () => chain,
    where: () => chain,
    limit: () => promise(),
    orderBy: () => promise(),
    set: () => chain,
    values: (values: unknown) => {
      onValues?.(values);
      return chain;
    },
    onConflictDoNothing: () => chain,
    returning: () => promise(),
    then: (resolve: Parameters<Promise<unknown>['then']>[0], reject: Parameters<Promise<unknown>['then']>[1]) => promise().then(resolve, reject),
    catch: (reject: Parameters<Promise<unknown>['catch']>[0]) => promise().catch(reject),
  };
  return chain;
}

function createFakeDb(input: { selects?: unknown[]; inserts?: unknown[]; updates?: unknown[] } = {}): FakeDb {
  const selects = [...(input.selects ?? [])];
  const inserts = [...(input.inserts ?? [])];
  const updates = [...(input.updates ?? [])];
  const insertedValues: unknown[] = [];
  let db: FakeDb;
  db = {
    insertedValues,
    select: () => createChain(selects.shift() ?? []),
    insert: () => createChain(inserts.shift() ?? [], (values) => insertedValues.push(values)),
    update: () => createChain(updates.shift() ?? []),
    transaction: async (callback: (tx: FakeDb) => Promise<void>) => callback(db),
  };
  return db;
}

function routeContext(request: Request, url = new URL(request.url)): APIContext {
  return { request, url } as APIContext;
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
            'gemini-3-fast': {
              id: 'gemini-3-fast',
              name: 'Gemini 3 Fast',
              cost: { input: 0.1, output: 0.5 },
              limit: { context: 64000, output: 4096 },
              modalities: { input: ['text'] },
            },
          },
        },
      });
    }
    throw new Error(`Unexpected fetch: ${url}`);
  };
}

function proxySelects(options: { free?: number; paid?: number; model?: string | null; routeProvider?: string } = {}) {
  const modelRoute = options.model === null ? [] : [{ value: JSON.stringify({ provider: options.routeProvider ?? 'opencode_zen', modelId: options.model ?? 'gemini-3-flash' }) }];
  return [
    [{ user: { id: 'user_1', email: 'pablo@example.com', role: 'user' }, tokenId: 'token_1' }],
    [{ id: 1 }],
    [{ free: options.free ?? 10, paid: options.paid ?? 0 }],
    modelRoute,
    [],
    [],
  ];
}

function authorizedChatRequest(body: unknown = { model: 'placeholder', messages: [{ role: 'user', content: 'hello' }] }, extraHeaders: Record<string, string> = {}) {
  return new Request('https://api.nvm.fyi/api/v1/chat/completions', {
    method: 'POST',
    headers: {
      authorization: 'Bearer nvm_pat_test',
      'content-type': 'application/json',
      'x-nevermind-client': 'desktop',
      'x-nevermind-client-version': '0.6.2',
      'x-nevermind-api-version': '1',
      ...extraHeaders,
    },
    body: JSON.stringify(body),
  });
}

function authorizedGoogleRequest(body: unknown = { contents: [{ parts: [{ text: 'hello' }] }] }) {
  return new Request('https://api.nvm.fyi/api/v1/models/placeholder:streamGenerateContent?alt=sse', {
    method: 'POST',
    headers: {
      'x-goog-api-key': 'nvm_pat_test',
      'content-type': 'application/json',
      'x-nevermind-client': 'desktop',
      'x-nevermind-client-version': '0.7.0',
      'x-nevermind-api-version': '1',
    },
    body: JSON.stringify(body),
  });
}

function streamFromTextChunks(chunks: string[]): ReadableStream<Uint8Array> {
  const encoder = new TextEncoder();
  return streamFromByteChunks(chunks.map((chunk) => encoder.encode(chunk)));
}

function streamFromByteChunks(chunks: Uint8Array[]): ReadableStream<Uint8Array> {
  return new ReadableStream({
    start(controller) {
      for (const chunk of chunks) controller.enqueue(chunk);
      controller.close();
    },
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
  const missingCodeBody = await missingCode.json() as any;
  assert.equal(missingCodeBody.error.type, 'invalid_request');
  assert.equal(missingCodeBody.error.message, 'Request body validation failed');

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

test('active-model route resolves admin-defined extension model roles', async () => {
  installModelsDevFetch();
  installDb(createFakeDb({ selects: proxySelects({ model: 'gemini-3-fast' }) }));
  const response = await getActiveModel(routeContext(new Request('https://api.nvm.fyi/api/v1/active-model?model=fast', {
    headers: { authorization: 'Bearer nvm_pat_test', 'x-request-id': 'req_fast_model' },
  })));
  const body = await response.json() as any;

  assert.equal(response.status, 200);
  assert.equal(response.headers.get('x-request-id'), 'req_fast_model');
  assert.equal(body.id, 'gemini-3-fast');
  assert.equal(body.name, 'Gemini 3 Fast');
  assert.equal(body.provider, 'nevermind');
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

test('proxy route honors extension smart/fast model selection headers', async () => {
  installModelsDevFetch();
  process.env.OPENCODE_API_KEY = 'upstream-key';
  process.env.OPENCODE_BASE_URL = 'https://upstream.example/v1';
  const db = installDb(createFakeDb({ selects: proxySelects({ free: 1000, model: 'gemini-3-fast' }) }));
  let forwardedBody = '';
  globalThis.fetch = async (input: string | URL | Request, init?: RequestInit) => {
    const url = String(input instanceof Request ? input.url : input);
    if (url === 'https://models.dev/api.json') {
      return Response.json({ opencode: { models: { 'gemini-3-fast': { id: 'gemini-3-fast', cost: { input: 0.1, output: 0.5 } } } } });
    }
    assert.equal(url, 'https://upstream.example/v1/chat/completions');
    forwardedBody = String(init?.body);
    assert.equal(new Headers(init?.headers).get('x-nevermind-ai-model'), null);
    return Response.json({ usage: { prompt_tokens: 2, completion_tokens: 3 } });
  };

  const response = await postChatCompletion(routeContext(authorizedChatRequest(undefined, { 'x-nevermind-ai-model': 'fast' })));

  assert.equal(response.status, 200);
  assert.equal(JSON.parse(forwardedBody).model, 'gemini-3-fast');
  assert.equal(db.insertedValues.length, 2);
  assert.equal((db.insertedValues.at(-1) as any).model, 'gemini-3-fast');
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

test('openai proxy records usage across hostile UTF-8 and CRLF frame splits', async () => {
  installModelsDevFetch();
  process.env.OPENCODE_API_KEY = 'upstream-key';
  process.env.OPENCODE_BASE_URL = 'https://upstream.example/v1';
  const db = installDb(createFakeDb({ selects: proxySelects({ free: 1000 }) }));
  const frame = 'data: {"usage":{"prompt_tokens":12,"completion_tokens":4},"note":"é"}\r\n';
  const encodedFrame = new TextEncoder().encode(frame);
  const promptTokenSplit = new TextEncoder().encode(frame.slice(0, frame.indexOf('prompt_tokens') + 7)).byteLength;
  const multibyteCharacterStart = new TextEncoder().encode(frame.slice(0, frame.indexOf('é'))).byteLength;
  const chunks = [
    encodedFrame.slice(0, 2),
    encodedFrame.slice(2, promptTokenSplit),
    encodedFrame.slice(promptTokenSplit, multibyteCharacterStart + 1),
    encodedFrame.slice(multibyteCharacterStart + 1, -1),
    encodedFrame.slice(-1),
  ];
  globalThis.fetch = async (input: string | URL | Request) => {
    const url = String(input instanceof Request ? input.url : input);
    if (url === 'https://models.dev/api.json') {
      return Response.json({ opencode: { models: { 'gemini-3-flash': { id: 'gemini-3-flash', cost: { input: 0.3, output: 2.5 } } } } });
    }
    assert.equal(url, 'https://upstream.example/v1/chat/completions');
    return new Response(streamFromByteChunks(chunks), {
      status: 200,
      headers: { 'content-type': 'text/event-stream' },
    });
  };

  const response = await postChatCompletion(routeContext(authorizedChatRequest()));

  assert.equal(response.status, 200);
  assert.deepEqual(new Uint8Array(await response.arrayBuffer()), encodedFrame);
  assert.equal((db.insertedValues.at(-1) as any).inputTokens, 12);
  assert.equal((db.insertedValues.at(-1) as any).outputTokens, 4);
});

test('anthropic proxy records usage from a split final frame without a newline', async () => {
  installModelsDevFetch();
  process.env.OPENCODE_API_KEY = 'upstream-key';
  process.env.OPENCODE_BASE_URL = 'https://upstream.example/v1';
  const db = installDb(createFakeDb({ selects: proxySelects({ free: 1000 }) }));
  const chunks = [
    'da',
    'ta: {"type":"message_delta","usage":{"input_to',
    'kens":11,"output_tokens":5}}',
  ];
  globalThis.fetch = async (input: string | URL | Request) => {
    const url = String(input instanceof Request ? input.url : input);
    if (url === 'https://models.dev/api.json') {
      return Response.json({ opencode: { models: { 'gemini-3-flash': { id: 'gemini-3-flash', cost: { input: 0.3, output: 2.5 } } } } });
    }
    assert.equal(url, 'https://upstream.example/v1/messages');
    return new Response(streamFromTextChunks(chunks), {
      status: 200,
      headers: { 'content-type': 'text/event-stream' },
    });
  };

  const response = await postMessages(routeContext(authorizedMessagesRequest()));

  assert.equal(response.status, 200);
  assert.equal(await response.text(), chunks.join(''));
  assert.equal((db.insertedValues.at(-1) as any).inputTokens, 11);
  assert.equal((db.insertedValues.at(-1) as any).outputTokens, 5);
});

const splitGoogleUsageStream = [
  'data: {"candidates":[{"content":{"parts":[{"text":"hi"}]}}],"usageMetadata":{"promptTokenCount":12,',
  '"candidatesTokenCount":4,"thoughtsTokenCount":2}}',
];

test('google proxy records split streaming usage metadata', async () => {
  installModelsDevFetch();
  process.env.OPENCODE_API_KEY = 'upstream-key';
  process.env.OPENCODE_BASE_URL = 'https://upstream.example/v1';
  const db = installDb(createFakeDb({ selects: proxySelects({ paid: 1000, free: 500 }) }));
  globalThis.fetch = async (input: string | URL | Request, init?: RequestInit) => {
    const url = String(input instanceof Request ? input.url : input);
    if (url === 'https://models.dev/api.json') {
      return Response.json({ opencode: { models: { 'gemini-3-flash': { id: 'gemini-3-flash', cost: { input: 0.3, output: 2.5 } } } } });
    }
    assert.equal(url, 'https://upstream.example/v1/models/gemini-3-flash:streamGenerateContent?alt=sse');
    assert.equal(new Headers(init?.headers).get('x-goog-api-key'), 'upstream-key');
    return new Response(streamFromTextChunks(splitGoogleUsageStream), {
      status: 200,
      headers: { 'content-type': 'text/event-stream' },
    });
  };

  const request = authorizedGoogleRequest();
  const response = await postGoogleModel({ ...routeContext(request), params: { path: 'placeholder:streamGenerateContent' } } as any);
  const text = await response.text();

  assert.equal(response.status, 200);
  assert.equal(text, splitGoogleUsageStream.join(''));
  assert.equal(db.insertedValues.length, 2);
  assert.equal((db.insertedValues[0] as any).kind, 'paid');
  assert.equal((db.insertedValues.at(-1) as any).inputTokens, 12);
  assert.equal((db.insertedValues.at(-1) as any).outputTokens, 6);
  assert.equal((db.insertedValues.at(-1) as any).costCredits, 1);
});

test('proxy failover: primary 5xx falls back to next provider in chain', async () => {
  resetPricingCacheForTests();
  process.env.OPENCODE_API_KEY = 'primary-key';
  process.env.OPENCODE_BASE_URL = 'https://primary.example/v1';
  process.env.GOOGLE_API_KEY = 'fallback-key';
  process.env.GOOGLE_BASE_URL = 'https://fallback.example';
  const modelRoute = [{ value: JSON.stringify({ provider: 'opencode_zen', modelId: 'gemini-3-flash' }) }];
  const providerChain = [{ providerId: 'google' }];
  const db = installDb(createFakeDb({
    selects: [
      [{ user: { id: 'user_1', email: 'pablo@example.com', role: 'user' }, tokenId: 'token_1' }],
      [{ id: 1 }],
      [{ paid: 1000, free: 500 }],
      modelRoute,
      providerChain,
    ],
  }));

  let primaryCalled = false;
  let fallbackCalled = false;
  globalThis.fetch = async (input: string | URL | Request, init?: RequestInit) => {
    const url = String(input instanceof Request ? input.url : input);
    if (url === 'https://models.dev/api.json') {
      return Response.json({ opencode: { models: { 'gemini-3-flash': { id: 'gemini-3-flash', cost: { input: 0.3, output: 2.5 } } } }, google: { models: { 'gemini-3-flash': { id: 'gemini-3-flash', cost: { input: 0.3, output: 2.5 } } } } });
    }
    if (url.startsWith('https://primary.example')) {
      primaryCalled = true;
      return Response.json({ error: { message: 'Down' } }, { status: 503 });
    }
    if (url.startsWith('https://fallback.example')) {
      fallbackCalled = true;
      assert.equal(new Headers(init?.headers).get('x-goog-api-key'), 'fallback-key');
      return Response.json({ usageMetadata: { promptTokenCount: 5, candidatesTokenCount: 10, thoughtsTokenCount: 0 } });
    }
    throw new Error(`Unexpected fetch: ${url}`);
  };

  const request = authorizedGoogleRequest();
  const response = await postGoogleModel({ ...routeContext(request), params: { path: 'placeholder:streamGenerateContent' } } as any);
  const body = await response.json() as any;

  assert.equal(response.status, 200);
  assert.ok(primaryCalled, 'primary provider should have been tried');
  assert.ok(fallbackCalled, 'fallback provider should have been tried');
  assert.equal(db.insertedValues.length, 2);
  assert.equal((db.insertedValues[0] as any).kind, 'paid');
  assert.equal((db.insertedValues.at(-1) as any).provider, 'google');
  assert.equal((db.insertedValues.at(-1) as any).inputTokens, 5);
  assert.equal((db.insertedValues.at(-1) as any).outputTokens, 10);
});

test('proxy failover: when all providers fail, last error passes through', async () => {
  resetPricingCacheForTests();
  process.env.OPENCODE_API_KEY = 'primary-key';
  process.env.OPENCODE_BASE_URL = 'https://primary.example/v1';
  process.env.GOOGLE_API_KEY = 'fallback-key';
  process.env.GOOGLE_BASE_URL = 'https://fallback.example';
  const modelRoute = [{ value: JSON.stringify({ provider: 'opencode_zen', modelId: 'gemini-3-flash' }) }];
  const providerChain = [{ providerId: 'google' }];
  installDb(createFakeDb({
    selects: [
      [{ user: { id: 'user_1', email: 'pablo@example.com', role: 'user' }, tokenId: 'token_1' }],
      [{ id: 1 }],
      [{ free: 1000, paid: 0 }],
      modelRoute,
      providerChain,
    ],
  }));

  globalThis.fetch = async (input: string | URL | Request) => {
    const url = String(input instanceof Request ? input.url : input);
    if (url === 'https://models.dev/api.json') {
      return Response.json({ opencode: { models: { 'gemini-3-flash': { id: 'gemini-3-flash', cost: { input: 0.3, output: 2.5 } } } }, google: { models: { 'gemini-3-flash': { id: 'gemini-3-flash', cost: { input: 0.3, output: 2.5 } } } } });
    }
    return Response.json({ error: { type: 'server_error', message: 'Down' } }, { status: 503 });
  };

  const response = await postChatCompletion(routeContext(authorizedChatRequest()));

  // Both providers 5xx, last provider's error passes through with compatibility headers
  assert.equal(response.status, 503);
  assert.ok(response.headers.get('x-request-id'));
  assert.ok(response.headers.get('x-nevermind-backend-version'));
});

test('proxy failover: 4xx errors do not trigger failover', async () => {
  resetPricingCacheForTests();
  installModelsDevFetch();
  process.env.OPENCODE_API_KEY = 'primary-key';
  process.env.OPENCODE_BASE_URL = 'https://primary.example/v1';
  process.env.GOOGLE_API_KEY = 'fallback-key';
  const modelRoute = [{ value: JSON.stringify({ provider: 'opencode_zen', modelId: 'gemini-3-flash' }) }];
  const providerChain = [{ providerId: 'google' }];
  installDb(createFakeDb({
    selects: [
      [{ user: { id: 'user_1', email: 'pablo@example.com', role: 'user' }, tokenId: 'token_1' }],
      [{ id: 1 }],
      [{ free: 1000, paid: 0 }],
      modelRoute,
      providerChain,
    ],
  }));

  let fallbackCalled = false;
  globalThis.fetch = async (input: string | URL | Request) => {
    const url = String(input instanceof Request ? input.url : input);
    if (url === 'https://models.dev/api.json') {
      return Response.json({ opencode: { models: { 'gemini-3-flash': { id: 'gemini-3-flash', cost: { input: 0.3, output: 2.5 } } } }, google: { models: { 'gemini-3-flash': { id: 'gemini-3-flash', cost: { input: 0.3, output: 2.5 } } } } });
    }
    if (url.startsWith('https://primary.example')) {
      return Response.json({ error: { message: 'Bad request' } }, { status: 400 });
    }
    fallbackCalled = true;
    return Response.json({}, { status: 200 });
  };

  const response = await postChatCompletion(routeContext(authorizedChatRequest()));

  assert.equal(response.status, 400);
  assert.ok(!fallbackCalled, 'fallback should NOT be called on 4xx');
});

test('proxy failover: ai_failover kill switch disables failover', async () => {
  process.env.NEVERMIND_KILL_SWITCHES = 'ai_failover';
  resetPricingCacheForTests();
  installModelsDevFetch();
  process.env.OPENCODE_API_KEY = 'primary-key';
  process.env.OPENCODE_BASE_URL = 'https://primary.example/v1';
  const modelRoute = [{ value: JSON.stringify({ provider: 'opencode_zen', modelId: 'gemini-3-flash' }) }];
  installDb(createFakeDb({
    selects: [
      [{ user: { id: 'user_1', email: 'pablo@example.com', role: 'user' }, tokenId: 'token_1' }],
      [{ id: 1 }],
      [{ free: 1000, paid: 0 }],
      modelRoute,
      [],
    ],
  }));

  let primaryCalled = false;
  globalThis.fetch = async (input: string | URL | Request) => {
    const url = String(input instanceof Request ? input.url : input);
    if (url === 'https://models.dev/api.json') {
      return Response.json({ opencode: { models: { 'gemini-3-flash': { id: 'gemini-3-flash', cost: { input: 0.3, output: 2.5 } } } } });
    }
    primaryCalled = true;
    return new Response('Service Unavailable', { status: 503 });
  };

  const response = await postChatCompletion(routeContext(authorizedChatRequest()));

  assert.equal(response.status, 503);
  assert.ok(primaryCalled);
  // With kill switch, the 503 is passed through (not chain-exhausted)
  assert.equal(response.status, 503);
});

test('proxy failover: format-incompatible provider is skipped', async () => {
  resetPricingCacheForTests();
  installModelsDevFetch();
  process.env.OPENCODE_API_KEY = 'primary-key';
  process.env.OPENCODE_BASE_URL = 'https://primary.example/v1';
  process.env.OPENAI_API_KEY = 'openai-key';
  process.env.OPENAI_BASE_URL = 'https://openai.example/v1';
  const modelRoute = [{ value: JSON.stringify({ provider: 'opencode_zen', modelId: 'claude-sonnet-4-6' }) }];
  const providerChain = [{ providerId: 'openai' }];
  installDb(createFakeDb({
    selects: [
      [{ user: { id: 'user_1', email: 'pablo@example.com', role: 'user' }, tokenId: 'token_1' }],
      [{ id: 1 }],
      [{ free: 1000, paid: 0 }],
      modelRoute,
      providerChain,
    ],
  }));

  let openaiCalled = false;
  let primaryCalled = false;
  const originalFetch = globalThis.fetch;
  globalThis.fetch = async (input: string | URL | Request) => {
    const url = String(input instanceof Request ? input.url : input);
    if (url === 'https://models.dev/api.json') {
      return Response.json({
        opencode: { models: { 'claude-sonnet-4-6': { id: 'claude-sonnet-4-6', cost: { input: 3, output: 15 } } } },
        openai: { models: { 'claude-sonnet-4-6': { id: 'claude-sonnet-4-6', cost: { input: 3, output: 15 } } } },
      });
    }
    if (url.startsWith('https://primary.example')) {
      primaryCalled = true;
      return new Response('Service Unavailable', { status: 503 });
    }
    openaiCalled = true;
    return Response.json({}, { status: 200 });
  };

  const response = await postMessages(routeContext(authorizedMessagesRequest()));

  assert.equal(response.status, 503);
  assert.ok(primaryCalled);
  assert.ok(!openaiCalled, 'openai should be skipped because it does not support anthropic-messages format');
  globalThis.fetch = originalFetch;
});

function authorizedMessagesRequest(body: unknown = { model: 'placeholder', messages: [{ role: 'user', content: 'hello' }] }) {
  return new Request('https://api.nvm.fyi/api/v1/messages', {
    method: 'POST',
    headers: {
      'x-api-key': 'nvm_pat_test',
      'content-type': 'application/json',
      'x-nevermind-client': 'desktop',
      'x-nevermind-client-version': '0.6.2',
      'x-nevermind-api-version': '1',
    },
    body: JSON.stringify(body),
  });
}

let postMessages: typeof postChatCompletion;
postMessages = async (ctx: Parameters<typeof postChatCompletion>[0]) => {
  const { POST } = await import('./v1/messages');
  return POST(ctx);
};

test('proxy failover: chain exhaustion when all providers are skipped returns upstream_unavailable', async () => {
  resetPricingCacheForTests();
  process.env.OPENAI_API_KEY = 'openai-key';
  process.env.OPENAI_BASE_URL = 'https://openai.example/v1';
  // Primary: openai with gemini model → google-generative-ai format.
  // openai only supports openai-completions → format-skipped → chain exhausted.
  const modelRoute = [{ value: JSON.stringify({ provider: 'openai', modelId: 'gemini-3-flash' }) }];
  installDb(createFakeDb({
    selects: [
      [{ user: { id: 'user_1', email: 'pablo@example.com', role: 'user' }, tokenId: 'token_1' }],
      [{ id: 1 }],
      [{ free: 1000, paid: 0 }],
      modelRoute,
      [],
    ],
  }));

  globalThis.fetch = async (input: string | URL | Request) => {
    const url = String(input instanceof Request ? input.url : input);
    if (url === 'https://models.dev/api.json') {
      return Response.json({ openai: { models: { 'gemini-3-flash': { id: 'gemini-3-flash', cost: { input: 0.3, output: 2.5 } } } } });
    }
    return Response.json({}, { status: 200 });
  };

  const response = await postChatCompletion(routeContext(authorizedChatRequest()));
  const body = await response.json() as any;

  assert.equal(response.status, 503);
  assert.equal(body.error.type, 'upstream_unavailable');
  assert.equal(body.error.message, 'All configured upstream providers are unavailable');
  assert.ok(response.headers.get('x-request-id'));
});
