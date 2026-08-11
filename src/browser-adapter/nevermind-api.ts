import type { NevermindApi } from '../preload-api';

export type BrowserRpcRequest = {
  method: string;
  params: unknown;
};

export type BrowserRpcClient = {
  call<T>(method: string, params?: unknown): Promise<T>;
  close(): void;
};

export type BrowserAdapterOptions = {
  rpcUrl: string;
  eventUrl: string;
  token?: string;
  fetch?: typeof fetch;
  eventSource?: new (url: string) => EventSource;
};

type EventListener = (payload: unknown) => void;
type PromiseResult<T> = T extends (
  ...args: infer _Arguments
) => Promise<infer Result>
  ? Result
  : never;

type EventSourceLike = {
  addEventListener(
    type: string,
    listener: (event: MessageEvent<string>) => void,
  ): void;
  close(): void;
};

type EventSourceConstructor = new (url: string) => EventSourceLike;

function unsupported(method: string): never {
  throw new Error(`${method} is unavailable in the browser.`);
}

function eventPayload(event: MessageEvent<string>): unknown {
  try {
    return JSON.parse(event.data) as unknown;
  } catch {
    return;
  }
}

function createEventSubscriptions(
  eventUrl: string,
  EventSource: EventSourceConstructor,
) {
  const listeners = new Map<string, Set<EventListener>>();
  const source = new EventSource(eventUrl);

  function subscribe(event: string, callback: EventListener) {
    let callbacks = listeners.get(event);
    if (!callbacks) {
      callbacks = new Set();
      listeners.set(event, callbacks);
      source.addEventListener(event, (message) => {
        const payload = eventPayload(message);
        for (const listener of listeners.get(event) || []) listener(payload);
      });
    }
    callbacks.add(callback);
    return () => callbacks?.delete(callback);
  }

  return { close: () => source.close(), subscribe };
}

function joinEventUrl(eventUrl: string, token?: string) {
  if (!token) return eventUrl;
  const url = new URL(
    eventUrl,
    globalThis.location?.href || 'http://localhost',
  );
  url.searchParams.set('token', token);
  return url.toString();
}

export function createBrowserRpcClient({
  rpcUrl,
  token,
  fetch: request = window.fetch.bind(window),
}: Omit<BrowserAdapterOptions, 'eventUrl' | 'eventSource'>): BrowserRpcClient {
  let closed = false;
  return {
    async call<T>(method: string, params?: unknown) {
      if (closed) throw new Error('Browser RPC client is closed.');
      const response = await request(rpcUrl, {
        method: 'POST',
        credentials: 'include',
        headers: {
          'content-type': 'application/json',
          ...(token ? { authorization: `Bearer ${token}` } : {}),
        },
        body: JSON.stringify({ method, params } satisfies BrowserRpcRequest),
      });
      if (!response.ok) throw new Error(await response.text());
      return (await response.json()) as T;
    },
    close() {
      closed = true;
    },
  };
}

export function createBrowserNevermindApi(
  options: BrowserAdapterOptions,
): NevermindApi & { close(): void } {
  const rpc = createBrowserRpcClient(options);
  const events = createEventSubscriptions(
    joinEventUrl(options.eventUrl, options.token),
    (options.eventSource || window.EventSource) as EventSourceConstructor,
  );
  const subscribe = <T>(event: string, callback: (payload: T) => void) =>
    events.subscribe(event, callback as EventListener);
  const invoke = <K extends keyof NevermindApi>(method: K, params?: unknown) =>
    rpc.call<PromiseResult<NevermindApi[K]>>(method, params);

  const api: NevermindApi = {
    getDesignTokens: () => invoke('getDesignTokens'),
    openDesignTokenEditor: () => invoke('getDesignTokens'),
    onOpenDesignTokenEditor: () => () => {},
    setDesignTokens: (overrides) => invoke('setDesignTokens', { overrides }),
    resetDesignTokens: () => invoke('resetDesignTokens'),
    closeDesignTokenEditor: async () => {},
    search: (query, options) => invoke('search', { query, options }),
    cancelSearch: (generation) => void rpc.call('cancelSearch', { generation }),
    onSearchUpdate: (callback) => subscribe('search-update', callback),
    execute: (action) => invoke('execute', { action }),
    runViewAction: (action) => invoke('runViewAction', { action }),
    refreshView: (input) => invoke('refreshView', input),
    pickFormFieldPaths: async () => ({ canceled: true, paths: [] }),
    startFileDrag: () => {},
    sendAiMessage: (message, chatId) =>
      invoke('sendAiMessage', { message, chatId }),
    aiChatExited: (chatId) => invoke('aiChatExited', { chatId }),
    abortAiChat: (chatId) => invoke('abortAiChat', { chatId }),
    resetAiChat: (chatId) => invoke('resetAiChat', { chatId }),
    setAlias: (action, alias) => invoke('setAlias', { action, alias }),
    removeAlias: (action, alias) => invoke('removeAlias', { action, alias }),
    setShortcut: (action, shortcut) =>
      invoke('setShortcut', { action, shortcut }),
    setPaletteHotkey: (accelerator) =>
      invoke('setPaletteHotkey', { accelerator }),
    getSetting: (id) => invoke('getSetting', { id }),
    openSystemKeyboardSettings: async () => ({ ok: false }),
    getShortcuts: () => invoke('getShortcuts'),
    removeShortcut: (actionId) => invoke('removeShortcut', { actionId }),
    suspendShortcuts: () => invoke('suspendShortcuts'),
    resumeShortcuts: () => invoke('resumeShortcuts'),
    setOverride: (action, instruction) =>
      invoke('setOverride', { action, instruction }),
    clearOverride: (action) => invoke('clearOverride', { action }),
    duplicateCreatedAction: (action) =>
      invoke('duplicateCreatedAction', { action }),
    removeCreatedAction: (action) => invoke('removeCreatedAction', { action }),
    tweakExtension: (input) => invoke('tweakExtension', input),
    startBuilderChat: (input) => invoke('startBuilderChat', input),
    getAppIcon: async () => null,
    getRunningAppPaths: async () => [],
    setPaletteMode: async () => {},
    hide: async () => {},
    testInvoke: () => unsupported('testInvoke'),
    testStageExtensionProposal: () => unsupported('testStageExtensionProposal'),
    testRunJob: () => unsupported('testRunJob'),
    testFailNextExtensionActivation: () =>
      unsupported('testFailNextExtensionActivation'),
    testIsActionShortcutRegistered: async () => ({ registered: false }),
    shortcutReady: async () => {},
    requestCameraAccess: async () => ({ ok: false, status: 'unavailable' }),
    log: (level, message, data) => invoke('log', { level, message, data }),
    getNevermindAuthStatus: () => invoke('getNevermindAuthStatus'),
    getNevermindDebugStatus: () => invoke('getNevermindDebugStatus'),
    getGhStatus: () => invoke('getGhStatus'),
    signInToNevermind: () => invoke('signInToNevermind'),
    onNevermindAuthChanged: (callback) =>
      subscribe('nevermind-auth-changed', callback),
    onShown: () => () => {},
    onShortcutShown: () => () => {},
    onHidden: () => () => {},
    onAppsIndexed: (callback) => subscribe('apps-indexed', callback),
    onRunningAppPathsChanged: (callback) =>
      subscribe('running-app-paths-changed', callback),
    onClipboardChanged: (callback) => subscribe('clipboard-changed', callback),
    onRootItemsChanged: (callback) => subscribe('root-items-changed', callback),
    onOpenActionView: (callback) => subscribe('open-action-view', callback),
    onAiChatEvent: (callback) => subscribe('ai-chat-event', callback),
    getExtensionWindowState: async () => undefined,
    closeExtensionWindow: async () => {},
    saveExtensionDraft: () => unsupported('saveExtensionDraft'),
    onExtensionWindowView: (callback) =>
      subscribe('extension-window-view', callback),
    onViewPatch: (callback) => subscribe('view-patch', callback),
    onViewHydrate: (callback) => subscribe('view-hydrate', callback),
    retryViewLoader: (viewId) => invoke('retryViewLoader', { viewId }),
  };

  return {
    ...api,
    close() {
      rpc.close();
      events.close();
    },
  };
}
