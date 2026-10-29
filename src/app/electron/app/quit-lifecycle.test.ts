import assert from 'node:assert/strict';
import test from 'node:test';
import { createQuitLifecycle } from './quit-lifecycle';

function lifecycle(held: boolean) {
  const calls: string[] = [];
  const noop = () => {};
  const quit = createQuitLifecycle({
    app: {
      isReady: () => true,
      hasSingleInstanceLock: () => held,
      releaseSingleInstanceLock: () => calls.push('release'),
    },
    globalShortcut: { unregisterAll: () => calls.push('shortcuts') },
    progressiveSearchCoordinator: { dispose: noop },
    updateManager: { clearTimers: noop },
    jobRegistry: { clear: noop },
    extensionWindowManager: { closeAll: () => calls.push('windows') },
    appIndexService: { closeWatchers: noop },
    extensionRuntimeState: { extensionFileWatchers: [] },
    markAppQuitting: noop,
    stateSafeQuit: { requestQuit: async () => {} },
  });
  return { calls, quit };
}

test('quit cleanup releases the single-instance lock last, once', () => {
  const { calls, quit } = lifecycle(true);
  quit.runQuitCleanup();
  quit.runQuitCleanup();
  assert.deepEqual(calls, ['shortcuts', 'windows', 'release']);
});

test('quit cleanup does not release a lock the process never held', () => {
  const { calls, quit } = lifecycle(false);
  quit.runQuitCleanup();
  assert.equal(calls.includes('release'), false);
});
