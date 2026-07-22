import assert from 'node:assert/strict';
import test from 'node:test';
import {
  type AppIpcHandlersDeps,
  registerAppIpcHandlers,
} from './app-ipc-handlers';

// biome-ignore lint/complexity/noExcessiveLinesPerFunction: test harness creates many stubs
function createDeps(overrides: Partial<AppIpcHandlersDeps> = {}) {
  // biome-ignore lint/suspicious/noExplicitAny: test harness uses any for flexibility
  const handles = new Map<string, (event: any, ...args: any[]) => unknown>();
  // biome-ignore lint/suspicious/noExplicitAny: test harness uses any for flexibility
  const listeners = new Map<string, (...args: any[]) => unknown>();
  const calls: string[] = [];
  const deps: AppIpcHandlersDeps = {
    ipcMain: {
      handle: (channel, handler) => handles.set(channel, handler),
      on: (channel, handler) => listeners.set(channel, handler),
    },
    measureDebugPerformance: (_name, _data, fn) => fn(),
    summarizeDebugValue: (value) => value,
    startSearch: (sender, input) => ({ sender, input }),
    cancelSearch: (sender, input) => calls.push(`cancel:${sender}:${input}`),
    executeActionForIpc: (action) => action,
    executeViewActionForIpc: (action) => action,
    refreshViewForIpc: (input) => ({ refreshed: input }),
    pickFormFieldPaths: (_event, input) => ({ picked: input }),
    startFileDrag: () => calls.push('drag'),
    sendAiChatMessage: (message, chatId) => ({ message, chatId }),
    noteAiChatExited: (chatId) => ({ chatId }),
    abortAiChat: (chatId) => ({ chatId }),
    resetAiChat: (chatId) => ({ chatId }),
    setAlias: (action, alias) => ({ action, alias }),
    removeAlias: (action, alias) => ({ action, alias }),
    setShortcut: (action, shortcut) => ({ action, shortcut }),
    setPaletteHotkey: (accelerator) => ({ accelerator }),
    getSetting: (id) => ({ id }),
    openSystemKeyboardSettings: () => 'keyboard-settings',
    getShortcuts: () => [],
    removeShortcut: (actionId) => ({ actionId }),
    unregisterActionShortcuts: () => calls.push('suspend-actions'),
    registerActionShortcuts: () => calls.push('resume-actions'),
    suspendPaletteHotkey: () => calls.push('suspend-hotkey'),
    resumePaletteHotkey: () => calls.push('resume-hotkey'),
    setOverride: (action, instruction) => ({ action, instruction }),
    clearOverride: (action) => action,
    duplicateCreatedAction: (action) => action,
    removeCreatedAction: (action) => action,
    getOrCreateExtensionChat: (file, title) => ({ file, title, messages: [] }),
    aiChatView: (item, options) => ({ type: 'chat', item, options }),
    normalizeHostViewResult: (result) => ({ normalized: result }),
    createDraftAiChat: (prompt) => ({ prompt, messages: [] }),
    getNevermindAuth: async () => null,
    getNevermindDebugStatus: () => ({
      client: { environment: 'production', baseUrl: 'https://api.nvm.fyi' },
      active: { environment: 'production', baseUrl: 'https://api.nvm.fyi' },
      backend: { environment: 'production', version: 'dev' },
    }),
    setActiveNevermindBaseUrl: (baseUrl) => calls.push(`base:${baseUrl || ''}`),
    warmNevermindCompatibilityCache: (baseUrl) => calls.push(`warm:${baseUrl}`),
    logInfo: (message) => calls.push(`info:${message}`),
    userDataPath: () => '/user-data',
    signInToNevermind: async () => ({ ok: false, error: 'nope' }),
    invalidateExtensionRootItems: () => calls.push('invalidate'),
    broadcastAuthChanged: () => calls.push('broadcast-auth'),
    appIconCache: { get: (appPath) => ({ appPath }) },
    runningAppStatus: { getForRenderer: (appPaths) => ({ appPaths }) },
    paletteWindow: {
      setPaletteSizeForMode: (mode) => calls.push(`mode:${mode}`),
      centerWindow: () => calls.push('center'),
      hidePalette: () => calls.push('hide'),
      revealPalette: () => calls.push('reveal'),
    },
    requestQuitApp: (reason) => calls.push(`quit:${reason}`),
    hasCapability: () => true,
    processPlatform: 'darwin',
    getCameraMediaAccessStatus: () => 'granted',
    extensionWindowManager: {
      getStateForSender: (sender) => ({ sender }),
      closeForSender: (sender) => {
        calls.push(`close-window:${String(sender)}`);
        return true;
      },
    },
    saveExtensionDraft: (input) => ({ ok: true, input }),
    logError: (message) => calls.push(`error:${message}`),
    logWarn: (message) => calls.push(`warn:${message}`),
    loggerDebug: (message) => calls.push(`debug:${message}`),
    probeGh: async () => ({ installed: true, authed: true }),
    ...overrides,
  };
  registerAppIpcHandlers(deps);
  return { handles, listeners, calls };
}

test('registerAppIpcHandlers registers core invoke handlers and drag listener', () => {
  const { handles, listeners } = createDeps();

  assert.equal(handles.has('actions:search'), true);
  assert.equal(handles.has('view:refresh'), true);
  assert.equal(handles.has('nevermind:auth-status'), true);
  assert.equal(handles.has('nevermind:debug-status'), true);
  assert.equal(handles.has('camera:request-access'), true);
  assert.equal(handles.has('logs:write'), true);
  assert.equal(listeners.has('drag:file'), true);
  assert.equal(listeners.has('actions:search:cancel'), true);
});

test('search IPC stays scoped to the originating sender', async () => {
  const starts: unknown[] = [];
  const cancels: unknown[] = [];
  const { handles, listeners } = createDeps({
    startSearch: (sender, input) => {
      starts.push({ sender, input });
      return { ok: true };
    },
    cancelSearch: (sender, input) => cancels.push({ sender, input }),
  });
  const sender = { id: 42 };
  const input = { query: 'abc', generation: 3 };
  assert.deepEqual(await handles.get('actions:search')?.({ sender }, input), {
    ok: true,
  });
  listeners.get('actions:search:cancel')?.({ sender }, { generation: 3 });
  assert.deepEqual(starts, [{ sender, input }]);
  assert.deepEqual(cancels, [{ sender, input: { generation: 3 } }]);
});

test('registerAppIpcHandlers preserves palette, camera, and window behavior', async () => {
  const { handles, calls } = createDeps();

  assert.deepEqual(
    await handles.get('palette:set-mode')?.({}, 'preview'),
    undefined,
  );
  assert.deepEqual(await handles.get('camera:request-access')?.({}), {
    ok: true,
    status: 'granted',
  });
  assert.deepEqual(
    await handles.get('extension-window:get-state')?.({ sender: 'window-a' }),
    {
      sender: 'window-a',
    },
  );
  await handles.get('extension-window:close')?.({ sender: 'sender' });

  assert.deepEqual(calls, ['mode:preview', 'center', 'close-window:sender']);
});

test('camera permission composition distinguishes Windows from unsupported platforms', async () => {
  const windows = createDeps({
    processPlatform: 'win32',
    getCameraMediaAccessStatus: () => {
      throw new Error('Windows must not use the macOS media status API');
    },
  });
  assert.deepEqual(await windows.handles.get('camera:request-access')?.({}), {
    ok: true,
    status: 'unknown',
  });

  const unsupported = createDeps({ hasCapability: () => false });
  assert.deepEqual(
    await unsupported.handles.get('camera:request-access')?.({}),
    { ok: false, status: 'unsupported' },
  );
});

test('registerAppIpcHandlers suspends and resumes palette hotkey with action shortcuts', async () => {
  const { handles, calls } = createDeps();

  await handles.get('actions:suspend-shortcuts')?.({});
  await handles.get('actions:resume-shortcuts')?.({});

  assert.deepEqual(calls, [
    'suspend-hotkey',
    'suspend-actions',
    'resume-actions',
    'resume-hotkey',
  ]);
});

test('registerAppIpcHandlers redacts auth status logs and keeps sign-in failures stable', async () => {
  const logEntries: unknown[] = [];
  const { handles } = createDeps({
    getNevermindAuth: async () => ({
      baseUrl: 'https://nvm.example',
      email: 'pablo@example.com',
    }),
    logInfo: (_message, data) => logEntries.push(data),
    signInToNevermind: async () => ({ ok: false, error: 'provider-secret' }),
  });

  assert.deepEqual(await handles.get('nevermind:auth-status')?.({}), {
    authed: true,
    email: 'pablo@example.com',
  });
  assert.deepEqual(logEntries, [{ authed: true }]);
  assert.deepEqual(await handles.get('nevermind:sign-in')?.({}), {
    ok: false,
    error: 'Unable to sign in',
  });
});

test('registerAppIpcHandlers exposes backend debug status', async () => {
  const status = {
    client: { environment: 'custom', baseUrl: 'https://preview.example' },
    active: { environment: 'custom', baseUrl: 'https://preview.example' },
    backend: { environment: 'preview', version: 'abc1234' },
  };
  const { handles } = createDeps({ getNevermindDebugStatus: () => status });

  assert.deepEqual(await handles.get('nevermind:debug-status')?.({}), status);
});

test('registerAppIpcHandlers handles auth status and sign in side effects', async () => {
  const { handles, calls } = createDeps({
    getNevermindAuth: async () => ({
      baseUrl: 'https://nvm.example',
      email: 'pablo@example.com',
    }),
    signInToNevermind: async () => ({
      ok: true,
      auth: { baseUrl: 'https://nvm.example', email: 'pablo@example.com' },
    }),
  });

  assert.deepEqual(await handles.get('nevermind:auth-status')?.({}), {
    authed: true,
    email: 'pablo@example.com',
  });
  assert.deepEqual(await handles.get('nevermind:sign-in')?.({}), {
    ok: true,
    email: 'pablo@example.com',
  });
  assert.deepEqual(calls, [
    'base:https://nvm.example',
    'warm:https://nvm.example',
    'info:nevermind.auth-status.check',
    'base:https://nvm.example',
    'warm:https://nvm.example',
    'invalidate',
    'broadcast-auth',
  ]);
});
