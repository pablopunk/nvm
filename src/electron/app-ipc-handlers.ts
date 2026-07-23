import {
  createMeasuredIpcRegistrar,
  type IpcMainLike,
} from './ipc-registration';

export interface AppIpcHandlersDeps {
  ipcMain: IpcMainLike & {
    // biome-ignore lint/suspicious/noExplicitAny: deps-object pattern uses any for maximum caller flexibility
    on(channel: string, handler: (...args: any[]) => unknown): void;
  };
  measureDebugPerformance: <T>(
    name: string,
    data: Record<string, unknown>,
    fn: () => T | Promise<T>,
  ) => T | Promise<T>;
  summarizeDebugValue: (value: unknown) => unknown;
  startSearch: (sender: unknown, input: unknown) => unknown;
  cancelSearch: (sender: unknown, input: unknown) => unknown;
  executeActionForIpc: (action: unknown) => unknown;
  executeViewActionForIpc: (action: unknown) => unknown;
  refreshViewForIpc: (input: unknown) => unknown;
  pickFormFieldPaths: (event: unknown, input?: unknown) => unknown;
  // biome-ignore lint/suspicious/noExplicitAny: deps-object pattern uses any for maximum caller flexibility
  startFileDrag: (...args: any[]) => unknown;
  sendAiChatMessage: (message: unknown, chatId: unknown) => unknown;
  noteAiChatExited: (chatId: unknown) => unknown;
  abortAiChat: (chatId: unknown) => unknown;
  resetAiChat: (chatId: unknown) => unknown;
  setAlias: (action: unknown, alias: unknown) => unknown;
  removeAlias: (action: unknown, alias: unknown) => unknown;
  setShortcut: (action: unknown, shortcut: unknown) => unknown;
  setPaletteHotkey: (accelerator: unknown) => unknown;
  getSetting: (id: unknown) => unknown;
  openSystemKeyboardSettings: () => unknown;
  getShortcuts: () => unknown;
  removeShortcut: (actionId: unknown) => unknown;
  unregisterActionShortcuts: () => unknown;
  registerActionShortcuts: () => unknown;
  suspendPaletteHotkey: () => unknown;
  resumePaletteHotkey: () => unknown;
  setOverride: (action: unknown, instruction: unknown) => unknown;
  clearOverride: (action: unknown) => unknown;
  duplicateCreatedAction: (action: unknown) => unknown;
  removeCreatedAction: (action: unknown) => unknown;
  getOrCreateExtensionChat: (file: unknown, title: unknown) => unknown;
  aiChatView: (item: unknown, options?: Record<string, unknown>) => unknown;
  normalizeHostViewResult: (result: unknown) => unknown;
  createDraftAiChat: (prompt: string) => unknown;
  getNevermindAuth: () => Promise<{ baseUrl?: string; email?: string } | null>;
  getNevermindDebugStatus: () => {
    client: { environment: string; baseUrl: string };
    active: { environment: string; baseUrl: string };
    backend: { environment: string; version: string } | null;
  };
  setActiveNevermindBaseUrl: (baseUrl: string | null) => void;
  warmNevermindCompatibilityCache: (baseUrl: string) => unknown;
  logInfo: (message: string, data?: unknown, context?: unknown) => unknown;
  userDataPath: () => string;
  signInToNevermind: () => Promise<
    | { ok: true; auth: { baseUrl: string; email?: string } }
    | { ok: false; error?: string }
  >;
  invalidateExtensionRootItems: () => unknown;
  broadcastAuthChanged: (status: {
    authed: boolean;
    email?: string;
  }) => unknown;
  appIconCache: { get(appPath: unknown): unknown };
  runningAppStatus: { getForRenderer(appPaths: unknown): unknown };
  paletteWindow: {
    setPaletteSizeForMode(mode: unknown): unknown;
    centerWindow(): unknown;
    hidePalette(): unknown;
    revealPalette(): unknown;
  };
  requestQuitApp: (reason: string) => unknown;
  hasCapability: (capability: string) => boolean;
  processPlatform: NodeJS.Platform | string;
  getCameraMediaAccessStatus: () => string;
  extensionWindowManager: {
    getStateForSender(sender: unknown): unknown;
    closeForSender(sender: unknown): boolean;
  };
  saveExtensionDraft: (input: unknown) => unknown;
  logError: (message: string, data?: unknown, context?: unknown) => unknown;
  logWarn: (message: string, data?: unknown, context?: unknown) => unknown;
  loggerDebug: (message: string, data?: unknown, context?: unknown) => unknown;
  probeGh: () => Promise<{ installed: boolean; authed: boolean }>;
}

// biome-ignore lint/complexity/noExcessiveLinesPerFunction: declarative IPC registration table
export function registerAppIpcHandlers(deps: AppIpcHandlersDeps) {
  const ipcHandleMeasured = createMeasuredIpcRegistrar({
    ipcMain: deps.ipcMain,
    measure: deps.measureDebugPerformance,
    summarize: deps.summarizeDebugValue,
  });

  ipcHandleMeasured('actions:search', (event, input) =>
    deps.startSearch(event.sender, input),
  );
  deps.ipcMain.on('actions:search:cancel', (event, input) =>
    deps.cancelSearch(event.sender, input),
  );
  ipcHandleMeasured('actions:execute', (_event, action) =>
    deps.executeActionForIpc(action),
  );
  ipcHandleMeasured('view-action:execute', (_event, action) =>
    deps.executeViewActionForIpc(action),
  );
  deps.ipcMain.handle('view:refresh', (event, input) =>
    deps.measureDebugPerformance(
      'ipc.view:refresh.handler',
      { args: [deps.summarizeDebugValue(input)], alwaysLog: true },
      () => deps.refreshViewForIpc(input),
    ),
  );
  ipcHandleMeasured('dialog:pick-form-field-paths', deps.pickFormFieldPaths);
  deps.ipcMain.on('drag:file', deps.startFileDrag);
  ipcHandleMeasured('ai:chat:send', (_event, message, chatId) =>
    deps.sendAiChatMessage(message, chatId),
  );
  ipcHandleMeasured('ai:chat:exited', (_event, chatId) =>
    deps.noteAiChatExited(chatId),
  );
  ipcHandleMeasured('ai:chat:abort', (_event, chatId) =>
    deps.abortAiChat(chatId),
  );
  ipcHandleMeasured('ai:chat:reset', (_event, chatId) =>
    deps.resetAiChat(chatId),
  );
  ipcHandleMeasured('actions:set-alias', (_event, action, alias) =>
    deps.setAlias(action, alias),
  );
  ipcHandleMeasured('actions:remove-alias', (_event, action, alias) =>
    deps.removeAlias(action, alias),
  );
  ipcHandleMeasured('actions:set-shortcut', (_event, action, shortcut) =>
    deps.setShortcut(action, shortcut),
  );
  ipcHandleMeasured('palette:set-hotkey', (_event, accelerator) =>
    deps.setPaletteHotkey(accelerator),
  );
  ipcHandleMeasured('settings:get', (_event, id) => deps.getSetting(id));
  ipcHandleMeasured('system:open-keyboard-settings', () =>
    deps.openSystemKeyboardSettings(),
  );
  ipcHandleMeasured('actions:get-shortcuts', () => deps.getShortcuts());
  ipcHandleMeasured('actions:remove-shortcut', (_event, actionId) =>
    deps.removeShortcut(actionId),
  );
  ipcHandleMeasured('actions:suspend-shortcuts', () => {
    deps.suspendPaletteHotkey();
    return deps.unregisterActionShortcuts();
  });
  ipcHandleMeasured('actions:resume-shortcuts', () => {
    const result = deps.registerActionShortcuts();
    deps.resumePaletteHotkey();
    return result;
  });
  ipcHandleMeasured('actions:set-override', (_event, action, instruction) =>
    deps.setOverride(action, instruction),
  );
  ipcHandleMeasured('actions:clear-override', (_event, action) =>
    deps.clearOverride(action),
  );
  ipcHandleMeasured('actions:duplicate-created', (_event, action) =>
    deps.duplicateCreatedAction(action),
  );
  ipcHandleMeasured('actions:remove-created', (_event, action) =>
    deps.removeCreatedAction(action),
  );
  ipcHandleMeasured(
    'ai-builder:tweak-extension',
    async (_event, input: unknown = {}) => {
      // biome-ignore lint/suspicious/noExplicitAny: legacy IPC payload shape
      const typedInput = input as any;
      const file = typedInput?.extensionFile || typedInput?.extensionId;
      if (!file) {
        return { toast: { message: 'No extension specified', tone: 'error' } };
      }
      const item = deps.getOrCreateExtensionChat(
        file,
        typedInput.title || file,
      );
      return deps.normalizeHostViewResult({
        view: await deps.aiChatView(item, { initialPrompt: typedInput.prompt }),
      });
    },
  );
  ipcHandleMeasured(
    'ai-builder:start-chat',
    async (_event, input: unknown = {}) => {
      // biome-ignore lint/suspicious/noExplicitAny: legacy IPC payload shape
      const typedInput = input as any;
      const item = deps.createDraftAiChat(
        String(typedInput?.prompt || typedInput?.query || ''),
      );
      // biome-ignore lint/suspicious/noExplicitAny: legacy IPC payload shape
      const messages = (item as any).messages;
      return deps.normalizeHostViewResult({
        view: await deps.aiChatView(item, {
          start: messages.length <= 1,
        }),
      });
    },
  );
  ipcHandleMeasured('nevermind:auth-status', async () => {
    const auth = await deps.getNevermindAuth();
    deps.setActiveNevermindBaseUrl(auth?.baseUrl || null);
    if (auth?.baseUrl) {
      deps.warmNevermindCompatibilityCache(auth.baseUrl);
    }
    deps.logInfo(
      'nevermind.auth-status.check',
      { authed: Boolean(auth) },
      { source: 'host', scope: 'nevermind' },
    );
    return auth ? { authed: true, email: auth.email } : { authed: false };
  });
  ipcHandleMeasured('nevermind:debug-status', () =>
    deps.getNevermindDebugStatus(),
  );
  ipcHandleMeasured('nevermind:sign-in', async () => {
    const result = await deps.signInToNevermind();
    if (result.ok) {
      deps.setActiveNevermindBaseUrl(result.auth.baseUrl);
      deps.warmNevermindCompatibilityCache(result.auth.baseUrl);
      deps.invalidateExtensionRootItems();
      deps.broadcastAuthChanged({ authed: true, email: result.auth.email });
      return { ok: true, email: result.auth.email };
    }
    return { ok: false, error: 'Unable to sign in' };
  });
  ipcHandleMeasured('apps:icon', (_event, appPath) =>
    deps.appIconCache.get(appPath),
  );
  ipcHandleMeasured('apps:running-paths', (_event, appPaths) =>
    deps.runningAppStatus.getForRenderer(appPaths),
  );
  ipcHandleMeasured('palette:set-mode', (_event, mode) => {
    deps.paletteWindow.setPaletteSizeForMode(mode);
    deps.paletteWindow.centerWindow();
  });
  ipcHandleMeasured('palette:hide', () => deps.paletteWindow.hidePalette());
  ipcHandleMeasured('app:quit', () => {
    deps.requestQuitApp('ipc');
    return { ok: true };
  });
  ipcHandleMeasured('palette:shortcut-ready', () =>
    deps.paletteWindow.revealPalette(),
  );
  ipcHandleMeasured('camera:request-access', () => {
    if (!deps.hasCapability('camera')) {
      return { ok: false, status: 'unsupported' };
    }
    if (deps.processPlatform !== 'darwin') {
      return { ok: true, status: 'unknown' };
    }
    const status = deps.getCameraMediaAccessStatus();
    if (status === 'granted') {
      return { ok: true, status };
    }
    if (status === 'denied' || status === 'restricted') {
      return { ok: false, status };
    }
    return { ok: true, status };
  });
  ipcHandleMeasured('gh:status', () => deps.probeGh());
  ipcHandleMeasured('extension-window:get-state', (event) =>
    deps.extensionWindowManager.getStateForSender(event.sender),
  );
  ipcHandleMeasured('extension-window:close', (event) =>
    deps.extensionWindowManager.closeForSender(event.sender),
  );
  ipcHandleMeasured('extension-draft:save', (_event, input) =>
    deps.saveExtensionDraft(input),
  );
  ipcHandleMeasured('logs:write', (_event, level, message, data) => {
    let method = deps.logInfo;
    if (level === 'error') {
      method = deps.logError;
    } else if (level === 'warn') {
      method = deps.logWarn;
    } else if (level === 'debug') {
      method = deps.loggerDebug;
    }
    method(String(message || ''), data, {
      source: 'renderer',
      scope: 'renderer',
    });
  });
}
