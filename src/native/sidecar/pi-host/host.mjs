// The pi host: the agent runtime Nevermind starts as a child process. It speaks JSON lines on stdio with the app.
//
// The app owns the user, the tools that touch the computer and every approval. This process owns the conversation:
// the model, its reasoning, compaction, and the web tools of pi-web-access. A session lives as long as the process, so
// a chat keeps its context between turns ("durable"); the app starts one process per chat.
//
// App -> host, one JSON object per line:
//   {type:"prompt", id, baseUrl, token, headers, role, system, text, images, history, tools, workspaceDir, agentDir}
//   {type:"tool_result", callId, text, status}   status: ran | failed | refused | cancelled
//   {type:"abort"}
//   {type:"shutdown"}
// Host -> app:
//   {type:"ready"}
//   {type:"delta", text} {type:"reasoning", text}
//   {type:"step"}
//   {type:"native_tool", phase:"start"|"end", callId, name, args, isError}   a tool this process runs itself
//   {type:"tool_call", callId, name, arguments}                              a tool the app runs and judges
//   {type:"done", usage} {type:"error", message, code} {type:"aborted"}

import readline from 'node:readline';
import path from 'node:path';
import fs from 'node:fs/promises';
import { createRequire } from 'node:module';

const require = createRequire(import.meta.url);

const PROVIDER_ID = 'nevermind';
const COMPACTION_RESERVE_TOKENS = 16_384;
const WEB_TOOLS = ['web_search', 'fetch_content'];
const THINKING_LEVELS = new Set(['off', 'minimal', 'low', 'medium', 'high', 'xhigh', 'max']);
const ACTIVE_MODEL_TIMEOUT_MS = 15_000;

function send(message) {
  process.stdout.write(`${JSON.stringify(message)}\n`);
}

function fail(code, error) {
  send({ type: 'error', code, message: error instanceof Error ? error.message : String(error) });
}

let pi = null;
let piAi = null;
let session = null;
let sessionKey = '';
let turn = null; // the prompt in flight: { id, aborted }
const pendingCalls = new Map(); // callId -> resolve

async function libraries() {
  if (!pi) pi = await import('@earendil-works/pi-coding-agent');
  if (!piAi) piAi = await import('@earendil-works/pi-ai');
  return { pi, piAi };
}

function normalizeThinkingLevel(value) {
  return THINKING_LEVELS.has(value) ? value : 'off';
}

// The backend reads the role from `model` and the prompt size from `chars`, as the desktop app sends them.
async function fetchActiveModel(baseUrl, token, headers, role, chars) {
  const trimmed = baseUrl.replace(/\/$/, '');
  const query = new URLSearchParams();
  if (role) query.set('model', role);
  if (Number.isFinite(chars) && chars > 0) query.set('chars', String(Math.floor(chars)));
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), ACTIVE_MODEL_TIMEOUT_MS);
  try {
    const response = await fetch(`${trimmed}/api/v1/active-model${query.size ? `?${query}` : ''}`, {
      headers: { ...headers, Authorization: `Bearer ${token}` },
      signal: controller.signal,
    });
    if (response.status === 401) {
      const error = new Error('Sign in to Nevermind to use the assistant.');
      error.code = 'auth_required';
      throw error;
    }
    if (!response.ok) throw new Error(`The model request failed (${response.status}).`);
    return refuseBlockedRoute(await response.json());
  } finally {
    clearTimeout(timeout);
  }
}

// A route the backend marks as blocked by the free spend cap is refused before any request, as the app's own chat does.
function refuseBlockedRoute(descriptor) {
  if (descriptor?.notice === 'blocked' && descriptor?.noticeReason === 'free_spend_cap_reached') {
    const error = new Error('The route is paused by the free spend cap.');
    error.code = 'free_spend_cap_reached';
    throw error;
  }
  return descriptor;
}

function modelFor(descriptor, headers, role) {
  const window = descriptor.maxInputTokens
    ? Math.min(descriptor.contextWindow, descriptor.maxInputTokens)
    : descriptor.contextWindow;
  return {
    id: descriptor.id,
    name: descriptor.name,
    api: descriptor.api,
    provider: descriptor.provider,
    baseUrl: descriptor.baseUrl,
    reasoning: descriptor.reasoning,
    input: descriptor.input,
    cost: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0 },
    contextWindow: window,
    maxTokens: descriptor.maxTokens,
    headers: {
      ...headers,
      ...(role ? { 'X-Nevermind-AI-Model': role } : {}),
      ...(descriptor.modelTier ? { 'X-Nevermind-AI-Model-Tier': descriptor.modelTier } : {}),
      ...(descriptor.creditKind ? { 'X-Nevermind-AI-Credit-Kind': descriptor.creditKind } : {}),
    },
  };
}

// registerProvider makes the app's backend a pi provider. A model whose API the provider does not speak natively goes
// through pi-ai's compatibility adapters, as it does in 1.x.
async function registerBackend(modelRuntime, descriptor, model) {
  const compatibility = await import('@earendil-works/pi-ai/compat');
  const native = modelRuntime.getRegisteredNativeProvider(PROVIDER_ID);
  const nativeApis = new Set(native?.getModels().map((m) => m.api) ?? []);
  modelRuntime.registerProvider(PROVIDER_ID, {
    name: 'Nevermind',
    api: descriptor.api,
    baseUrl: descriptor.baseUrl,
    models: [model],
    streamSimple(m, context, options) {
      if (native && nativeApis.has(m.api)) return native.streamSimple(m, context, options);
      const adapter = compatibility.getApiProvider(m.api);
      if (!adapter) throw new Error(`No pi API adapter is registered for "${m.api}".`);
      return adapter.streamSimple(m, context, options);
    },
  });
}

function compactionSettings(model) {
  return {
    enabled: true,
    reserveTokens: COMPACTION_RESERVE_TOKENS,
    keepRecentTokens: Math.min(
      pi.DEFAULT_COMPACTION_SETTINGS.keepRecentTokens,
      Math.max(1, Math.floor(model.contextWindow - COMPACTION_RESERVE_TOKENS - 1)),
    ),
  };
}

// hostTools are the app's tools as pi tools: a call is sent to the app, which checks its policy, asks the user when the
// tool needs it, runs it, and answers with the text the model reads.
function hostTools(definitions) {
  return definitions.map((definition) =>
    pi.defineTool({
      name: definition.name,
      label: definition.label || definition.name,
      description: definition.description,
      parameters: piAi.Type.Unsafe(definition.parameters),
      async execute(toolCallId, params, signal) {
        const result = await new Promise((resolve) => {
          pendingCalls.set(toolCallId, resolve);
          signal?.addEventListener('abort', () => resolve({ text: 'Stopped before it ran.', status: 'cancelled' }), {
            once: true,
          });
          send({ type: 'tool_call', callId: toolCallId, name: definition.name, arguments: params });
        });
        pendingCalls.delete(toolCallId);
        // A call that did not run is an error to the model, with the reason the app gave it.
        if (result.status !== 'ran') throw new Error(result.text);
        return { content: [{ type: 'text', text: result.text }], details: { status: 'ran' } };
      },
    }),
  );
}

async function resourceLoader({ agentDir, workspaceDir, settingsManager, system }) {
  const webAccess = path.dirname(require.resolve('pi-web-access/package.json'));
  const loader = new pi.DefaultResourceLoader({
    agentDir,
    cwd: workspaceDir,
    settingsManager,
    additionalExtensionPaths: [webAccess],
    disabledBuiltinExtensions: ['codemode', 'mcp', 'tool-search'],
    noExtensions: true,
    noSkills: true,
    noPromptTemplates: true,
    noThemes: true,
    noContextFiles: true,
    systemPrompt: system,
  });
  await loader.reload();
  const errors = loader.getExtensions().errors;
  if (errors.length > 0) throw new Error(`The web tools failed to load: ${errors[0].error}`);
  return loader;
}

async function createSession(prompt) {
  await libraries();
  const { agentDir, workspaceDir } = prompt;
  await fs.mkdir(agentDir, { recursive: true });
  await fs.mkdir(workspaceDir, { recursive: true });
  const descriptor = await fetchActiveModel(prompt.baseUrl, prompt.token, prompt.headers, prompt.role, prompt.chars);
  const model = modelFor(descriptor, prompt.headers, prompt.role);
  const modelRuntime = await pi.ModelRuntime.create({
    authPath: path.join(agentDir, 'auth.json'),
    modelsPath: null,
    allowModelNetwork: false,
  });
  await registerBackend(modelRuntime, descriptor, model);
  await modelRuntime.setRuntimeApiKey(PROVIDER_ID, prompt.token);
  const settingsManager = pi.SettingsManager.inMemory({
    compaction: compactionSettings(model),
    retry: { enabled: true, maxRetries: 2 },
  });
  const loader = await resourceLoader({ agentDir, workspaceDir, settingsManager, system: prompt.system });
  const customTools = hostTools(prompt.tools || []);
  const allowed = [...customTools.map((tool) => tool.name), ...WEB_TOOLS];
  const created = await pi.createAgentSession({
    cwd: workspaceDir,
    agentDir,
    model,
    thinkingLevel: normalizeThinkingLevel(descriptor.thinkingLevel),
    modelRuntime,
    resourceLoader: loader,
    noTools: 'builtin',
    tools: allowed,
    customTools,
    sessionManager: pi.SessionManager.inMemory(workspaceDir),
    settingsManager,
  });
  const missing = WEB_TOOLS.filter((name) => !created.session.agent.state.tools.some((tool) => tool.name === name));
  if (missing.length > 0) {
    created.session.dispose?.();
    throw new Error(`The web tools failed to load: ${missing.join(', ')}`);
  }
  created.session.subscribe(onSessionEvent);
  return { session: created.session, modelRuntime };
}

let runtime = null;

function onSessionEvent(event) {
  if (!turn) return;
  if (event.type === 'message_update' && event.assistantMessageEvent?.type === 'text_delta') {
    send({ type: 'delta', text: event.assistantMessageEvent.delta });
  } else if (event.type === 'message_update' && event.assistantMessageEvent?.type === 'thinking_delta') {
    send({ type: 'reasoning', text: event.assistantMessageEvent.delta });
  } else if (event.type === 'turn_start') {
    send({ type: 'step' });
  } else if (event.type === 'tool_execution_start' && WEB_TOOLS.includes(event.toolName)) {
    send({ type: 'native_tool', phase: 'start', callId: event.toolCallId, name: event.toolName, args: event.args });
  } else if (event.type === 'tool_execution_end' && WEB_TOOLS.includes(event.toolName)) {
    send({ type: 'native_tool', phase: 'end', callId: event.toolCallId, name: event.toolName, isError: Boolean(event.isError) });
  }
}

function sessionKeyOf(prompt) {
  return JSON.stringify([prompt.baseUrl, prompt.role, prompt.system, (prompt.tools || []).map((t) => t.name)]);
}

function images(list) {
  return (list || []).map((image) => ({ type: 'image', data: image.data, mimeType: image.mimeType }));
}

// ensureSession makes the session of a request ready. A session that already exists is kept, after the route of this
// request's size is checked, so a blocked route is refused on every turn.
async function ensureSession(prompt) {
  const key = sessionKeyOf(prompt);
  if (!session || sessionKey !== key) {
    session?.dispose?.();
    session = null;
    runtime = await createSession(prompt);
    session = runtime.session;
    sessionKey = key;
    return;
  }
  await fetchActiveModel(prompt.baseUrl, prompt.token, prompt.headers, prompt.role, prompt.chars);
  await runtime.modelRuntime.setRuntimeApiKey(PROVIDER_ID, prompt.token);
}

// handlePrepare readies the session without a prompt, so the first reply does not pay for it.
async function handlePrepare(request) {
  if (turn) return fail('busy', 'A reply is already running.');
  turn = { id: request.id, aborted: false };
  try {
    await ensureSession(request);
    send({ type: 'prepared', id: request.id });
  } catch (error) {
    fail(error?.code || 'failed', error);
  } finally {
    turn = null;
  }
}

async function handlePrompt(prompt) {
  if (turn) return fail('busy', 'A reply is already running.');
  turn = { id: prompt.id, aborted: false };
  try {
    await ensureSession(prompt);
    const text = prompt.history ? `${prompt.history}\n\n${prompt.text}` : prompt.text;
    await session.prompt(text, { images: images(prompt.images) });
    if (turn.aborted) send({ type: 'aborted' });
    else send({ type: 'done', usage: usageOf(session) });
  } catch (error) {
    if (turn?.aborted || /abort/i.test(String(error?.name || error?.message))) send({ type: 'aborted' });
    else fail(error?.code || 'failed', error);
  } finally {
    turn = null;
  }
}

function usageOf(active) {
  try {
    const stats = active.getSessionStats?.();
    return { inputTokens: stats?.tokens?.input ?? 0, outputTokens: stats?.tokens?.output ?? 0 };
  } catch {
    return { inputTokens: 0, outputTokens: 0 };
  }
}

async function abort() {
  if (!turn) return;
  turn.aborted = true;
  for (const resolve of pendingCalls.values()) resolve({ text: 'Stopped before it ran.', status: 'cancelled' });
  await session?.abort?.();
}

const lines = readline.createInterface({ input: process.stdin });
lines.on('line', (line) => {
  let message;
  try {
    message = JSON.parse(line);
  } catch {
    return fail('protocol', 'A line was not JSON.');
  }
  if (message.type === 'prompt') void handlePrompt(message);
  else if (message.type === 'prepare') void handlePrepare(message);
  else if (message.type === 'tool_result') pendingCalls.get(message.callId)?.({ text: message.text, status: message.status });
  else if (message.type === 'abort') void abort();
  else if (message.type === 'shutdown') {
    session?.dispose?.();
    process.exit(0);
  }
});
lines.on('close', () => {
  session?.dispose?.();
  process.exit(0);
});
process.on('uncaughtException', (error) => fail('crashed', error));
process.on('unhandledRejection', (error) => fail('crashed', error));

send({ type: 'ready' });
