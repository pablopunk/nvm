// biome-ignore-all lint: Test-mode IPC handlers are dynamic renderer payload shims moved verbatim from main.ts.
import { app } from 'electron';
import { piRuntimeModuleResolution } from '../ai/pi-runtime';
import {
  createPackagedPiSessionFixture,
  type PackagedPiSessionAi,
} from '../ai/packaged-pi-session-fixture';
import type { ExtensionRuntimeState } from '../extension-host/extension-runtime-state';
import type { JobRegistry } from '../platform/jobs';
import { isNvmPackagedPiTestMode } from '../platform/test-mode-flags';
import { normalizeAccelerator } from '../shortcuts/shortcut-utils';
import { testModePaletteActionIsSafe } from './test-mode-safety';

type MeasuredIpcHandle = (
  channel: string,
  handler: (event: any, ...args: any[]) => unknown,
) => void;

export type TestModeIpcDeps = {
  ipcMain: {
    on: (
      channel: string,
      listener: (event: any, ...args: any[]) => void,
    ) => unknown;
  };
  createMeasuredIpcHandle: () => MeasuredIpcHandle;
  globalShortcut: { isRegistered: (accelerator: string) => boolean };
  paletteWindow: {
    hidePalette: () => unknown;
    revealPalette: () => unknown;
  };
  extensionWindowManager: {
    getStateForSender: (sender: unknown) => unknown;
    closeForSender: (sender: unknown) => unknown;
  };
  jobRegistry: Pick<JobRegistry, 'has' | 'run'>;
  extensionRuntimeState: Pick<
    ExtensionRuntimeState,
    'testExtensionActivationFailurePhase'
  >;
  isActionShortcutOwned: (accelerator: string) => boolean;
  startProgressiveSearch: (sender: any, input: any) => unknown;
  cancelProgressiveSearch: (sender: any, input: any) => void;
  searchActions: (query: any, options?: any) => Promise<any[]>;
  executeActionForIpc: (action: any) => Promise<any>;
  executeViewActionForIpc: (
    action: any,
    context: any,
    senderId: number,
  ) => unknown;
  stageExtensionProposal: (filename: any, source: any) => unknown;
  setShortcut: (action: any, shortcut: any) => unknown;
  getSetting: (id: any) => unknown;
  getShortcuts: () => unknown;
  showRendererIndicator: (sender: any, input: any) => unknown;
  initializeAi: () => Promise<unknown>;
  getAi: () => PackagedPiSessionAi | null;
};

export function createTestModeIpc(deps: TestModeIpcDeps) {
  const {
    ipcMain,
    createMeasuredIpcHandle,
    globalShortcut,
    paletteWindow,
    extensionWindowManager,
    jobRegistry,
    extensionRuntimeState,
    isActionShortcutOwned,
    startProgressiveSearch,
    cancelProgressiveSearch,
    searchActions,
    executeActionForIpc,
    executeViewActionForIpc,
    stageExtensionProposal,
    setShortcut,
    getSetting,
    getShortcuts,
    showRendererIndicator,
    initializeAi,
    getAi,
  } = deps;
  const runPiPackagedSession = createPackagedPiSessionFixture({
    enabled: isNvmPackagedPiTestMode,
    isPackaged: () => app.isPackaged,
    appPath: () => app.getAppPath(),
    initializeAi,
    getAi,
    resolveModules: piRuntimeModuleResolution,
  });

  function registerTestModeIpcHandlers() {
    const handle = createMeasuredIpcHandle();
    handle('actions:search', (event, input) =>
      startProgressiveSearch(event.sender, input),
    );
    ipcMain.on('actions:search:cancel', (event, input) =>
      cancelProgressiveSearch(event.sender, input),
    );
    handle('actions:execute', (_event, action) => {
      if (!testModePaletteActionIsSafe(action))
        return {
          toast: { message: 'Production actions are disabled in test mode' },
        };
      return executeActionForIpc(action);
    });
    handle('test:invoke', async () => {
      const actions = await searchActions('Test: Confirm safe action');
      void executeActionForIpc(actions[0]).catch((error) =>
        console.error('test action failed', error),
      );
      return { found: actions.length > 0 };
    });
    handle('test:stage-extension-proposal', (_event, filename, source) =>
      stageExtensionProposal(filename, source),
    );
    handle('test:run-job', async (_event, id) => {
      if (!jobRegistry.has(id)) return { found: false };
      await jobRegistry.run(id, 'electron-smoke');
      return { found: true };
    });
    if (isNvmPackagedPiTestMode)
      handle('test:run-pi-session', () => runPiPackagedSession());
    handle('test:fail-next-extension-activation', (_event, phase) => {
      extensionRuntimeState.testExtensionActivationFailurePhase = String(
        phase || 'after-persist',
      );
      return { armed: true };
    });
    handle('test:is-action-shortcut-registered', (_event, accelerator) => {
      const normalized = normalizeAccelerator(String(accelerator || ''));
      return {
        registered:
          isActionShortcutOwned(normalized) &&
          globalShortcut.isRegistered(normalized),
      };
    });
    handle('view-action:execute', (event, action, context) =>
      executeViewActionForIpc(action, context, event.sender.id),
    );
    handle('extension-window:get-state', (event) =>
      extensionWindowManager.getStateForSender(event.sender),
    );
    handle('extension-window:close', (event) =>
      extensionWindowManager.closeForSender(event.sender),
    );
    handle('actions:set-shortcut', (_event, action, shortcut) =>
      setShortcut(action, shortcut),
    );
    handle('nevermind:auth-status', () => ({ authed: false }));
    handle('gh:status', () => ({ installed: false, authed: false }));
    handle('settings:get', (_event, id) => getSetting(id));
    handle('palette:set-mode', () => undefined);
    handle('palette:hide', () => paletteWindow.hidePalette());
    handle('indicator:show', (event, input) =>
      showRendererIndicator(event.sender, input),
    );
    handle('palette:shortcut-ready', () => paletteWindow.revealPalette());
    handle('actions:suspend-shortcuts', () => undefined);
    handle('actions:resume-shortcuts', () => undefined);
    handle('actions:get-shortcuts', () => getShortcuts());
    handle('logs:write', () => undefined);
  }

  return { registerTestModeIpcHandlers };
}
