import assert from 'node:assert/strict';
import {
  createServer,
  type IncomingMessage,
  type ServerResponse,
} from 'node:http';
import fs from 'node:fs/promises';
import { createRequire } from 'node:module';
import os from 'node:os';
import path from 'node:path';
import test, { after, mock } from 'node:test';

const testDataRoot = await fs.mkdtemp(
  path.join(os.tmpdir(), 'nvm-pi-session-integration-'),
);
const fixtureEvidence: Array<Record<string, unknown>> = [];
const previousPiCodingAgentDir = process.env.PI_CODING_AGENT_DIR;
process.env.PI_CODING_AGENT_DIR = path.join(testDataRoot, 'pi-config');
const repoRoot =
  path.basename(process.cwd()) === 'backend'
    ? path.resolve(process.cwd(), '..', '..')
    : process.cwd();

mock.module('electron', {
  namedExports: {
    app: {
      getPath: () => testDataRoot,
      getAppPath: () => repoRoot,
      getVersion: () => 'test',
      isPackaged: false,
    },
    safeStorage: {
      isEncryptionAvailable: () => false,
      encryptString: (value: string) => Buffer.from(value),
      decryptString: (value: Buffer) => value.toString(),
    },
  },
});

const { createNevermindAi, PI_AI_COMPACTION_RESERVE_TOKENS } = await import(
  './ai'
);
const {
  clearNevermindAuthCacheForTests,
  setActiveNevermindAuthBaseUrl,
  setNevermindAuthFilePathForTests,
} = await import('../backend/nevermind-auth');
const byoKey = await import('../backend/byo-key');
const { MAX_INPUT_TOKENS, estimateInputTokensFromValue } = await import(
  '../../../backend/src/lib/limits'
);

after(async () => {
  clearNevermindAuthCacheForTests();
  setNevermindAuthFilePathForTests(null);
  byoKey.clearByoKeyCacheForTests();
  byoKey.setByoKeyFilePathForTests(null);
  if (previousPiCodingAgentDir === undefined)
    delete process.env.PI_CODING_AGENT_DIR;
  else process.env.PI_CODING_AGENT_DIR = previousPiCodingAgentDir;
  const evidenceDirectory = process.env.PI_INTEGRATION_EVIDENCE_DIR;
  if (evidenceDirectory) {
    const piRequire = createRequire(import.meta.url);
    const piEntry = piRequire.resolve('@earendil-works/pi-coding-agent');
    const piPackage = JSON.parse(
      await fs.readFile(
        path.resolve(path.dirname(piEntry), '../package.json'),
        'utf8',
      ),
    ) as { version: string };
    await fs.mkdir(evidenceDirectory, { recursive: true });
    await fs.writeFile(
      path.join(evidenceDirectory, 'integration-evidence.json'),
      `${JSON.stringify(
        {
          sdkVersion: piPackage.version,
          commitSha: process.env.GITHUB_SHA || 'local',
          outcomes: fixtureEvidence,
        },
        null,
        2,
      )}\n`,
    );
  }
  await fs.rm(testDataRoot, { recursive: true, force: true });
});

type CapturedRequest = {
  headers: IncomingMessage['headers'];
  body: Record<string, any>;
  responseStatus?: number;
};

type FixtureScenario =
  | 'builder'
  | 'builder-validation-failure'
  | 'request-too-large'
  | 'backend-estimator-boundary'
  | 'session-overflow'
  | 'summary-unauthorized'
  | 'summary-credits'
  | 'summary-hangs'
  | 'recovery-exhausted'
  | 'raw-provider-413'
  | 'builder-transient'
  | 'builder-retry-hangs'
  | 'general-transient'
  | 'queue-conversations'
  | 'complete';

async function readRequestBody(request: IncomingMessage) {
  const chunks: Buffer[] = [];
  for await (const chunk of request) chunks.push(Buffer.from(chunk));
  return Buffer.concat(chunks).toString('utf8');
}

function sendJson(response: ServerResponse, status: number, value: unknown) {
  response.writeHead(status, { 'content-type': 'application/json' });
  response.end(JSON.stringify(value));
}

function sendStream(response: ServerResponse, value: object) {
  response.writeHead(200, {
    'cache-control': 'no-cache',
    connection: 'keep-alive',
    'content-type': 'text/event-stream; charset=utf-8',
  });
  response.end(
    `data: ${JSON.stringify({
      id: 'fixture-response',
      object: 'chat.completion.chunk',
      created: 1,
      model: 'fixture-model',
      choices: [{ index: 0, ...value }],
    })}\n\ndata: ${JSON.stringify({
      id: 'fixture-response',
      object: 'chat.completion.chunk',
      created: 1,
      model: 'fixture-model',
      choices: [],
      usage: {
        prompt_tokens: 1_280,
        completion_tokens: 18,
        total_tokens: 1_298,
      },
    })}\n\ndata: [DONE]\n\n`,
  );
}

function sendText(response: ServerResponse, text: string) {
  sendStream(response, {
    delta: { content: text },
    finish_reason: 'stop',
  });
}

function sendAnthropicText(response: ServerResponse, text: string) {
  const events = [
    [
      'message_start',
      {
        type: 'message_start',
        message: {
          id: 'fixture-message',
          type: 'message',
          role: 'assistant',
          model: 'fixture-model',
          content: [],
          stop_reason: null,
          stop_sequence: null,
          usage: { input_tokens: 12, output_tokens: 0 },
        },
      },
    ],
    [
      'content_block_start',
      {
        type: 'content_block_start',
        index: 0,
        content_block: { type: 'text', text: '' },
      },
    ],
    [
      'content_block_delta',
      {
        type: 'content_block_delta',
        index: 0,
        delta: { type: 'text_delta', text },
      },
    ],
    ['content_block_stop', { type: 'content_block_stop', index: 0 }],
    [
      'message_delta',
      {
        type: 'message_delta',
        delta: { stop_reason: 'end_turn', stop_sequence: null },
        usage: { output_tokens: 12 },
      },
    ],
    ['message_stop', { type: 'message_stop' }],
  ];
  response.writeHead(200, {
    'cache-control': 'no-cache',
    connection: 'keep-alive',
    'content-type': 'text/event-stream; charset=utf-8',
  });
  response.end(
    events
      .map(
        ([event, data]) => `event: ${event}\ndata: ${JSON.stringify(data)}\n\n`,
      )
      .join(''),
  );
}

function sendGoogleText(response: ServerResponse, text: string) {
  response.writeHead(200, {
    'cache-control': 'no-cache',
    connection: 'keep-alive',
    'content-type': 'text/event-stream; charset=utf-8',
  });
  response.end(
    `data: ${JSON.stringify({
      candidates: [
        {
          content: { parts: [{ text }], role: 'model' },
          finishReason: 'STOP',
          index: 0,
        },
      ],
      usageMetadata: {
        promptTokenCount: 12,
        candidatesTokenCount: 12,
        totalTokenCount: 24,
      },
    })}\n\n`,
  );
}

function sendTextForApi(response: ServerResponse, api: string, text: string) {
  if (api === 'anthropic-messages') return sendAnthropicText(response, text);
  if (api === 'google-generative-ai') return sendGoogleText(response, text);
  return sendText(response, text);
}

function sendBuilderToolCall(response: ServerResponse, params = '{}') {
  sendToolCall(response, 'read_extension_api', params);
}

function sendToolCall(response: ServerResponse, name: string, params = '{}') {
  sendStream(response, {
    delta: {
      tool_calls: [
        {
          index: 0,
          id: `call_${name}_${params === '{}' ? 'first' : 'continued'}`,
          type: 'function',
          function: { name, arguments: params },
        },
      ],
    },
    finish_reason: 'tool_calls',
  });
}

function sendPromptTooLarge(response: ServerResponse) {
  sendJson(response, 413, {
    error: {
      type: 'prompt_too_large',
      message: 'Prompt exceeds 100000 input tokens; context_length_exceeded',
    },
  });
}

function sendPromptTooLargeForApi(
  response: ServerResponse,
  api: string,
  includeLegacyOverflowHint = true,
) {
  const message = includeLegacyOverflowHint
    ? 'Prompt exceeds 100000 input tokens; context_length_exceeded'
    : 'Prompt exceeds 100000 input tokens';
  if (api === 'anthropic-messages') {
    sendJson(response, 413, {
      type: 'error',
      error: {
        type: 'invalid_request_error',
        message,
      },
    });
    return;
  }
  if (api === 'google-generative-ai') {
    sendJson(response, 413, {
      error: {
        code: 413,
        status: 'RESOURCE_EXHAUSTED',
        message,
      },
    });
    return;
  }
  sendJson(response, 413, {
    error: { type: 'prompt_too_large', message },
  });
}

function hasBuilderTools(body: Record<string, any>) {
  return Array.isArray(body.tools) && body.tools.length > 0;
}

function hasTrailingToolCompaction(version: string) {
  const [major = 0, minor = 0, patch = 0] = version
    .split('.')
    .map((part) => Number(part));
  return major > 0 || minor > 84 || (minor === 84 && patch >= 4);
}

async function installedPiCodingAgentVersion() {
  const piRequire = createRequire(import.meta.url);
  const piEntry = piRequire.resolve('@earendil-works/pi-coding-agent');
  const packageJson = JSON.parse(
    await fs.readFile(
      path.resolve(path.dirname(piEntry), '../package.json'),
      'utf8',
    ),
  ) as { version: string };
  return packageJson.version;
}

function containsApiReadResult(body: Record<string, any>) {
  return JSON.stringify(body.messages || []).includes('[Read page:');
}

function createProtocolFixture(
  scenario: FixtureScenario = 'builder',
  api = 'openai-completions',
) {
  const requests: CapturedRequest[] = [];
  const requestWaiters: Array<{ count: number; resolve: () => void }> = [];
  let rejectedLargeResult = false;
  let requestedContinuation = false;
  let summaryServed = false;
  const server = createServer(async (request, response) => {
    const bodyText = await readRequestBody(request);
    if (request.url === '/api/compatibility') {
      sendJson(response, 200, {
        backend: { environment: 'fixture', version: '1' },
        api: { currentVersion: 1, supportedVersions: [1] },
        desktop: {
          minimumSupportedVersion: '0.0.0',
          latestVersion: null,
          updateUrl: 'https://example.invalid/update',
        },
        client: { compatible: true, unsupportedReason: null },
        features: {},
      });
      return;
    }
    if (request.url?.startsWith('/api/v1/active-model')) {
      const contextWindow =
        scenario === 'backend-estimator-boundary'
          ? MAX_INPUT_TOKENS + PI_AI_COMPACTION_RESERVE_TOKENS + 20_000
          : 24_576;
      sendJson(response, 200, {
        id: 'fixture-model',
        name: 'Fixture Model',
        provider: 'openai',
        api: 'openai-completions',
        baseUrl: `${fixture.baseUrl}/v1`,
        contextWindow,
        maxInputTokens: contextWindow,
        maxTokens: 2_048,
        reasoning: false,
        thinkingLevel: 'off',
        input: ['text'],
        modelTier: 'pro',
        creditKind: 'paid',
        credits: { paid: 1000, free: 0, total: 1000 },
        notice: 'ok',
      });
      return;
    }
    const requestApi = request.url?.startsWith('/v1/chat/completions')
      ? 'openai-completions'
      : request.url?.startsWith('/v1/messages')
        ? 'anthropic-messages'
        : request.url?.startsWith('/v1beta/models/')
          ? 'google-generative-ai'
          : undefined;
    if (requestApi !== api) {
      sendJson(response, 404, { error: 'unknown fixture route' });
      return;
    }

    let body: Record<string, any>;
    try {
      body = JSON.parse(bodyText);
    } catch {
      sendJson(response, 400, { error: 'invalid request JSON' });
      return;
    }
    requests.push({ headers: request.headers, body });
    const callIndex = requests.length;
    for (const waiter of requestWaiters.splice(0)) {
      if (requests.length >= waiter.count) waiter.resolve();
      else requestWaiters.push(waiter);
    }

    if (
      scenario === 'session-overflow' ||
      scenario === 'summary-unauthorized' ||
      scenario === 'summary-credits' ||
      scenario === 'summary-hangs' ||
      scenario === 'recovery-exhausted' ||
      scenario === 'raw-provider-413'
    ) {
      if (
        callIndex === 1 ||
        (scenario === 'recovery-exhausted' && callIndex >= 3)
      ) {
        requests.at(-1)!.responseStatus = 413;
        sendPromptTooLargeForApi(
          response,
          api,
          scenario !== 'raw-provider-413',
        );
      } else if (callIndex === 2 && scenario === 'summary-unauthorized') {
        requests.at(-1)!.responseStatus = 401;
        sendJson(response, 401, {
          error: { type: 'invalid_api_key', message: 'Invalid API key' },
        });
      } else if (callIndex === 2 && scenario === 'summary-credits') {
        requests.at(-1)!.responseStatus = 402;
        sendJson(response, 402, {
          error: {
            type: 'insufficient_credits',
            message: 'No credits remaining',
          },
        });
      } else if (callIndex === 2 && scenario === 'summary-hangs') {
        return;
      } else if (callIndex === 2) {
        sendTextForApi(
          response,
          api,
          'The extension session context is summarized.',
        );
      } else {
        sendTextForApi(
          response,
          api,
          'The extension session recovered successfully.',
        );
      }
      return;
    }

    if (scenario === 'request-too-large') {
      if (callIndex === 2) {
        sendText(
          response,
          'The rejected request body was not summarized as context.',
        );
      } else {
        requests.at(-1)!.responseStatus = 413;
        sendJson(response, 413, {
          error: {
            type: 'request_too_large',
            message: 'AI request bodies must be 4 MB or smaller.',
          },
        });
      }
      return;
    }

    if (
      scenario === 'builder-transient' ||
      scenario === 'builder-retry-hangs' ||
      scenario === 'general-transient'
    ) {
      const transientFailureLimit = scenario === 'builder-transient' ? 2 : 1;
      if (callIndex <= transientFailureLimit) {
        requests.at(-1)!.responseStatus = 500;
        sendJson(response, 500, {
          error: { type: 'server_error', message: 'Fixture temporary failure' },
        });
      } else if (scenario === 'builder-retry-hangs') {
        return;
      } else {
        sendText(response, 'The transient provider failure recovered.');
      }
      return;
    }

    if (scenario === 'complete') {
      sendText(response, 'The fixture request completed.');
      return;
    }

    if (scenario === 'queue-conversations') {
      const sendQueuedResponse = () =>
        sendText(response, `Queued request ${callIndex} completed.`);
      if (callIndex === 1) setTimeout(sendQueuedResponse, 100);
      else sendQueuedResponse();
      return;
    }

    if (scenario === 'backend-estimator-boundary') {
      if (summaryServed) {
        sendText(response, 'The requested boundary operation is complete.');
        return;
      }
      if (!hasBuilderTools(body)) {
        summaryServed = true;
        sendText(response, 'The oversized builder history is summarized.');
        return;
      }
      const backendEstimate = estimateInputTokensFromValue(body);
      if (backendEstimate >= MAX_INPUT_TOKENS) {
        rejectedLargeResult = true;
        requests.at(-1)!.responseStatus = 413;
        sendPromptTooLarge(response);
        return;
      }
      sendBuilderToolCall(response, JSON.stringify({ offset: 0 }));
      return;
    }

    if (scenario === 'builder-validation-failure') {
      if (callIndex === 1) {
        sendToolCall(
          response,
          'read_extension',
          JSON.stringify({ filename: 'existing.ts' }),
        );
      } else if (callIndex === 2) {
        sendToolCall(
          response,
          'write_extension',
          JSON.stringify({
            filename: 'existing.ts',
            code: 'export default { this is not valid TypeScript };',
          }),
        );
      } else {
        sendText(response, 'The invalid update was rejected safely.');
      }
      return;
    }

    if (callIndex === 1) {
      sendBuilderToolCall(response);
      return;
    }

    if (
      hasBuilderTools(body) &&
      containsApiReadResult(body) &&
      !rejectedLargeResult
    ) {
      rejectedLargeResult = true;
      requests.at(-1)!.responseStatus = 413;
      sendPromptTooLarge(response);
      return;
    }

    if (!hasBuilderTools(body)) {
      sendText(response, 'The conversation context is summarized.');
      return;
    }

    if (!requestedContinuation) {
      requestedContinuation = true;
      const nextOffset = fixture.nextOffset;
      if (typeof nextOffset !== 'number') {
        sendJson(response, 500, {
          error: 'read page did not offer continuation',
        });
        return;
      }
      sendBuilderToolCall(response, JSON.stringify({ offset: nextOffset }));
      return;
    }

    sendText(response, 'The requested extension operation is complete.');
  });

  const fixture = {
    server,
    requests,
    baseUrl: '',
    nextOffset: undefined as number | undefined,
    async listen() {
      await new Promise<void>((resolve) =>
        server.listen(0, '127.0.0.1', resolve),
      );
      const address = server.address();
      if (!address || typeof address === 'string')
        throw new Error('Pi protocol fixture did not bind a TCP port.');
      fixture.baseUrl = `http://127.0.0.1:${address.port}`;
      return fixture.baseUrl;
    },
    async close() {
      server.closeAllConnections();
      await new Promise<void>((resolve, reject) =>
        server.close((error) => (error ? reject(error) : resolve())),
      );
    },
    waitForRequests(count: number) {
      if (requests.length >= count) return Promise.resolve();
      return new Promise<void>((resolve) => {
        requestWaiters.push({ count, resolve });
      });
    },
  };
  return fixture;
}

async function configureAuth(baseUrl: string, suffix: string) {
  const filePath = path.join(testDataRoot, `auth-${suffix}.json`);
  await fs.writeFile(
    filePath,
    JSON.stringify({
      [baseUrl]: {
        email: 'fixture@example.invalid',
        role: 'pro',
        baseUrl,
        token: Buffer.from('fixture-token').toString('base64'),
      },
    }),
  );
  setNevermindAuthFilePathForTests(filePath);
  setActiveNevermindAuthBaseUrl(baseUrl);
}

function createAi(
  fixture: ReturnType<typeof createProtocolFixture>,
  events: any[],
  activeExtensionFile?: string,
) {
  return createNevermindAi({
    agentDir: path.join(testDataRoot, 'agent'),
    workspaceDir: path.join(testDataRoot, 'workspace'),
    extensionsDir: path.join(testDataRoot, 'extensions'),
    extensionDraftsDir: path.join(testDataRoot, 'drafts'),
    extensionApiPath: path.join(
      repoRoot,
      'src/app/resources/nevermind-extension-api.d.ts',
    ),
    extensionTypesPath: path.join(
      repoRoot,
      'src/app/resources/nevermind-extension-api.d.ts',
    ),
    skillPath: path.join(
      repoRoot,
      'src/app/resources/skills/nevermind-extension-builder/SKILL.md',
    ),
    reloadExtensions: () => {},
    getActiveChat: () =>
      activeExtensionFile
        ? { generatedExtensionFile: activeExtensionFile }
        : null,
    activateGeneratedExtension: async (filename) => ({
      filename,
      preview: {
        extensionId: 'fixture',
        rootItems: [],
        commands: [],
        actions: [],
      },
    }),
    canWriteExtension: () => true,
    onEvent: (event) => {
      events.push(event);
      if (
        event.type === 'tool_trace_end' &&
        event.name === 'read_extension_api'
      ) {
        try {
          const details = JSON.parse(
            String(event.data?.output?.details || '{}'),
          );
          fixture.nextOffset = details.nextOffset ?? undefined;
        } catch {}
      }
    },
  });
}

function terminalEvents(events: Array<{ type: string }>) {
  return events.filter((event) =>
    ['done', 'error', 'aborted'].includes(event.type),
  );
}

async function withFixture(
  scenario: FixtureScenario,
  suffix: string,
  run: (fixture: ReturnType<typeof createProtocolFixture>) => Promise<void>,
  api = 'openai-completions',
) {
  const fixture = createProtocolFixture(scenario, api);
  await fixture.listen();
  await configureAuth(fixture.baseUrl, suffix);
  byoKey.clearByoKeyCacheForTests();
  try {
    await run(fixture);
  } finally {
    await fixture.close();
    clearNevermindAuthCacheForTests();
    setNevermindAuthFilePathForTests(null);
    byoKey.clearByoKeyCacheForTests();
    byoKey.setByoKeyFilePathForTests(null);
  }
}

test('the real builder adapter bounds and continues API reads, recovers a backend-shaped 413, and accepts a follow-up', async (context) => {
  await withFixture('builder', 'proactive', async (fixture) => {
    const events: Array<{ type: string; name?: string; data?: any }> = [];
    const ai = createAi(fixture, events);
    try {
      await ai.send(
        'The user confirmed the extension request. Read the API, then summarize the requested change.',
        'large-builder-chat',
      );
      assert.deepEqual(
        terminalEvents(events).map((event) => event.type),
        ['done'],
      );
      assert.ok(
        events.some(
          (event) =>
            event.type === 'tool_trace_end' &&
            event.name === 'read_extension_api' &&
            Number(event.data?.output?.textBytes) > 30_000,
        ),
        'the real builder read tool should return a substantial bounded page',
      );
      const apiReadEvents = events.filter(
        (event) =>
          event.type === 'tool_trace_end' &&
          event.name === 'read_extension_api',
      );
      assert.equal(apiReadEvents.length, 2);
      const pageDetails = apiReadEvents.map((event) =>
        JSON.parse(String(event.data?.output?.details || '{}')),
      );
      assert.ok(pageDetails[0].nextOffset > 0);
      assert.notEqual(pageDetails[1].totalLines, undefined);
      assert.equal(
        fixture.requests.filter((request) => request.responseStatus === 413)
          .length,
        1,
      );
      assert.ok(fixture.requests.length >= 4);
      const piRequire = createRequire(import.meta.url);
      const piEntry = piRequire.resolve('@earendil-works/pi-coding-agent');
      const piPackage = JSON.parse(
        await fs.readFile(
          path.resolve(path.dirname(piEntry), '../package.json'),
          'utf8',
        ),
      ) as { version: string };
      if (!hasTrailingToolCompaction(piPackage.version)) {
        assert.ok(
          hasBuilderTools(fixture.requests[1].body) &&
            containsApiReadResult(fixture.requests[1].body),
          'locked Pi should use overflow fallback after the large API result',
        );
      } else {
        assert.ok(
          !hasBuilderTools(fixture.requests[1].body),
          'upgraded Pi should compact before its next normal request',
        );
      }
      assert.ok(
        fixture.requests.every(
          (request) =>
            request.headers.authorization === 'Bearer fixture-token' &&
            request.headers['x-nevermind-ai-model'] === 'smart' &&
            request.headers['x-nevermind-ai-model-tier'] === 'pro' &&
            request.headers['x-nevermind-ai-credit-kind'] === 'paid',
        ),
        'normal and summary requests must preserve managed routing and billing headers',
      );
      const packagePi = await import('@earendil-works/pi-coding-agent');
      const summaryRequest = fixture.requests.find(
        (request) => !hasBuilderTools(request.body),
      );
      assert.ok(summaryRequest, 'Pi should make a summary request');
      const piSessionBudget = events.filter(
        (event) =>
          event.type === 'debug' && event.label === 'pi-session-budget',
      );
      assert.ok(piSessionBudget.length > 0);
      assert.ok(
        Number(piSessionBudget.at(-1)?.data?.reportedInputTokens) > 0,
        'Pi session diagnostics should include provider-reported usage',
      );
      const tokenEvidence = fixture.requests.map((request) => ({
        backendEstimate: estimateInputTokensFromValue(request.body),
        toolCount: Array.isArray(request.body.tools)
          ? request.body.tools.length
          : 0,
        requestBytes: Buffer.byteLength(JSON.stringify(request.body), 'utf8'),
      }));
      const budgetEvidence = {
        piEstimatedContextTokens:
          piSessionBudget.at(-1)?.data?.piEstimatedContextTokens ?? null,
        reportedInputTokens:
          piSessionBudget.at(-1)?.data?.reportedInputTokens ?? 0,
        effectiveContextWindow:
          piSessionBudget.at(-1)?.data?.contextWindow ?? 24_576,
        reserveTokens: PI_AI_COMPACTION_RESERVE_TOKENS,
        piProactiveThreshold:
          Number(piSessionBudget.at(-1)?.data?.contextWindow ?? 24_576) -
          PI_AI_COMPACTION_RESERVE_TOKENS,
        summaryRequestEstimate: estimateInputTokensFromValue(
          summaryRequest.body,
        ),
      };
      context.diagnostic(
        JSON.stringify({
          sdk: packagePi.AgentSession.name,
          version: piPackage.version,
          defaultMaxBytes: packagePi.DEFAULT_MAX_BYTES,
          reserveTokens: PI_AI_COMPACTION_RESERVE_TOKENS,
          piSessionBudget: piSessionBudget.at(-1)?.data,
          effectiveContextWindow:
            piSessionBudget.at(-1)?.data?.contextWindow ?? 24_576,
          piEstimatedContextTokens:
            piSessionBudget.at(-1)?.data?.piEstimatedContextTokens ?? null,
          reportedInputTokens:
            piSessionBudget.at(-1)?.data?.reportedInputTokens ?? 0,
          ...budgetEvidence,
          tokenEvidence,
        }),
      );
      assert.ok(
        apiReadEvents.every(
          (event) =>
            Number(event.data?.output?.textBytes) <=
            packagePi.DEFAULT_MAX_BYTES,
        ),
        'every page returned to Pi must respect its exported byte ceiling',
      );
      assert.ok(
        events.some(
          (event) =>
            event.type === 'delta' &&
            String((event as { text?: string }).text || '').includes(
              'The requested extension operation is complete.',
            ),
        ),
        'the completed builder response should stream through the actual adapter',
      );
      assert.ok(
        fixture.requests[0].body.tools.some(
          (tool: { function?: { name?: string } }) =>
            tool.function?.name === 'read_extension_api',
        ),
      );
      const builderToolNames = fixture.requests[0].body.tools.map(
        (tool: { function?: { name?: string } }) => tool.function?.name,
      );
      const registeredToolEvent = events.find(
        (event) => event.type === 'debug' && event.label === 'registered-tools',
      );
      assert.ok(Array.isArray(registeredToolEvent?.data));
      const registeredToolNames = registeredToolEvent.data as string[];
      const proactiveCompaction = hasTrailingToolCompaction(piPackage.version)
        ? !hasBuilderTools(fixture.requests[1].body)
        : false;
      const overflowFallback = !hasTrailingToolCompaction(piPackage.version)
        ? hasBuilderTools(fixture.requests[1].body) &&
          containsApiReadResult(fixture.requests[1].body)
        : false;
      const raw413Recovered = fixture.requests.some(
        (request) => request.responseStatus === 413,
      );
      const allowedBuilderTools = new Set([
        'read_extension_api',
        'list_capabilities',
        'list_shortcuts',
        'read_app_logs',
        'list_extensions',
        'read_extension',
        'read_current_extension',
        'write_extension',
        'remove_extension',
        'validate_extension',
        'install_extension',
        'web_search',
        'source_check',
        'fetch_content',
        'get_search_content',
      ]);
      assert.ok(
        [
          'read_extension_api',
          'list_capabilities',
          'list_shortcuts',
          'read_app_logs',
          'list_extensions',
          'read_extension',
          'read_current_extension',
          'write_extension',
          'remove_extension',
          'validate_extension',
          'install_extension',
        ].every((name) => builderToolNames.includes(name)),
        'all intended Nevermind builder tools should be active',
      );
      assert.ok(
        builderToolNames.every((name) => allowedBuilderTools.has(name)),
        `the builder loaded an unintended tool: ${builderToolNames.find((name) => !allowedBuilderTools.has(name))}`,
      );
      assert.ok(
        registeredToolNames.every((name) => allowedBuilderTools.has(name)),
        `the builder registered an undeclared or deferred tool: ${registeredToolNames.find((name) => !allowedBuilderTools.has(name))}`,
      );
      assert.ok(
        !registeredToolNames.some((name) =>
          ['tool_search', 'codemode', 'mcp'].some((forbidden) =>
            name.toLowerCase().includes(forbidden),
          ),
        ),
        'the builder must not load tool search, codemode, or MCP tools',
      );
      fixtureEvidence.push({
        scenario:
          'builder-proactive-compaction-overflow-continuation-follow-up',
        finalOutcome: 'done',
        proactiveCompaction,
        overflowFallback,
        raw413Recovered,
        providerCalls: fixture.requests.length,
        tokenEvidence,
        pageCount: apiReadEvents.length,
        toolAllowlist: builderToolNames,
        registeredToolAllowlist: registeredToolNames,
        managedHeadersPreserved: true,
        budgetEvidence,
      });
      for (const forbidden of [
        'bash',
        'edit',
        'write',
        'tool_search',
        'codemode',
        'mcp',
      ])
        assert.equal(
          fixture.requests[0].body.tools.some(
            (tool: { function?: { name?: string } }) =>
              tool.function?.name === forbidden,
          ),
          false,
          `the builder must not expose ${forbidden}`,
        );

      events.length = 0;
      await ai.send(
        'Make one more small change and explain it.',
        'large-builder-chat',
      );
      assert.deepEqual(
        terminalEvents(events).map((event) => event.type),
        ['done'],
      );
    } finally {
      await ai.disposeAllSessions();
    }
  });
});

test('backend token admission can reject below Pi proactive compaction threshold and Pi recovers', async () => {
  await withFixture(
    'backend-estimator-boundary',
    'backend-estimator-boundary',
    async (fixture) => {
      const events: Array<{
        type: string;
        label?: string;
        data?: any;
        text?: string;
      }> = [];
      const ai = createAi(fixture, events);
      try {
        await ai.send(
          'Read the extension API repeatedly while preserving this request. After recovery, finish the requested change.',
          'backend-estimator-boundary-chat',
        );
        assert.deepEqual(
          terminalEvents(events).map((event) => event.type),
          ['done'],
        );
        const rejectedRequest = fixture.requests.find(
          (request) => request.responseStatus === 413,
        );
        assert.ok(
          rejectedRequest,
          'the fixture backend should reject at its token cap',
        );
        const backendEstimate = estimateInputTokensFromValue(
          rejectedRequest.body,
        );
        const budgetEvent = events.find(
          (event) =>
            event.type === 'debug' && event.label === 'pi-session-budget',
        );
        const effectiveContextWindow = Number(budgetEvent?.data?.contextWindow);
        const piThreshold =
          effectiveContextWindow - PI_AI_COMPACTION_RESERVE_TOKENS;
        assert.ok(backendEstimate >= MAX_INPUT_TOKENS);
        assert.ok(
          backendEstimate < piThreshold,
          `backend estimate ${backendEstimate} should remain below Pi threshold ${piThreshold}`,
        );
        assert.ok(fixture.requests.length > 3);
        assert.match(
          String(events.find((event) => event.type === 'delta')?.text),
          /boundary operation is complete/,
        );
        fixtureEvidence.push({
          scenario: 'backend-admission-below-pi-proactive-threshold',
          finalOutcome: 'done',
          raw413Recovered: true,
          providerCalls: fixture.requests.length,
          backendEstimate,
          backendLimit: MAX_INPUT_TOKENS,
          effectiveContextWindow,
          reserveTokens: PI_AI_COMPACTION_RESERVE_TOKENS,
          piProactiveThreshold: piThreshold,
        });
      } finally {
        await ai.disposeAllSessions();
      }
    },
  );
});

test('invalid Pi builder output leaves the current live extension untouched', async () => {
  await withFixture(
    'builder-validation-failure',
    'validation-failure',
    async (fixture) => {
      const extensionsDir = path.join(testDataRoot, 'extensions');
      const liveSource = [
        'export default {',
        '  id: "fixture-live-extension",',
        '  commands: [],',
        '} satisfies NevermindExtension;',
        '',
      ].join('\n');
      await fs.mkdir(extensionsDir, { recursive: true });
      const livePath = path.join(extensionsDir, 'existing.ts');
      await fs.writeFile(livePath, liveSource);
      const events: Array<{
        type: string;
        name?: string;
        isError?: boolean;
      }> = [];
      const ai = createAi(fixture, events, 'existing.ts');
      try {
        await ai.send(
          'Read existing.ts, then try the requested update. If validation rejects it, report the failure.',
          'validation-failure-chat',
        );
        assert.deepEqual(
          terminalEvents(events).map((event) => event.type),
          ['done'],
        );
        assert.equal(await fs.readFile(livePath, 'utf8'), liveSource);
        assert.ok(
          events.some(
            (event) =>
              event.type === 'tool_trace_end' &&
              event.name === 'write_extension' &&
              event.isError === true,
          ),
          'the actual builder tool should report its validation failure',
        );
        assert.equal(
          events.some((event) => event.type === 'extension_activated'),
          false,
        );
        fixtureEvidence.push({
          scenario: 'invalid-generated-extension-preserves-live-source',
          finalOutcome: 'done',
          providerCalls: fixture.requests.length,
          validationFailurePreservedLiveSource: true,
        });
      } finally {
        await ai.disposeAllSessions();
      }
    },
  );
});

test('a byte-size 413 retains its distinct host notice after Pi recovery', async () => {
  await withFixture(
    'request-too-large',
    'request-too-large',
    async (fixture) => {
      const events: Array<{ type: string; data?: any }> = [];
      const ai = createAi(fixture, events);
      try {
        await ai.send(
          'Send a request with an oversized input.',
          'request-too-large-chat',
        );
        const terminal = terminalEvents(events);
        assert.deepEqual(
          terminal.map((event) => event.type),
          ['error'],
        );
        assert.equal(terminal[0].data?.kind, 'request_too_large');
        fixtureEvidence.push({
          scenario: 'byte-size-413-keeps-request-too-large-notice',
          finalOutcome: 'error',
          noticeKind: terminal[0].data?.kind,
        });
      } finally {
        await ai.disposeAllSessions();
      }
    },
  );
});

test('extension sessions keep overflow recovery, final status, and BYO routing inside Pi', async () => {
  await withFixture(
    'session-overflow',
    'extension-session',
    async (fixture) => {
      const byoPath = path.join(testDataRoot, 'byo-extension-session.json');
      byoKey.setByoKeyFilePathForTests(byoPath);
      await byoKey.persistByoKey({
        providerId: 'openai',
        apiKey: 'byo-fixture-token',
        provider: 'openai',
        api: 'openai-completions',
        baseUrl: `${fixture.baseUrl}/v1`,
        modelId: 'fixture-model',
        modelName: 'Fixture Model',
      });
      const ai = createAi(fixture, []);
      const events: Array<{ type: string }> = [];
      try {
        const response = await ai
          .session('extension-overflow-session', {
            model: 'fast',
            toolMode: 'none',
          })
          .ask('Continue this extension conversation after context recovery.', {
            images: [
              {
                type: 'image',
                data: 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/2WQAAAAASUVORK5CYII=',
                mimeType: 'image/png',
              },
            ],
            onEvent: (event) => events.push(event),
          });
        assert.match(String(response), /recovered successfully/);
        assert.deepEqual(
          terminalEvents(events).map((event) => event.type),
          ['done'],
        );
        assert.equal(fixture.requests.length, 3);
        assert.ok(
          JSON.stringify(fixture.requests[0].body.messages).includes(
            'iVBORw0KGgo',
          ),
          'the provider request should preserve supported image input',
        );
        const byoTokenEstimate = estimateInputTokensFromValue(
          fixture.requests[0].body,
        );
        assert.ok(byoTokenEstimate > 0);
        assert.ok(
          fixture.requests.every(
            (request) =>
              request.headers.authorization === 'Bearer byo-fixture-token' &&
              !request.headers['x-nevermind-ai-model-tier'] &&
              !request.headers['x-nevermind-ai-credit-kind'],
          ),
          'BYO credentials must be retained for both summary and recovered requests',
        );
        fixtureEvidence.push({
          scenario: 'byo-extension-session-overflow-recovery',
          finalOutcome: 'done',
          providerCalls: fixture.requests.length,
          imageRequestEstimate: byoTokenEstimate,
          byoRoutingPreserved: true,
        });
      } finally {
        await ai.disposeAllSessions();
      }
    },
  );
});

test('configured provider adapters recover the backend-compatible token-limit 413', async () => {
  const providers = [
    {
      provider: 'openai',
      api: 'openai-completions',
      baseUrl: (fixture: ReturnType<typeof createProtocolFixture>) =>
        `${fixture.baseUrl}/v1`,
    },
    {
      provider: 'anthropic',
      api: 'anthropic-messages',
      baseUrl: (fixture: ReturnType<typeof createProtocolFixture>) =>
        fixture.baseUrl,
    },
    {
      provider: 'google',
      api: 'google-generative-ai',
      baseUrl: (fixture: ReturnType<typeof createProtocolFixture>) =>
        `${fixture.baseUrl}/v1beta`,
    },
  ];

  for (const provider of providers) {
    await withFixture(
      'session-overflow',
      `raw-413-${provider.provider}`,
      async (fixture) => {
        const byoPath = path.join(
          testDataRoot,
          `byo-raw-413-${provider.provider}.json`,
        );
        byoKey.setByoKeyFilePathForTests(byoPath);
        await byoKey.persistByoKey({
          providerId: provider.provider,
          apiKey: 'byo-fixture-token',
          provider: provider.provider,
          api: provider.api,
          baseUrl: provider.baseUrl(fixture),
          modelId:
            provider.provider === 'anthropic'
              ? 'claude-fixture'
              : provider.provider === 'google'
                ? 'gemini-fixture'
                : 'fixture-model',
          modelName: 'Fixture Model',
        });
        const ai = createAi(fixture, []);
        const events: Array<{ type: string }> = [];
        try {
          const response = await ai
            .session(`raw-413-${provider.provider}`, {
              model: 'fast',
              toolMode: 'none',
            })
            .ask('Recover this provider request after Pi compacts.', {
              onEvent: (event) => events.push(event),
            });
          assert.match(String(response), /recovered successfully/);
          assert.equal(fixture.requests.length, 3);
          assert.deepEqual(
            terminalEvents(events).map((event) => event.type),
            ['done'],
          );
          assert.equal(fixture.requests[0].responseStatus, 413);
          fixtureEvidence.push({
            scenario: `provider-413-${provider.api}`,
            finalOutcome: 'done',
            providerCalls: fixture.requests.length,
            raw413Recovered: true,
          });
        } finally {
          await ai.disposeAllSessions();
        }
      },
      provider.api,
    );
  }
});

test('configured provider adapters classify unmodified 413 responses by the installed Pi version', async () => {
  const providers = [
    {
      provider: 'openai',
      api: 'openai-completions',
      baseUrl: (fixture: ReturnType<typeof createProtocolFixture>) =>
        `${fixture.baseUrl}/v1`,
    },
    {
      provider: 'anthropic',
      api: 'anthropic-messages',
      baseUrl: (fixture: ReturnType<typeof createProtocolFixture>) =>
        fixture.baseUrl,
    },
    {
      provider: 'google',
      api: 'google-generative-ai',
      baseUrl: (fixture: ReturnType<typeof createProtocolFixture>) =>
        `${fixture.baseUrl}/v1beta`,
    },
  ];
  const raw413Recognized = hasTrailingToolCompaction(
    await installedPiCodingAgentVersion(),
  );

  for (const provider of providers) {
    await withFixture(
      'raw-provider-413',
      `unmodified-413-${provider.provider}`,
      async (fixture) => {
        const byoPath = path.join(
          testDataRoot,
          `byo-unmodified-413-${provider.provider}.json`,
        );
        byoKey.setByoKeyFilePathForTests(byoPath);
        await byoKey.persistByoKey({
          providerId: provider.provider,
          apiKey: 'byo-fixture-token',
          provider: provider.provider,
          api: provider.api,
          baseUrl: provider.baseUrl(fixture),
          modelId:
            provider.provider === 'anthropic'
              ? 'claude-fixture'
              : provider.provider === 'google'
                ? 'gemini-fixture'
                : 'fixture-model',
          modelName: 'Fixture Model',
        });
        const events: Array<{ type: string }> = [];
        const ai = createAi(fixture, []);
        let response: unknown;
        let error: unknown;
        try {
          try {
            response = await ai
              .session(`unmodified-413-${provider.provider}`, {
                model: 'fast',
                toolMode: 'none',
              })
              .ask(
                'Recover this unmodified provider 413 when supported by Pi.',
                {
                  onEvent: (event) => events.push(event),
                },
              );
          } catch (caught) {
            error = caught;
          }

          if (raw413Recognized) {
            assert.equal(error, undefined);
            assert.match(String(response), /recovered successfully/);
            assert.equal(fixture.requests.length, 3);
            assert.deepEqual(
              terminalEvents(events).map((event) => event.type),
              ['done'],
            );
          } else {
            assert.ok(error instanceof Error);
            assert.equal(fixture.requests.length, 1);
            assert.deepEqual(
              terminalEvents(events).map((event) => event.type),
              ['error'],
            );
          }
          fixtureEvidence.push({
            scenario: `provider-raw-413-${provider.api}`,
            finalOutcome: raw413Recognized ? 'done' : 'error',
            providerCalls: fixture.requests.length,
            raw413Recovered: raw413Recognized,
          });
        } finally {
          await ai.disposeAllSessions();
        }
      },
      provider.api,
    );
  }
});

test('one-shot AI keeps using the direct streaming path', async () => {
  await withFixture('complete', 'one-shot', async (fixture) => {
    const events: Array<{ type: string; label?: string }> = [];
    const ai = createAi(fixture, events);
    const response = await ai.ask('Answer with a short fixture response.', {
      model: 'fast',
      onEvent: (event) => events.push(event),
    });
    assert.match(String(response), /fixture request completed/);
    assert.equal(fixture.requests.length, 1);
    assert.deepEqual(
      terminalEvents(events).map((event) => event.type),
      ['done'],
    );
    assert.equal(
      events.some((event) => event.label === 'pi-session-budget'),
      false,
      'tool-free one-shot requests should keep the direct stream path',
    );
    fixtureEvidence.push({
      scenario: 'tool-free-one-shot-direct-stream',
      finalOutcome: 'done',
      providerCalls: fixture.requests.length,
    });
  });
});

test('prepared tool-enabled fallback reuses the real loaded session', async () => {
  await withFixture('complete', 'prepared-fallback', async (fixture) => {
    const ai = createAi(fixture, []);
    const events: Array<{ type: string }> = [];
    try {
      await ai.prepare({ model: 'fast', toolMode: 'conversation' });
      const response = await ai.ask(
        'Use the prepared read-only conversation session.',
        {
          model: 'fast',
          toolMode: 'conversation',
          onEvent: (event) => events.push(event),
        },
      );
      assert.match(String(response), /fixture request completed/);
      assert.equal(fixture.requests.length, 1);
      assert.deepEqual(
        terminalEvents(events).map((event) => event.type),
        ['done'],
      );
      assert.deepEqual(
        fixture.requests[0].body.tools
          .map((tool: { function?: { name?: string } }) => tool.function?.name)
          .sort(),
        ['fetch_content', 'find', 'grep', 'ls', 'read', 'web_search'],
      );
      fixtureEvidence.push({
        scenario: 'prepared-fallback-session-reused',
        finalOutcome: 'done',
        providerCalls: fixture.requests.length,
      });
    } finally {
      await ai.disposeAllSessions();
    }
  });
});

test('overlapping prompts on one extension session settle in request order', async () => {
  await withFixture('queue-conversations', 'queued-asks', async (fixture) => {
    const ai = createAi(fixture, []);
    try {
      const session = ai.session('queued-extension-session', {
        model: 'fast',
        toolMode: 'none',
      });
      const first = session.ask('First queued extension request.');
      await fixture.waitForRequests(1);
      const second = session.ask('Second queued extension request.');
      const [firstResult, secondResult] = await Promise.all([first, second]);

      assert.match(String(firstResult), /Queued request 1 completed/);
      assert.match(String(secondResult), /Queued request 2 completed/);
      assert.match(
        JSON.stringify(fixture.requests[0].body.messages),
        /First queued extension request/,
      );
      assert.match(
        JSON.stringify(fixture.requests[1].body.messages),
        /Second queued extension request/,
      );
      fixtureEvidence.push({
        scenario: 'general-session-prompts-serialize-outcomes',
        finalOutcome: 'done',
        providerCalls: fixture.requests.length,
      });
    } finally {
      await ai.disposeAllSessions();
    }
  });
});

test('Pi summary auth and credit failures produce one terminal error, never success', async () => {
  for (const scenario of ['summary-unauthorized', 'summary-credits'] as const) {
    await withFixture(scenario, scenario, async (fixture) => {
      const ai = createAi(fixture, []);
      const events: Array<{ type: string; message?: string }> = [];
      try {
        await assert.rejects(
          ai
            .session(`summary-failure-${scenario}`, {
              model: 'fast',
              toolMode: 'none',
            })
            .ask(
              'Recover this request after Pi summarizes the rejected input.',
              {
                onEvent: (event) => events.push(event),
              },
            ),
        );
        assert.equal(fixture.requests.length, 2);
        assert.deepEqual(
          terminalEvents(events).map((event) => event.type),
          ['error'],
        );
        fixtureEvidence.push({
          scenario: `summary-failure-${scenario}`,
          finalOutcome: 'error',
          providerCalls: fixture.requests.length,
          noFalseSuccess: true,
        });
      } finally {
        await ai.disposeAllSessions();
      }
    });
  }
});

test('Pi stops after an exhausted overflow recovery attempt and emits one final error', async () => {
  await withFixture(
    'recovery-exhausted',
    'recovery-exhausted',
    async (fixture) => {
      const ai = createAi(fixture, []);
      const events: Array<{ type: string }> = [];
      try {
        await assert.rejects(
          ai
            .session('recovery-exhausted-session', {
              model: 'fast',
              toolMode: 'none',
            })
            .ask('Return an error if Pi cannot recover the rejected request.', {
              onEvent: (event) => events.push(event),
            }),
        );
        assert.ok(fixture.requests.length >= 3);
        assert.ok(fixture.requests.length <= 5);
        assert.deepEqual(
          terminalEvents(events).map((event) => event.type),
          ['error'],
        );
        fixtureEvidence.push({
          scenario: 'overflow-recovery-exhausted',
          finalOutcome: 'error',
          providerCalls: fixture.requests.length,
          noFalseSuccess: true,
        });
      } finally {
        await ai.disposeAllSessions();
      }
    },
  );
});

test('host cancellation during Pi compaction settles as aborted without false success', async () => {
  await withFixture('summary-hangs', 'summary-cancel', async (fixture) => {
    const ai = createAi(fixture, []);
    const events: Array<{ type: string }> = [];
    const controller = new AbortController();
    try {
      const pending = ai
        .session('summary-cancel-session', {
          model: 'fast',
          toolMode: 'none',
        })
        .ask('Cancel while Pi is preparing a context summary.', {
          signal: controller.signal,
          onEvent: (event) => events.push(event),
        });
      let summaryTimeout: ReturnType<typeof setTimeout> | undefined;
      try {
        await Promise.race([
          fixture.waitForRequests(2),
          new Promise<never>((_resolve, reject) => {
            summaryTimeout = setTimeout(
              () => reject(new Error('Pi did not start its summary')),
              15_000,
            );
          }),
        ]);
      } finally {
        if (summaryTimeout) clearTimeout(summaryTimeout);
      }
      controller.abort();
      await assert.rejects(pending, /aborted/i);
      assert.deepEqual(
        terminalEvents(events).map((event) => event.type),
        ['aborted'],
      );
      fixtureEvidence.push({
        scenario: 'host-cancellation-during-compaction',
        finalOutcome: 'aborted',
        providerCalls: fixture.requests.length,
        noFalseSuccess: true,
      });
    } finally {
      controller.abort();
      await ai.disposeAllSessions();
    }
  });
});

test('host cancellation during Pi transient retry emits only an aborted outcome', async () => {
  await withFixture(
    'builder-retry-hangs',
    'builder-retry-cancel',
    async (fixture) => {
      const events: Array<{ type: string }> = [];
      const ai = createAi(fixture, events);
      try {
        const pending = ai.send(
          'Cancel after Pi starts a transient retry.',
          'builder-retry-cancel-chat',
        );
        await fixture.waitForRequests(2);
        await ai.abort('builder-retry-cancel-chat');
        await pending;
        assert.deepEqual(
          terminalEvents(events).map((event) => event.type),
          ['aborted'],
        );
        fixtureEvidence.push({
          scenario: 'host-cancellation-during-transient-retry',
          finalOutcome: 'aborted',
          providerCalls: fixture.requests.length,
          noFalseSuccess: true,
        });
      } finally {
        await ai.disposeAllSessions();
      }
    },
  );
});

test('builder transient retries stay enabled while general session retries stay disabled', async () => {
  await withFixture(
    'builder-transient',
    'builder-transient',
    async (fixture) => {
      const events: Array<{ type: string }> = [];
      const ai = createAi(fixture, events);
      try {
        await ai.send(
          'Recover one temporary provider failure.',
          'transient-chat',
        );
        assert.equal(fixture.requests.length, 3);
        assert.deepEqual(
          terminalEvents(events).map((event) => event.type),
          ['done'],
        );
        fixtureEvidence.push({
          scenario: 'builder-transient-retry-budget-two',
          finalOutcome: 'done',
          providerCalls: fixture.requests.length,
        });
      } finally {
        await ai.disposeAllSessions();
      }
    },
  );

  await withFixture(
    'general-transient',
    'general-transient',
    async (fixture) => {
      const ai = createAi(fixture, []);
      const events: Array<{ type: string }> = [];
      try {
        await assert.rejects(
          ai
            .session('general-transient-session', { model: 'fast' })
            .ask('Do not retry this temporary provider failure.', {
              onEvent: (event) => events.push(event),
            }),
        );
        assert.equal(fixture.requests.length, 1);
        assert.deepEqual(
          terminalEvents(events).map((event) => event.type),
          ['error'],
        );
        fixtureEvidence.push({
          scenario: 'general-transient-retries-disabled',
          finalOutcome: 'error',
          providerCalls: fixture.requests.length,
        });
      } finally {
        await ai.disposeAllSessions();
      }
    },
  );
});

test('conversation AI exposes the exact real read-only and web tool allowlist', async () => {
  await withFixture('complete', 'conversation', async (fixture) => {
    const ai = createAi(fixture, []);
    const events: Array<{ type: string }> = [];
    try {
      await ai
        .session('conversation-tool-session', {
          model: 'fast',
          toolMode: 'conversation',
        })
        .ask('Summarize the available conversation tools.', {
          onEvent: (event) => events.push(event),
        });
      assert.deepEqual(
        terminalEvents(events).map((event) => event.type),
        ['done'],
      );
      const names = fixture.requests[0].body.tools.map(
        (tool: { function?: { name?: string } }) => tool.function?.name,
      );
      assert.deepEqual(names.sort(), [
        'fetch_content',
        'find',
        'grep',
        'ls',
        'read',
        'web_search',
      ]);
      fixtureEvidence.push({
        scenario: 'conversation-explicit-read-only-tool-allowlist',
        finalOutcome: 'done',
        providerCalls: fixture.requests.length,
        toolAllowlist: names,
      });
      for (const forbidden of [
        'bash',
        'edit',
        'write',
        'source_check',
        'get_search_content',
        'tool_search',
        'codemode',
        'mcp',
      ])
        assert.equal(names.includes(forbidden), false);
    } finally {
      await ai.disposeAllSessions();
    }
  });
});
