// biome-ignore-all lint: Preload instrumentation intentionally uses Electron environment flags and best-effort Performance APIs.
import { contextBridge, type IpcRendererEvent, ipcRenderer } from 'electron';
import type { NevermindApi } from '../preload-api';

const DEBUG_PERFORMANCE_SLOW_MS = 8;

function debugPerformanceEnabled() {
  if (
    process.env.NVM_DEBUG_PERFORMANCE === '0' ||
    process.env.NVM_DEBUG_PERFORMANCE === 'false'
  )
    return false;
  return Boolean(
    process.env.NVM_DEBUG_PERFORMANCE || process.env.ELECTRON_RENDERER_URL,
  );
}

async function invokeMeasured<T>(
  channel: string,
  ...args: unknown[]
): Promise<T> {
  if (!debugPerformanceEnabled()) return ipcRenderer.invoke(channel, ...args);
  const startedAt = performance.now();
  const markId = `nvm:ipc.${channel}:${startedAt}`;
  try {
    performance.mark(`${markId}:start`);
  } catch {}
  try {
    return await ipcRenderer.invoke(channel, ...args);
  } finally {
    const durationMs = performance.now() - startedAt;
    try {
      performance.mark(`${markId}:end`);
      performance.measure(
        `nvm:ipc.${channel}`,
        `${markId}:start`,
        `${markId}:end`,
      );
      performance.clearMarks(`${markId}:start`);
      performance.clearMarks(`${markId}:end`);
    } catch {}
    if (channel !== 'logs:write' && durationMs >= DEBUG_PERFORMANCE_SLOW_MS) {
      ipcRenderer
        .invoke('logs:write', 'debug', 'performance.measure', {
          name: `ipc.${channel}`,
          durationMs: Math.round(durationMs * 100) / 100,
        })
        .catch(() => {});
    }
  }
}

const api: NevermindApi = {
  getDesignTokens: () => invokeMeasured('design-tokens:get'),
  openDesignTokenEditor: () => invokeMeasured('design-tokens:open'),
  onOpenDesignTokenEditor: (callback) => {
    const listener = (_event: IpcRendererEvent, state) => callback(state);
    ipcRenderer.on('design-tokens:open-editor', listener);
    return () =>
      ipcRenderer.removeListener('design-tokens:open-editor', listener);
  },
  setDesignTokens: (overrides) =>
    invokeMeasured('design-tokens:set', overrides),
  resetDesignTokens: () => invokeMeasured('design-tokens:reset'),
  closeDesignTokenEditor: () => invokeMeasured('design-tokens:close'),
  search: (query, options) =>
    invokeMeasured('actions:search', { query, ...options }),
  cancelSearch: (generation) =>
    ipcRenderer.send('actions:search:cancel', { generation }),
  onSearchUpdate: (callback) => {
    const listener = (
      _event: IpcRendererEvent,
      snapshot: Parameters<NevermindApi['onSearchUpdate']>[0] extends (
        snapshot: infer Snapshot,
      ) => void
        ? Snapshot
        : never,
    ) => callback(snapshot);
    ipcRenderer.on('actions:search:update', listener);
    return () => ipcRenderer.removeListener('actions:search:update', listener);
  },
  execute: (action) => invokeMeasured('actions:execute', action),
  runViewAction: (action) => invokeMeasured('view-action:execute', action),
  refreshView: (input) => invokeMeasured('view:refresh', input),
  pickFormFieldPaths: (input) =>
    invokeMeasured('dialog:pick-form-field-paths', input),
  startFileDrag: (filePath) => ipcRenderer.send('drag:file', filePath),
  sendAiMessage: (message, chatId) =>
    invokeMeasured('ai:chat:send', message, chatId),
  aiChatExited: (chatId) => invokeMeasured('ai:chat:exited', chatId),
  abortAiChat: (chatId) => invokeMeasured('ai:chat:abort', chatId),
  resetAiChat: (chatId) => invokeMeasured('ai:chat:reset', chatId),
  setAlias: (action, alias) =>
    invokeMeasured('actions:set-alias', action, alias),
  removeAlias: (action, alias) =>
    invokeMeasured('actions:remove-alias', action, alias),
  setShortcut: (action, shortcut) =>
    invokeMeasured('actions:set-shortcut', action, shortcut),
  setPaletteHotkey: (accelerator) =>
    invokeMeasured('palette:set-hotkey', accelerator),
  getSetting: (id) => invokeMeasured('settings:get', id),
  openSystemKeyboardSettings: () =>
    invokeMeasured('system:open-keyboard-settings'),
  getShortcuts: () => invokeMeasured('actions:get-shortcuts'),
  removeShortcut: (actionId) =>
    invokeMeasured('actions:remove-shortcut', actionId),
  suspendShortcuts: () => invokeMeasured('actions:suspend-shortcuts'),
  resumeShortcuts: () => invokeMeasured('actions:resume-shortcuts'),
  setOverride: (action, instruction) =>
    invokeMeasured('actions:set-override', action, instruction),
  clearOverride: (action) => invokeMeasured('actions:clear-override', action),
  duplicateCreatedAction: (action) =>
    invokeMeasured('actions:duplicate-created', action),
  removeCreatedAction: (action) =>
    invokeMeasured('actions:remove-created', action),
  tweakExtension: (input) =>
    invokeMeasured('ai-builder:tweak-extension', input),
  startBuilderChat: (input) => invokeMeasured('ai-builder:start-chat', input),
  getAppIcon: (appPath) => invokeMeasured('apps:icon', appPath),
  getRunningAppPaths: (appPaths) =>
    invokeMeasured('apps:running-paths', appPaths),
  setPaletteMode: (mode) => invokeMeasured('palette:set-mode', mode),
  hide: () => invokeMeasured('palette:hide'),
  testInvoke: () => invokeMeasured('test:invoke'),
  testStageExtensionProposal: (filename, source) =>
    invokeMeasured('test:stage-extension-proposal', filename, source),
  testRunJob: (id) => invokeMeasured('test:run-job', id),
  testFailNextExtensionActivation: (phase) =>
    invokeMeasured('test:fail-next-extension-activation', phase),
  testIsActionShortcutRegistered: (accelerator) =>
    invokeMeasured('test:is-action-shortcut-registered', accelerator),
  quitApp: () => invokeMeasured('app:quit'),
  shortcutReady: () => invokeMeasured('palette:shortcut-ready'),
  requestCameraAccess: () => invokeMeasured('camera:request-access'),
  getGhStatus: () => invokeMeasured('gh:status'),
  getExtensionWindowState: () => invokeMeasured('extension-window:get-state'),
  closeExtensionWindow: () => invokeMeasured('extension-window:close'),
  saveExtensionDraft: (ref, content) =>
    invokeMeasured('extension-draft:save', { ref, content }),
  log: (level, message, data) =>
    invokeMeasured('logs:write', level, message, data),
  getNevermindAuthStatus: () => invokeMeasured('nevermind:auth-status'),
  getNevermindDebugStatus: () => invokeMeasured('nevermind:debug-status'),
  signInToNevermind: () => invokeMeasured('nevermind:sign-in'),
  onNevermindAuthChanged: (callback) => {
    const listener = (
      _event: IpcRendererEvent,
      status: { authed: boolean; email?: string },
    ) => callback(status);
    ipcRenderer.on('nevermind:auth-changed', listener);
    return () => ipcRenderer.removeListener('nevermind:auth-changed', listener);
  },
  onShown: (callback) => {
    const listener = () => callback();
    ipcRenderer.on('palette:shown', listener);
    return () => ipcRenderer.removeListener('palette:shown', listener);
  },
  onShortcutShown: (callback) => {
    const listener = () => callback();
    ipcRenderer.on('palette:shortcut-show', listener);
    return () => ipcRenderer.removeListener('palette:shortcut-show', listener);
  },
  onHidden: (callback) => {
    const listener = () => callback();
    ipcRenderer.on('palette:hidden', listener);
    return () => ipcRenderer.removeListener('palette:hidden', listener);
  },
  onAppsIndexed: (callback) => {
    const listener = (_event: IpcRendererEvent, count: number) =>
      callback(count);
    ipcRenderer.on('apps:indexed', listener);
    return () => ipcRenderer.removeListener('apps:indexed', listener);
  },
  onRunningAppPathsChanged: (callback) => {
    const listener = () => callback();
    ipcRenderer.on('apps:running-paths-changed', listener);
    return () =>
      ipcRenderer.removeListener('apps:running-paths-changed', listener);
  },
  onClipboardChanged: (callback) => {
    const listener = () => callback();
    ipcRenderer.on('clipboard:changed', listener);
    return () => ipcRenderer.removeListener('clipboard:changed', listener);
  },
  onRootItemsChanged: (callback) => {
    const listener = () => callback();
    ipcRenderer.on('root-items:changed', listener);
    return () => ipcRenderer.removeListener('root-items:changed', listener);
  },
  onOpenActionView: (callback) => {
    const listener = (
      _event: IpcRendererEvent,
      payload?: Parameters<NevermindApi['onOpenActionView']>[0] extends (
        payload?: infer Payload,
      ) => void
        ? Payload
        : never,
    ) => callback(payload);
    ipcRenderer.on('action:view-open', listener);
    return () => ipcRenderer.removeListener('action:view-open', listener);
  },
  onAiChatEvent: (callback) => {
    const listener = (
      _event: IpcRendererEvent,
      payload: Parameters<NevermindApi['onAiChatEvent']>[0] extends (
        event: infer Event,
      ) => void
        ? Event
        : never,
    ) => callback(payload);
    ipcRenderer.on('ai:chat:event', listener);
    return () => ipcRenderer.removeListener('ai:chat:event', listener);
  },
  onExtensionWindowView: (callback) => {
    const listener = (
      _event: IpcRendererEvent,
      payload: Parameters<NevermindApi['onExtensionWindowView']>[0] extends (
        payload: infer Payload,
      ) => void
        ? Payload
        : never,
    ) => callback(payload);
    ipcRenderer.on('extension-window:view', listener);
    return () => ipcRenderer.removeListener('extension-window:view', listener);
  },
  onViewPatch: (callback) => {
    const listener = (
      _event: IpcRendererEvent,
      payload: Parameters<NevermindApi['onViewPatch']>[0] extends (
        payload: infer Payload,
      ) => void
        ? Payload
        : never,
    ) => callback(payload);
    ipcRenderer.on('view:patch', listener);
    return () => ipcRenderer.removeListener('view:patch', listener);
  },
  onViewHydrate: (callback) => {
    const listener = (
      _event: IpcRendererEvent,
      payload: Parameters<NevermindApi['onViewHydrate']>[0] extends (
        payload: infer Payload,
      ) => void
        ? Payload
        : never,
    ) => callback(payload);
    ipcRenderer.on('view:hydrate', listener);
    return () => ipcRenderer.removeListener('view:hydrate', listener);
  },
  retryViewLoader: (viewId) => invokeMeasured('view:hydrate:retry', viewId),
};

contextBridge.exposeInMainWorld('nvm', api);
