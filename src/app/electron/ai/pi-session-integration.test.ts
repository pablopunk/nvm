import assert from 'node:assert/strict';
import {
  createServer,
  type IncomingMessage,
  type ServerResponse,
} from 'node:http';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
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
const piCodingAgentVersion = await installedPiCodingAgentVersion();

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
const FIXTURE_REQUEST_TIMEOUT_MS = 10_000;
const FIXTURE_CLOSE_TIMEOUT_MS = 5_000;
const LONG_SESSION_TURN =
  'Deterministic session history retained by the Pi SDK fixture. '.repeat(110);
const LONG_SESSION_TURN_COUNT = 16;
const BUILDER_HISTORY_TURN_COUNT = 8;
const ZERO_USAGE = { prompt_tokens: 0, completion_tokens: 0, total_tokens: 0 };

type FixtureAiEvent = {
  type: string;
  name?: string;
  label?: string;
  data?: unknown;
  text?: string;
  message?: string;
  isError?: boolean;
};

type PiToolPolicy = {
  registeredTools: Array<{ name: string; exposure: string }>;
  activeToolNames: string[];
  callableToolNames: string[];
};

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function eventData(event: FixtureAiEvent) {
  return isRecord(event.data) ? event.data : undefined;
}

function eventDataValue(event: FixtureAiEvent, key: string) {
  return eventData(event)?.[key];
}

function toolTraceOutput(event: FixtureAiEvent) {
  const output = eventDataValue(event, 'output');
  return isRecord(output) ? output : undefined;
}

function toolTraceInput(event: FixtureAiEvent) {
  const input = eventDataValue(event, 'input');
  return isRecord(input) ? input : undefined;
}

function toolTraceDetails(event: FixtureAiEvent) {
  const details = toolTraceOutput(event)?.details;
  if (typeof details !== 'string') return {};
  try {
    const parsed: unknown = JSON.parse(details);
    return isRecord(parsed) ? parsed : {};
  } catch {
    return {};
  }
}

function isPiToolPolicy(value: unknown): value is PiToolPolicy {
  if (!isRecord(value)) return false;
  return (
    Array.isArray(value.registeredTools) &&
    value.registeredTools.every(
      (tool: unknown) =>
        isRecord(tool) &&
        typeof tool.name === 'string' &&
        typeof tool.exposure === 'string',
    ) &&
    Array.isArray(value.activeToolNames) &&
    value.activeToolNames.every((name: unknown) => typeof name === 'string') &&
    Array.isArray(value.callableToolNames) &&
    value.callableToolNames.every((name: unknown) => typeof name === 'string')
  );
}

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
    await fs.mkdir(evidenceDirectory, { recursive: true });
    await fs.writeFile(
      path.join(evidenceDirectory, 'integration-evidence.json'),
      `${JSON.stringify(
        {
          sdkVersion: piCodingAgentVersion,
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
  method?: string;
  path: string;
  phase: string;
  responseStatus?: number;
};

function hasByoRequestHeaders(request: CapturedRequest) {
  return (
    request.headers.authorization === 'Bearer byo-fixture-token' &&
    request.headers['x-nevermind-client'] === 'desktop' &&
    request.headers['x-nevermind-client-version'] === 'test' &&
    request.headers['x-nevermind-api-version'] === '1' &&
    !request.headers['x-nevermind-ai-model-tier'] &&
    !request.headers['x-nevermind-ai-credit-kind']
  );
}

type FixtureScenario =
  | 'builder'
  | 'builder-long-session'
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
  | 'prepare-cancel'
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

function sendStream(
  response: ServerResponse,
  value: object,
  usage = {
    prompt_tokens: 1_280,
    completion_tokens: 18,
    total_tokens: 1_298,
  },
) {
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
      usage,
    })}\n\ndata: [DONE]\n\n`,
  );
}

function sendText(
  response: ServerResponse,
  text: string,
  usage?: {
    prompt_tokens: number;
    completion_tokens: number;
    total_tokens: number;
  },
) {
  sendStream(
    response,
    {
      delta: { content: text },
      finish_reason: 'stop',
    },
    usage,
  );
}

function sendAnthropicText(
  response: ServerResponse,
  text: string,
  usage?: typeof ZERO_USAGE,
) {
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
          usage: { input_tokens: usage?.prompt_tokens ?? 12, output_tokens: 0 },
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
        usage: { output_tokens: usage?.completion_tokens ?? 12 },
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

function sendGoogleText(
  response: ServerResponse,
  text: string,
  usage?: typeof ZERO_USAGE,
) {
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
        promptTokenCount: usage?.prompt_tokens ?? 12,
        candidatesTokenCount: usage?.completion_tokens ?? 12,
        totalTokenCount: usage?.total_tokens ?? 24,
      },
    })}\n\n`,
  );
}

function sendTextForApi(
  response: ServerResponse,
  api: string,
  text: string,
  usage?: typeof ZERO_USAGE,
) {
  if (api === 'anthropic-messages')
    return sendAnthropicText(response, text, usage);
  if (api === 'google-generative-ai')
    return sendGoogleText(response, text, usage);
  return sendText(response, text, usage);
}

function sendBuilderToolCall(
  response: ServerResponse,
  params = '{}',
  usage?: typeof ZERO_USAGE,
) {
  sendToolCall(response, 'read_extension_api', params, usage);
}

function sendToolCall(
  response: ServerResponse,
  name: string,
  params = '{}',
  usage?: typeof ZERO_USAGE,
) {
  sendStream(
    response,
    {
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
    },
    usage,
  );
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

function providerApiForRequestPath(
  requestUrl: string | undefined,
  baseUrl: string,
) {
  const request = new URL(requestUrl || '/', baseUrl);
  if (
    request.pathname === '/v1/chat/completions' &&
    request.search.length === 0
  )
    return 'openai-completions';
  if (
    request.pathname === '/v1/messages' &&
    (request.search.length === 0 || request.search === '?beta=true')
  )
    return 'anthropic-messages';
  if (
    /^\/v1beta\/models\/[^/]+:streamGenerateContent$/.test(request.pathname) &&
    [...request.searchParams.keys()].length === 1 &&
    request.searchParams.get('alt') === 'sse'
  )
    return 'google-generative-ai';
  return undefined;
}

function hasBuilderTools(body: Record<string, any>) {
  return Array.isArray(body.tools) && body.tools.length > 0;
}

async function installedPiCodingAgentVersion() {
  const modulePath = fileURLToPath(
    import.meta.resolve('@earendil-works/pi-coding-agent'),
  );
  const packagePath = path.resolve(
    path.dirname(modulePath),
    '..',
    'package.json',
  );
  const packageJson: unknown = JSON.parse(
    await fs.readFile(packagePath, 'utf8'),
  );
  if (!isRecord(packageJson) || typeof packageJson.version !== 'string')
    throw new Error('The installed Pi coding agent package has no version.');
  return packageJson.version;
}

function containsApiReadResult(body: Record<string, any>) {
  return JSON.stringify(body.messages || []).includes('[Read page:');
}

function providerMessageText(value: unknown): string {
  if (typeof value === 'string') return value;
  if (Array.isArray(value)) return value.map(providerMessageText).join('\n');
  if (!isRecord(value)) return '';
  if (typeof value.text === 'string') return value.text;
  return [value.content, value.parts].map(providerMessageText).join('\n');
}

function latestProviderUserText(body: Record<string, any>) {
  const messages = Array.isArray(body.messages)
    ? body.messages
    : Array.isArray(body.contents)
      ? body.contents
      : [];
  const userMessages = messages.filter(
    (message: unknown) =>
      isRecord(message) &&
      ['user', 'human'].includes(String(message.role).toLowerCase()),
  );
  const latest = userMessages.at(-1);
  return isRecord(latest)
    ? providerMessageText(latest.content ?? latest.parts)
    : '';
}

function compactionSummaryPhase(body: Record<string, any>) {
  const serialized = JSON.stringify(body.messages || body.contents || body);
  if (
    serialized.includes('The messages above are a conversation to summarize.')
  )
    return 'history';
  if (
    serialized.includes(
      'The messages above are earlier context from an ongoing conversation.',
    )
  )
    return 'turn-prefix';
  return undefined;
}

function isLongSessionSeed(text: string) {
  return /^Earlier fixture session turn \d+\.$/.test(text.trim());
}

function isLongSessionTask(text: string) {
  return [
    'Continue this extension conversation after context recovery.',
    'Recover this provider request after Pi compacts.',
    'Recover this unmodified provider 413 when supported by Pi.',
    'Recover this request after Pi summarizes the rejected input.',
    'Return an error if Pi cannot recover the rejected request.',
    'Cancel while Pi is preparing a context summary.',
  ].includes(text.trim().split('\n')[0]);
}

function assertLongSessionPhaseTrace(
  requests: readonly CapturedRequest[],
  outcome:
    | 'recovered'
    | 'summary-error'
    | 'summary-pending'
    | 'recovery-exhausted'
    | 'raw-413',
) {
  const phases = requests.map((request) => request.phase);
  const seedRequests = requests.filter(
    (request) => request.phase === 'session-seed',
  );
  const taskRequests = requests.filter(
    (request) => request.phase === 'session-task',
  );
  const summaryRequests = requests.filter((request) =>
    request.phase.startsWith('session-summary-'),
  );
  const recoveryRequests = requests.filter(
    (request) => request.phase === 'session-recovery',
  );

  assert.equal(seedRequests.length, LONG_SESSION_TURN_COUNT);
  assert.equal(
    taskRequests.length,
    1,
    'one semantic task request must be rejected',
  );
  assert.equal(taskRequests[0].responseStatus, 413);
  assert.ok(
    isLongSessionTask(latestProviderUserText(taskRequests[0].body)),
    'the actual user request must be the one rejected by the fixture backend',
  );
  assert.ok(
    seedRequests.every(
      (request, index) =>
        isLongSessionSeed(latestProviderUserText(request.body)) &&
        requests.indexOf(request) === index,
    ),
    'the fixture must seed the requested history turns in order',
  );

  const taskIndex = requests.indexOf(taskRequests[0]);
  const summaryIndices = summaryRequests.map((request) =>
    requests.indexOf(request),
  );
  assert.ok(
    summaryIndices.every((index) => index > taskIndex),
    'Pi must start compaction only after the task request is rejected',
  );
  const summaryKinds = summaryRequests.map((request) =>
    request.phase.replace('session-summary-', ''),
  );
  assert.ok(summaryKinds.length <= 2, 'Pi compaction must remain bounded');
  assert.ok(
    summaryKinds.length === 0 ||
      (summaryKinds.length === 1 &&
        ['history', 'turn-prefix'].includes(summaryKinds[0])) ||
      (summaryKinds.length === 2 &&
        summaryKinds[0] === 'history' &&
        summaryKinds[1] === 'turn-prefix'),
    `Pi summary phases should follow its history-then-split-turn order: ${summaryKinds.join(', ')}`,
  );

  if (outcome === 'raw-413') {
    assert.equal(summaryRequests.length, 0);
    assert.equal(recoveryRequests.length, 0);
  } else if (outcome === 'summary-error' || outcome === 'summary-pending') {
    assert.equal(summaryRequests.length, 1);
    assert.equal(recoveryRequests.length, 0);
    if (outcome === 'summary-error')
      assert.ok([401, 402].includes(summaryRequests[0].responseStatus || 0));
    else assert.equal(summaryRequests[0].responseStatus, undefined);
  } else if (outcome === 'recovery-exhausted') {
    assert.ok(summaryRequests.length >= 1);
    assert.equal(recoveryRequests.length, 1);
    assert.ok(
      isLongSessionTask(latestProviderUserText(recoveryRequests[0].body)),
      'Pi must retry the rejected task after compaction',
    );
    assert.ok(
      summaryIndices.every(
        (index) => index < requests.indexOf(recoveryRequests[0]),
      ),
      'Pi must finish every summary call before retrying the rejected task',
    );
    assert.equal(recoveryRequests[0].responseStatus, 413);
  } else {
    assert.ok(summaryRequests.length >= 1);
    assert.equal(recoveryRequests.length, 1);
    assert.ok(
      isLongSessionTask(latestProviderUserText(recoveryRequests[0].body)),
      'Pi must retry the rejected task after compaction',
    );
    assert.ok(
      summaryIndices.every(
        (index) => index < requests.indexOf(recoveryRequests[0]),
      ),
      'Pi must finish every summary call before retrying the rejected task',
    );
    assert.notEqual(recoveryRequests[0].responseStatus, 413);
  }

  assert.ok(
    requests.every((request) => request.phase !== 'unclassified'),
    `every provider call must have a semantic fixture phase: ${phases.join(', ')}`,
  );
  assert.ok(
    requests.length <=
      seedRequests.length +
        taskRequests.length +
        summaryRequests.length +
        recoveryRequests.length,
    'the provider trace must contain only seeded turns, task, compaction, and recovery calls',
  );
  return { phases, summaryKinds };
}

function createProtocolFixture(
  scenario: FixtureScenario = 'builder',
  api = 'openai-completions',
) {
  const requests: CapturedRequest[] = [];
  const unexpectedRoutes: Array<{
    method: string;
    path: string;
    expectedApi: string;
    detectedApi: string | null;
  }> = [];
  const managedDescriptors: Array<{ provider: string; api: string }> = [];
  const requestWaiters: Array<{
    count: number;
    resolve: () => void;
  }> = [];
  const phaseWaiters: Array<{
    phase: string;
    resolve: () => void;
    reject: (error: Error) => void;
    timeout: ReturnType<typeof setTimeout>;
  }> = [];
  let rejectedLargeResult = false;
  let requestedContinuation = false;
  let builderTaskStarted = false;
  let summaryServed = false;
  let sessionTaskRejected = false;
  let fixtureClosed = false;
  let activeModelRequestCount = 0;
  let notifyModelRefreshStarted!: () => void;
  let releaseModelRefresh!: () => void;
  const modelRefreshStarted = new Promise<void>((resolve) => {
    notifyModelRefreshStarted = resolve;
  });
  const modelRefreshGate = new Promise<void>((resolve) => {
    releaseModelRefresh = resolve;
  });
  function setRequestPhase(request: CapturedRequest, phase: string) {
    request.phase = phase;
    for (let index = phaseWaiters.length - 1; index >= 0; index -= 1) {
      const waiter = phaseWaiters[index];
      if (waiter.phase !== phase) continue;
      clearTimeout(waiter.timeout);
      phaseWaiters.splice(index, 1);
      waiter.resolve();
    }
  }
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
      activeModelRequestCount += 1;
      if (scenario === 'prepare-cancel' && activeModelRequestCount === 2) {
        notifyModelRefreshStarted();
        await modelRefreshGate;
      }
      const contextWindow =
        scenario === 'builder-long-session'
          ? 40_000
          : scenario === 'backend-estimator-boundary'
            ? MAX_INPUT_TOKENS + PI_AI_COMPACTION_RESERVE_TOKENS + 20_000
            : [
                  'session-overflow',
                  'summary-unauthorized',
                  'summary-credits',
                  'summary-hangs',
                  'recovery-exhausted',
                  'raw-provider-413',
                ].includes(scenario)
              ? 200_000
              : 24_576;
      const descriptor = {
        id: 'fixture-model',
        name: 'Fixture Model',
        provider: 'nevermind',
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
      };
      managedDescriptors.push(descriptor);
      sendJson(response, 200, descriptor);
      return;
    }
    const requestUrl = new URL(request.url || '/', fixture.baseUrl);
    const requestPath = `${requestUrl.pathname}${requestUrl.search}`;
    const requestApi = providerApiForRequestPath(request.url, fixture.baseUrl);
    if (request.method !== 'POST' || requestApi !== api) {
      const route = {
        method: request.method || 'UNKNOWN',
        path: requestPath,
        expectedApi: api,
        detectedApi: requestApi || null,
      };
      unexpectedRoutes.push(route);
      console.error(`[Pi fixture route mismatch] ${JSON.stringify(route)}`);
      sendJson(response, 404, {
        error: `unknown fixture route: ${request.method} ${requestPath} for ${api}`,
      });
      return;
    }

    let body: Record<string, any>;
    try {
      body = JSON.parse(bodyText);
    } catch {
      sendJson(response, 400, { error: 'invalid request JSON' });
      return;
    }
    requests.push({
      headers: request.headers,
      body,
      method: request.method,
      path: requestPath,
      phase: 'unclassified',
    });
    const callIndex = requests.length;
    for (const waiter of requestWaiters.splice(0)) {
      if (requests.length >= waiter.count) waiter.resolve();
      else requestWaiters.push(waiter);
    }

    if (scenario === 'builder-long-session') {
      const currentRequest = requests.at(-1)!;
      const latestUserText = latestProviderUserText(body);
      const summaryPhase = compactionSummaryPhase(body);

      if (summaryPhase) {
        setRequestPhase(
          currentRequest,
          `builder-proactive-summary-${summaryPhase}`,
        );
        sendText(response, 'The builder conversation context is summarized.');
        return;
      }

      if (latestUserText.includes('PI_BUILDER_HISTORY_SEED ')) {
        setRequestPhase(currentRequest, 'builder-seed');
        sendTextForApi(response, api, LONG_SESSION_TURN, ZERO_USAGE);
        return;
      }

      if (latestUserText.includes('Make one more small change')) {
        setRequestPhase(currentRequest, 'builder-follow-up');
        sendText(response, 'The follow-up change is complete.');
        return;
      }

      if (
        builderTaskStarted ||
        latestUserText.includes('The user confirmed the extension request.')
      ) {
        if (!builderTaskStarted) {
          builderTaskStarted = true;
          setRequestPhase(currentRequest, 'builder-task-start');
          sendBuilderToolCall(response, '{}', ZERO_USAGE);
        } else if (!requestedContinuation) {
          requestedContinuation = true;
          setRequestPhase(currentRequest, 'builder-read-continuation');
          const nextOffset = fixture.nextOffset;
          if (typeof nextOffset !== 'number') {
            sendJson(response, 500, {
              error: 'read page did not offer continuation',
            });
            return;
          }
          sendBuilderToolCall(
            response,
            JSON.stringify({ offset: nextOffset }),
            ZERO_USAGE,
          );
        } else {
          setRequestPhase(currentRequest, 'builder-task-complete');
          sendText(response, 'The requested extension operation is complete.');
        }
        return;
      }

      sendJson(response, 500, {
        error: `unrecognized builder fixture turn: ${latestUserText.slice(0, 120)}`,
      });
      return;
    }

    if (
      scenario === 'session-overflow' ||
      scenario === 'summary-unauthorized' ||
      scenario === 'summary-credits' ||
      scenario === 'summary-hangs' ||
      scenario === 'recovery-exhausted' ||
      scenario === 'raw-provider-413'
    ) {
      const currentRequest = requests.at(-1)!;
      const latestUserText = latestProviderUserText(body);
      const summaryPhase = compactionSummaryPhase(body);

      if (isLongSessionSeed(latestUserText)) {
        setRequestPhase(currentRequest, 'session-seed');
        sendTextForApi(response, api, LONG_SESSION_TURN, ZERO_USAGE);
        return;
      }
      if (summaryPhase) {
        setRequestPhase(currentRequest, `session-summary-${summaryPhase}`);
        if (scenario === 'summary-unauthorized') {
          currentRequest.responseStatus = 401;
          sendJson(response, 401, {
            error: { type: 'invalid_api_key', message: 'Invalid API key' },
          });
        } else if (scenario === 'summary-credits') {
          currentRequest.responseStatus = 402;
          sendJson(response, 402, {
            error: {
              type: 'insufficient_credits',
              message: 'No credits remaining',
            },
          });
        } else if (scenario === 'summary-hangs') {
          return;
        } else {
          sendTextForApi(
            response,
            api,
            'The extension session context is summarized.',
          );
        }
        return;
      }

      if (!isLongSessionTask(latestUserText)) {
        setRequestPhase(currentRequest, 'session-unexpected');
        sendJson(response, 500, {
          error: `unrecognized session fixture turn: ${latestUserText.slice(0, 120)}`,
        });
        return;
      }

      if (!sessionTaskRejected) {
        sessionTaskRejected = true;
        setRequestPhase(currentRequest, 'session-task');
        currentRequest.responseStatus = 413;
        sendPromptTooLargeForApi(
          response,
          api,
          scenario !== 'raw-provider-413',
        );
        return;
      }

      setRequestPhase(currentRequest, 'session-recovery');
      if (scenario === 'recovery-exhausted') {
        currentRequest.responseStatus = 413;
        sendPromptTooLargeForApi(response, api);
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

    if (scenario === 'complete' || scenario === 'prepare-cancel') {
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
    unexpectedRoutes,
    managedDescriptors,
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
      fixtureClosed = true;
      notifyModelRefreshStarted();
      releaseModelRefresh();
      for (const waiter of requestWaiters.splice(0)) waiter.resolve();
      for (const waiter of phaseWaiters.splice(0)) {
        clearTimeout(waiter.timeout);
        waiter.reject(
          new Error('Fixture closed before expected request phase.'),
        );
      }
      server.closeAllConnections();
      let timeout: ReturnType<typeof setTimeout> | undefined;
      try {
        await Promise.race([
          new Promise<void>((resolve, reject) =>
            server.close((error) => (error ? reject(error) : resolve())),
          ),
          new Promise<never>((_resolve, reject) => {
            timeout = setTimeout(
              () =>
                reject(
                  new Error(
                    `Fixture server did not close within ${FIXTURE_CLOSE_TIMEOUT_MS}ms.`,
                  ),
                ),
              FIXTURE_CLOSE_TIMEOUT_MS,
            );
          }),
        ]);
      } finally {
        if (timeout) clearTimeout(timeout);
      }
    },
    waitForRequests(count: number) {
      if (requests.length >= count) return Promise.resolve();
      if (fixtureClosed)
        return Promise.reject(
          new Error('Fixture closed before provider request.'),
        );
      return new Promise<void>((resolve, reject) => {
        let timeout: ReturnType<typeof setTimeout> | undefined;
        const waiter = {
          count,
          resolve() {
            if (timeout) clearTimeout(timeout);
            resolve();
          },
        };
        timeout = setTimeout(() => {
          const waiterIndex = requestWaiters.indexOf(waiter);
          if (waiterIndex >= 0) requestWaiters.splice(waiterIndex, 1);
          reject(
            new Error(
              `Timed out waiting for provider request ${count}; received ${requests.length}.`,
            ),
          );
        }, FIXTURE_REQUEST_TIMEOUT_MS);
        requestWaiters.push(waiter);
      });
    },
    waitForPhase(phase: string) {
      if (requests.some((request) => request.phase === phase))
        return Promise.resolve();
      if (fixtureClosed)
        return Promise.reject(
          new Error(`Fixture closed before provider phase ${phase}.`),
        );
      return new Promise<void>((resolve, reject) => {
        const waiter = {
          phase,
          resolve,
          reject,
          timeout: setTimeout(() => {
            const waiterIndex = phaseWaiters.indexOf(waiter);
            if (waiterIndex >= 0) phaseWaiters.splice(waiterIndex, 1);
            reject(
              new Error(
                `Timed out waiting for provider phase ${phase}; observed ${requests.map((request) => request.phase).join(', ')}.`,
              ),
            );
          }, FIXTURE_REQUEST_TIMEOUT_MS),
        };
        phaseWaiters.push(waiter);
      });
    },
    async waitForModelRefresh() {
      let timeout: ReturnType<typeof setTimeout> | undefined;
      try {
        await Promise.race([
          modelRefreshStarted,
          new Promise<never>((_resolve, reject) => {
            timeout = setTimeout(
              () => reject(new Error('Model refresh did not start')),
              15_000,
            );
          }),
        ]);
      } finally {
        if (timeout) clearTimeout(timeout);
      }
    },
    releaseModelRefresh() {
      releaseModelRefresh();
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
  events: FixtureAiEvent[],
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
        const nextOffset = toolTraceDetails(event).nextOffset;
        if (typeof nextOffset === 'number') fixture.nextOffset = nextOffset;
      }
    },
  });
}

function terminalEvents(events: readonly FixtureAiEvent[]) {
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

async function seedLongPiSession(session: {
  ask: (message: string) => Promise<unknown>;
}) {
  for (let turn = 0; turn < LONG_SESSION_TURN_COUNT; turn += 1)
    await session.ask(`Earlier fixture session turn ${turn + 1}.`);
}

test('the real builder adapter proactively compacts bounded API reads and accepts a follow-up', async (context) => {
  await withFixture('builder-long-session', 'proactive', async (fixture) => {
    const events: FixtureAiEvent[] = [];
    const ai = createAi(fixture, events);
    try {
      let toolPolicy: unknown;
      for (let turn = 0; turn < BUILDER_HISTORY_TURN_COUNT; turn += 1) {
        events.length = 0;
        await ai.send(
          `PI_BUILDER_HISTORY_SEED ${turn + 1}`,
          'large-builder-chat',
        );
        assert.deepEqual(
          terminalEvents(events).map((event) => event.type),
          ['done'],
          'each earlier builder turn should finish through the real adapter',
        );
        if (turn === 0) {
          const policyEvent = events.find(
            (event) =>
              event.type === 'debug' && event.label === 'pi-tool-policy',
          );
          toolPolicy = policyEvent ? eventData(policyEvent) : undefined;
        }
      }
      events.length = 0;
      await ai.send(
        'The user confirmed the extension request. Read the API, then summarize the requested change.',
        'large-builder-chat',
      );
      assert.ok(fixture.managedDescriptors.length > 0);
      assert.ok(
        fixture.managedDescriptors.every(
          (descriptor) =>
            descriptor.provider === 'nevermind' &&
            descriptor.api === 'openai-completions',
        ),
        'managed identity must stay Nevermind while API format identifies OpenAI compatibility',
      );
      assert.deepEqual(
        terminalEvents(events).map((event) => event.type),
        ['done'],
      );
      const apiReadEvents = events.filter(
        (event) =>
          event.type === 'tool_trace_end' &&
          event.name === 'read_extension_api',
      );
      assert.equal(apiReadEvents.length, 2);
      const pageInputs = apiReadEvents.map(toolTraceInput);
      const pageDetails = apiReadEvents.map(toolTraceDetails);
      const firstNextOffset = Number(pageDetails[0].nextOffset);
      const apiSource = await fs.readFile(
        path.join(repoRoot, 'src/app/resources/nevermind-extension-api.d.ts'),
        'utf8',
      );
      const apiLines = apiSource.split('\n');
      if (apiLines.at(-1) === '') apiLines.pop();
      assert.ok(
        firstNextOffset > 0,
        'the first API page must offer continuation',
      );
      assert.equal(pageDetails[0].totalLines, apiLines.length);
      assert.ok(firstNextOffset < apiLines.length);
      assert.equal(Number(pageInputs[1]?.offset), firstNextOffset);
      assert.equal(pageDetails[1].nextOffset, null);
      assert.equal(pageDetails[1].complete, true);
      assert.equal(pageDetails[1].totalLines, pageDetails[0].totalLines);
      assert.equal(pageDetails[1].revision, pageDetails[0].revision);
      const continuationLine = apiLines
        .slice(firstNextOffset)
        .find((line) => line.trim().length > 0);
      assert.ok(continuationLine);
      assert.ok(
        String(toolTraceOutput(apiReadEvents[1])?.text || '').includes(
          continuationLine,
        ),
        'the continued tool output must include source content after the returned offset',
      );
      const taskPromptIndex = fixture.requests.findIndex(
        (request) => request.phase === 'builder-task-start',
      );
      assert.ok(
        taskPromptIndex >= BUILDER_HISTORY_TURN_COUNT,
        'the task must follow the semantically seeded history turns',
      );
      const taskRequests = fixture.requests.slice(taskPromptIndex);
      assert.equal(
        fixture.requests.filter((request) => request.phase === 'builder-seed')
          .length,
        BUILDER_HISTORY_TURN_COUNT,
      );
      assert.equal(
        taskRequests.filter((request) => request.responseStatus === 413).length,
        0,
      );
      const proactiveSummaryIndices = taskRequests.flatMap((request, index) =>
        request.phase.startsWith('builder-proactive-summary-') &&
        containsApiReadResult(request.body)
          ? [index]
          : [],
      );
      const continuationIndex = taskRequests.findIndex(
        (request) => request.phase === 'builder-read-continuation',
      );
      const completionIndex = taskRequests.findIndex(
        (request) => request.phase === 'builder-task-complete',
      );
      const proactiveCompaction = proactiveSummaryIndices.some(
        (index) => index < continuationIndex,
      );
      assert.ok(
        proactiveSummaryIndices.length <= 4,
        'each API page may cause at most a history and split-turn summary',
      );
      assert.ok(continuationIndex > 0);
      assert.equal(
        proactiveCompaction,
        true,
        'Pi must compact the first large API result before requesting the continued page',
      );
      assert.ok(completionIndex > continuationIndex);
      assert.ok(
        taskRequests.every((request) => request.phase !== 'unclassified'),
        `every builder provider call must have a semantic phase: ${taskRequests.map((request) => request.phase).join(', ')}`,
      );
      assert.ok(
        taskRequests.some(
          (request) => request.phase === 'builder-task-complete',
        ),
        'the task must complete after the continued API read',
      );
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
      const summaryRequest = taskRequests.find(
        (request) =>
          request.phase.includes('-summary-') &&
          containsApiReadResult(request.body),
      );
      assert.ok(summaryRequest, 'Pi should make a summary request');
      const piSessionBudget = events.filter(
        (event) =>
          event.type === 'debug' && event.label === 'pi-session-budget',
      );
      assert.ok(piSessionBudget.length > 0);
      const finalBudget = piSessionBudget.at(-1);
      const finalBudgetData = finalBudget ? eventData(finalBudget) : undefined;
      assert.ok(
        Number(finalBudgetData?.reportedInputTokens) > 0,
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
          finalBudgetData?.piEstimatedContextTokens ?? null,
        reportedInputTokens: finalBudgetData?.reportedInputTokens ?? 0,
        effectiveContextWindow: finalBudgetData?.contextWindow ?? 24_576,
        reserveTokens: PI_AI_COMPACTION_RESERVE_TOKENS,
        piProactiveThreshold:
          Number(finalBudgetData?.contextWindow ?? 24_576) -
          PI_AI_COMPACTION_RESERVE_TOKENS,
        summaryRequestEstimate: estimateInputTokensFromValue(
          summaryRequest.body,
        ),
      };
      context.diagnostic(
        JSON.stringify({
          sdk: packagePi.AgentSession.name,
          version: piCodingAgentVersion,
          defaultMaxBytes: packagePi.DEFAULT_MAX_BYTES,
          reserveTokens: PI_AI_COMPACTION_RESERVE_TOKENS,
          piSessionBudget: finalBudgetData,
          effectiveContextWindow: finalBudgetData?.contextWindow ?? 24_576,
          piEstimatedContextTokens:
            finalBudgetData?.piEstimatedContextTokens ?? null,
          reportedInputTokens: finalBudgetData?.reportedInputTokens ?? 0,
          ...budgetEvidence,
          tokenEvidence,
        }),
      );
      assert.ok(
        apiReadEvents.every(
          (event) =>
            Number(toolTraceOutput(event)?.textBytes) > 0 &&
            Number(toolTraceOutput(event)?.textBytes) <=
              packagePi.DEFAULT_MAX_BYTES,
        ),
        'every page returned to Pi must respect its exported byte ceiling',
      );
      assert.ok(
        events.some(
          (event) =>
            event.type === 'delta' &&
            String(event.text || '').includes(
              'The requested extension operation is complete.',
            ),
        ),
        'the completed builder response should stream through the actual adapter',
      );
      const taskStartRequest = taskRequests.find((request) =>
        hasBuilderTools(request.body),
      );
      assert.ok(taskStartRequest, 'builder should start the task with tools');
      assert.ok(
        taskStartRequest.body.tools.some(
          (tool: { function?: { name?: string } }) =>
            tool.function?.name === 'read_extension_api',
        ),
      );
      const builderToolNames = taskStartRequest.body.tools.map(
        (tool: { function?: { name?: string } }) => tool.function?.name,
      );
      assert.ok(isPiToolPolicy(toolPolicy));
      const policy = toolPolicy;
      const registeredToolNames = policy.registeredTools.map(
        (tool) => tool.name,
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
        policy.activeToolNames.every((name) => allowedBuilderTools.has(name)),
        'the builder must declare only tools in its active allowlist',
      );
      assert.ok(
        policy.callableToolNames.every((name) => allowedBuilderTools.has(name)),
        'the builder must keep deferred and callable tools within its allowlist',
      );
      assert.ok(
        !registeredToolNames.some((name) =>
          ['tool_search', 'codemode', 'mcp'].some((forbidden) =>
            name.toLowerCase().includes(forbidden),
          ),
        ),
        'the builder must not load tool search, codemode, or MCP tools',
      );
      for (const forbidden of [
        'bash',
        'edit',
        'write',
        'tool_search',
        'codemode',
        'mcp',
      ])
        assert.equal(
          taskStartRequest.body.tools.some(
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
      assert.ok(
        events.some(
          (event) =>
            event.type === 'delta' &&
            String(event.text || '').includes(
              'The follow-up change is complete.',
            ),
        ),
        'the recovered builder session must answer its follow-up through the adapter',
      );
      assert.equal(fixture.requests.at(-1)?.phase, 'builder-follow-up');
      fixtureEvidence.push({
        scenario: 'builder-proactive-compaction-continuation-follow-up',
        finalOutcome: 'done',
        proactiveCompaction,
        providerCalls: fixture.requests.length,
        requestPhases: fixture.requests.map((request) => request.phase),
        tokenEvidence,
        pageCount: apiReadEvents.length,
        toolAllowlist: builderToolNames,
        activeToolNames: policy.activeToolNames,
        callableToolNames: policy.callableToolNames,
        registeredTools: policy.registeredTools,
        managedHeadersPreserved: true,
        managedProviderIdentity: 'nevermind',
        providerApiFormat: 'openai-completions',
        budgetEvidence,
        followUpCompleted: true,
      });
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
      const events: FixtureAiEvent[] = [];
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
        const effectiveContextWindow = Number(
          budgetEvent
            ? eventDataValue(budgetEvent, 'contextWindow')
            : undefined,
        );
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
      const events: FixtureAiEvent[] = [];
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
      const events: FixtureAiEvent[] = [];
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
        const noticeKind = eventDataValue(terminal[0], 'kind');
        assert.equal(noticeKind, 'request_too_large');
        fixtureEvidence.push({
          scenario: 'byte-size-413-keeps-request-too-large-notice',
          finalOutcome: 'error',
          noticeKind,
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
      const events: FixtureAiEvent[] = [];
      try {
        const session = ai.session('extension-overflow-session', {
          model: 'fast',
          toolMode: 'none',
        });
        await seedLongPiSession(session);
        const response = await session.ask(
          'Continue this extension conversation after context recovery.',
          {
            images: [
              {
                type: 'image',
                data: 'iVBORw0KGgoAAAANSUhEUgAAAAIAAAACCAIAAAD91JpzAAAAEElEQVR4nGNQaDgARAwQCgAkDgWBfAdHdAAAAABJRU5ErkJggg==',
                mimeType: 'image/png',
              },
            ],
            onEvent: (event) => events.push(event),
          },
        );
        assert.match(String(response), /recovered successfully/);
        assert.deepEqual(
          terminalEvents(events).map((event) => event.type),
          ['done'],
        );
        const phaseTrace = assertLongSessionPhaseTrace(
          fixture.requests,
          'recovered',
        );
        const taskRequest = fixture.requests.find(
          (request) => request.phase === 'session-task',
        );
        assert.ok(taskRequest);
        assert.ok(
          JSON.stringify(taskRequest.body).includes('iVBORw0KGgo'),
          'the provider request should preserve supported image input',
        );
        const byoTokenEstimate = estimateInputTokensFromValue(taskRequest.body);
        assert.ok(byoTokenEstimate > 0);
        assert.ok(
          fixture.requests.every(
            (request) =>
              hasByoRequestHeaders(request) &&
              request.body.model === 'fixture-model',
          ),
          'BYO auth and model identity must be retained for normal, summary, and recovered requests',
        );
        fixtureEvidence.push({
          scenario: 'byo-extension-session-overflow-recovery',
          finalOutcome: 'done',
          providerCalls: fixture.requests.length,
          requestPhases: phaseTrace.phases,
          compactionPhases: phaseTrace.summaryKinds,
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
      modelId: 'fixture-model',
      expectedPath: '/v1/chat/completions',
      baseUrl: (fixture: ReturnType<typeof createProtocolFixture>) =>
        `${fixture.baseUrl}/v1`,
    },
    {
      provider: 'anthropic',
      api: 'anthropic-messages',
      modelId: 'claude-fixture',
      expectedPath: '/v1/messages?beta=true',
      baseUrl: (fixture: ReturnType<typeof createProtocolFixture>) =>
        fixture.baseUrl,
    },
    {
      provider: 'google',
      api: 'google-generative-ai',
      modelId: 'gemini-fixture',
      expectedPath:
        '/v1beta/models/gemini-fixture:streamGenerateContent?alt=sse',
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
          modelId: provider.modelId,
          modelName: 'Fixture Model',
        });
        const ai = createAi(fixture, []);
        const session = ai.session(`raw-413-${provider.provider}`, {
          model: 'fast',
          toolMode: 'none',
        });
        const events: FixtureAiEvent[] = [];
        try {
          await seedLongPiSession(session);
          const response = await session.ask(
            'Recover this provider request after Pi compacts.',
            { onEvent: (event) => events.push(event) },
          );
          assert.match(String(response), /recovered successfully/);
          const phaseTrace = assertLongSessionPhaseTrace(
            fixture.requests,
            'recovered',
          );
          assert.equal(
            fixture.managedDescriptors.length,
            0,
            'BYO provider identities must not be replaced by the managed Nevermind identity',
          );
          assert.deepEqual(
            terminalEvents(events).map((event) => event.type),
            ['done'],
          );
          assert.equal(
            fixture.requests.find((request) => request.phase === 'session-task')
              ?.responseStatus,
            413,
          );
          assert.deepEqual(fixture.unexpectedRoutes, []);
          assert.ok(
            fixture.requests.every(
              (request) =>
                hasByoRequestHeaders(request) &&
                (provider.provider === 'google'
                  ? request.path.includes(`/models/${provider.modelId}:`)
                  : request.body.model === provider.modelId),
            ),
            'BYO model and credentials must remain attached to every normal, summary, and recovered request',
          );
          assert.deepEqual(
            fixture.requests.map(({ method, path: requestPath }) => [
              method,
              requestPath,
            ]),
            Array.from({ length: fixture.requests.length }, () => [
              'POST',
              provider.expectedPath,
            ]),
            `${provider.provider} should use the installed SDK's exact endpoint beneath the configured base URL`,
          );
          fixtureEvidence.push({
            scenario: `provider-413-${provider.api}`,
            finalOutcome: 'done',
            providerCalls: fixture.requests.length,
            requestPhases: phaseTrace.phases,
            compactionPhases: phaseTrace.summaryKinds,
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
      modelId: 'fixture-model',
      expectedPath: '/v1/chat/completions',
      baseUrl: (fixture: ReturnType<typeof createProtocolFixture>) =>
        `${fixture.baseUrl}/v1`,
    },
    {
      provider: 'anthropic',
      api: 'anthropic-messages',
      modelId: 'claude-fixture',
      expectedPath: '/v1/messages?beta=true',
      baseUrl: (fixture: ReturnType<typeof createProtocolFixture>) =>
        fixture.baseUrl,
    },
    {
      provider: 'google',
      api: 'google-generative-ai',
      modelId: 'gemini-fixture',
      expectedPath:
        '/v1beta/models/gemini-fixture:streamGenerateContent?alt=sse',
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
          modelId: provider.modelId,
          modelName: 'Fixture Model',
        });
        const events: FixtureAiEvent[] = [];
        const ai = createAi(fixture, []);
        const session = ai.session(`unmodified-413-${provider.provider}`, {
          model: 'fast',
          toolMode: 'none',
        });
        let response: unknown;
        let error: unknown;
        try {
          await seedLongPiSession(session);
          try {
            response = await session.ask(
              'Recover this unmodified provider 413 when supported by Pi.',
              { onEvent: (event) => events.push(event) },
            );
          } catch (caught) {
            error = caught;
          }

          if (raw413Recognized) {
            assert.equal(error, undefined);
            assert.match(String(response), /recovered successfully/);
            const phaseTrace = assertLongSessionPhaseTrace(
              fixture.requests,
              'recovered',
            );
            assert.deepEqual(
              terminalEvents(events).map((event) => event.type),
              ['done'],
            );
            fixtureEvidence.push({
              scenario: `provider-raw-413-${provider.api}`,
              finalOutcome: 'done',
              providerCalls: fixture.requests.length,
              requestPhases: phaseTrace.phases,
              compactionPhases: phaseTrace.summaryKinds,
              raw413Recovered: true,
            });
          } else {
            assert.ok(error instanceof Error);
            assertLongSessionPhaseTrace(fixture.requests, 'raw-413');
            assert.deepEqual(
              terminalEvents(events).map((event) => event.type),
              ['error'],
            );
            fixtureEvidence.push({
              scenario: `provider-raw-413-${provider.api}`,
              finalOutcome: 'error',
              providerCalls: fixture.requests.length,
              raw413Recovered: false,
            });
          }
          assert.deepEqual(
            fixture.requests.map(({ method, path: requestPath }) => [
              method,
              requestPath,
            ]),
            Array.from({ length: fixture.requests.length }, () => [
              'POST',
              provider.expectedPath,
            ]),
            `${provider.provider} should use the installed SDK's exact endpoint beneath the configured base URL`,
          );
          assert.deepEqual(fixture.unexpectedRoutes, []);
          assert.ok(
            fixture.requests.every(
              (request) =>
                hasByoRequestHeaders(request) &&
                (provider.provider === 'google'
                  ? request.path.includes(`/models/${provider.modelId}:`)
                  : request.body.model === provider.modelId),
            ),
            'BYO model and credentials must remain attached when Pi classifies an unmodified 413',
          );
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
    const events: FixtureAiEvent[] = [];
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
    const events: FixtureAiEvent[] = [];
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
      await fixture.waitForRequests(2);
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

test('cancellation during model preparation prevents the next provider request', async () => {
  await withFixture('prepare-cancel', 'prepare-cancel', async (fixture) => {
    const ai = createAi(fixture, []);
    const events: FixtureAiEvent[] = [];
    const session = ai.session('prepare-cancel-session', {
      model: 'fast',
      toolMode: 'none',
    });
    try {
      await session.ask('Complete the first fixture request.');
      assert.equal(fixture.requests.length, 1);

      const controller = new AbortController();
      const pending = session.ask('Cancel before this request is sent.', {
        signal: controller.signal,
        onEvent: (event) => events.push(event),
      });
      await fixture.waitForModelRefresh();
      controller.abort();
      fixture.releaseModelRefresh();

      await assert.rejects(pending, /aborted/i);
      assert.equal(fixture.requests.length, 1);
      assert.deepEqual(
        terminalEvents(events).map((event) => event.type),
        ['aborted'],
      );
      fixtureEvidence.push({
        scenario: 'general-session-cancellation-during-model-prepare',
        finalOutcome: 'aborted',
        providerCalls: fixture.requests.length,
        promptSuppressed: true,
      });
    } finally {
      fixture.releaseModelRefresh();
      await ai.disposeAllSessions();
    }
  });
});

test('Pi summary auth and credit failures produce one terminal error, never success', async () => {
  for (const scenario of ['summary-unauthorized', 'summary-credits'] as const) {
    await withFixture(scenario, scenario, async (fixture) => {
      const ai = createAi(fixture, []);
      const session = ai.session(`summary-failure-${scenario}`, {
        model: 'fast',
        toolMode: 'none',
      });
      const events: FixtureAiEvent[] = [];
      try {
        await seedLongPiSession(session);
        await assert.rejects(
          session.ask(
            'Recover this request after Pi summarizes the rejected input.',
            { onEvent: (event) => events.push(event) },
          ),
        );
        const phaseTrace = assertLongSessionPhaseTrace(
          fixture.requests,
          'summary-error',
        );
        assert.deepEqual(
          terminalEvents(events).map((event) => event.type),
          ['error'],
        );
        fixtureEvidence.push({
          scenario: `summary-failure-${scenario}`,
          finalOutcome: 'error',
          providerCalls: fixture.requests.length,
          requestPhases: phaseTrace.phases,
          compactionPhases: phaseTrace.summaryKinds,
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
      const session = ai.session('recovery-exhausted-session', {
        model: 'fast',
        toolMode: 'none',
      });
      const events: FixtureAiEvent[] = [];
      try {
        await seedLongPiSession(session);
        await assert.rejects(
          session.ask(
            'Return an error if Pi cannot recover the rejected request.',
            {
              onEvent: (event) => events.push(event),
            },
          ),
        );
        const phaseTrace = assertLongSessionPhaseTrace(
          fixture.requests,
          'recovery-exhausted',
        );
        assert.deepEqual(
          terminalEvents(events).map((event) => event.type),
          ['error'],
        );
        fixtureEvidence.push({
          scenario: 'overflow-recovery-exhausted',
          finalOutcome: 'error',
          providerCalls: fixture.requests.length,
          requestPhases: phaseTrace.phases,
          compactionPhases: phaseTrace.summaryKinds,
          recoveryRejectedByBackend: true,
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
    const events: FixtureAiEvent[] = [];
    const controller = new AbortController();
    const session = ai.session('summary-cancel-session', {
      model: 'fast',
      toolMode: 'none',
    });
    try {
      await seedLongPiSession(session);
      const pending = session.ask(
        'Cancel while Pi is preparing a context summary.',
        {
          signal: controller.signal,
          onEvent: (event) => events.push(event),
        },
      );
      let summaryTimeout: ReturnType<typeof setTimeout> | undefined;
      try {
        await Promise.race([
          fixture.waitForPhase('session-summary-history'),
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
      const phaseTrace = assertLongSessionPhaseTrace(
        fixture.requests,
        'summary-pending',
      );
      assert.deepEqual(
        terminalEvents(events).map((event) => event.type),
        ['aborted'],
      );
      fixtureEvidence.push({
        scenario: 'host-cancellation-during-compaction',
        finalOutcome: 'aborted',
        providerCalls: fixture.requests.length,
        requestPhases: phaseTrace.phases,
        compactionPhases: phaseTrace.summaryKinds,
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
      const events: FixtureAiEvent[] = [];
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
      const events: FixtureAiEvent[] = [];
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
      const events: FixtureAiEvent[] = [];
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
    const events: FixtureAiEvent[] = [];
    const ai = createAi(fixture, events);
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
      const policyEvent = events.find(
        (event) => event.type === 'debug' && event.label === 'pi-tool-policy',
      );
      const policyValue = policyEvent ? eventData(policyEvent) : undefined;
      assert.ok(isPiToolPolicy(policyValue));
      const policy = policyValue;
      assert.deepEqual(policy.activeToolNames, names);
      assert.deepEqual(policy.callableToolNames, names);
      for (const inactiveWebTool of ['source_check', 'get_search_content']) {
        assert.equal(policy.activeToolNames.includes(inactiveWebTool), false);
        assert.equal(
          policy.callableToolNames.includes(inactiveWebTool),
          false,
          `${inactiveWebTool} must remain uncallable, whether registered or deferred`,
        );
      }
      assert.equal(
        policy.registeredTools.some((tool) =>
          ['tool_search', 'codemode', 'mcp'].some((forbidden) =>
            tool.name.toLowerCase().includes(forbidden),
          ),
        ),
        false,
        'tool discovery and native extension tools must not be registered',
      );
      fixtureEvidence.push({
        scenario: 'conversation-explicit-read-only-tool-allowlist',
        finalOutcome: 'done',
        providerCalls: fixture.requests.length,
        toolAllowlist: names,
        activeToolNames: policy.activeToolNames,
        callableToolNames: policy.callableToolNames,
        registeredTools: policy.registeredTools,
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
