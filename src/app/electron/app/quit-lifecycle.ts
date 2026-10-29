// biome-ignore-all lint: Quit cleanup moved verbatim from main.ts keeps its linear teardown sequence.
import { info as logInfo } from '../platform/logger';

export type QuitLifecycleDeps = {
  app: {
    isReady: () => boolean;
    hasSingleInstanceLock: () => boolean;
    releaseSingleInstanceLock: () => void;
  };
  globalShortcut: { unregisterAll: () => void };
  progressiveSearchCoordinator: { dispose: () => void };
  updateManager: { clearTimers: () => void };
  jobRegistry: { clear: () => void };
  extensionWindowManager: { closeAll: () => void };
  appIndexService: { closeWatchers: () => void };
  extensionRuntimeState: {
    extensionFileWatchers: Iterable<{ close: () => unknown }>;
  };
  markAppQuitting: () => void;
  stateSafeQuit: { requestQuit: (reason: string) => Promise<void> };
};

export function createQuitLifecycle(deps: QuitLifecycleDeps) {
  const {
    app,
    globalShortcut,
    progressiveSearchCoordinator,
    updateManager,
    jobRegistry,
    extensionWindowManager,
    appIndexService,
    extensionRuntimeState,
    markAppQuitting,
    stateSafeQuit,
  } = deps;

  let didRunQuitCleanup = false;

  function runQuitCleanup() {
    if (didRunQuitCleanup) return;
    didRunQuitCleanup = true;
    progressiveSearchCoordinator.dispose();
    if (app.isReady()) globalShortcut.unregisterAll();
    updateManager.clearTimers();
    jobRegistry.clear();
    extensionWindowManager.closeAll();
    appIndexService.closeWatchers();
    for (const watcher of extensionRuntimeState.extensionFileWatchers)
      watcher.close();
    // State is flushed by now. Release the lock explicitly so a relaunch, such as
    // after an update, never loses it to a process the OS has not finished tearing down.
    if (app.hasSingleInstanceLock()) app.releaseSingleInstanceLock();
  }

  function requestQuitApp(reason = 'action') {
    markAppQuitting();
    logInfo('app.quit.requested', { reason }, { source: 'host', scope: 'app' });
    return stateSafeQuit.requestQuit(reason);
  }

  return { runQuitCleanup, requestQuitApp };
}
