// biome-ignore-all lint: Runner harness stubs dynamic action shapes by design.
import assert from 'node:assert/strict';
import test, { mock } from 'node:test';

mock.module('electron', {
  namedExports: {
    app: { isPackaged: false, getPath: () => '/tmp/nvm-shortcut-runner-test' },
  },
});

mock.module('electron-log/main', {
  defaultExport: { error() {}, info() {}, warn() {}, debug() {} },
});

const { createShortcutRunner } = await import('./shortcut-runner');

function createRunner(overrides: Record<string, any> = {}) {
  const executed: any[] = [];
  const shown: any[] = [];
  const runner = createShortcutRunner({
    captureFrontmostTarget: async () => null,
    paletteWindow: { win: null, showPalette: (options) => shown.push(options) },
    paneInvocations: { register() {}, invalidate() {} },
    paneInvocationWindows: new WeakSet(),
    currentActionForStoredShortcut: (action) => action,
    isClipboardHistoryAction: () => false,
    clipboardHistoryView: () => null,
    recordRecent() {},
    runInBackground: (task) => {
      void task();
    },
    executeAction: async (action) => {
      executed.push(action);
    },
    presentActionErrorFeedback() {},
    normalizeHostViewResult: (result) => result,
    actionFailedFeedbackView: () => null,
    ...overrides,
  });
  return { runner, executed, shown };
}

test('background actions still run when the frontmost target is unavailable', async () => {
  const { runner, executed } = createRunner({
    captureFrontmostTarget: async () => {
      throw new Error('Install xclip to enable selected-text control');
    },
  });

  await runner.executeShortcutAction({ kind: 'test', mode: 'noView' });
  await new Promise((resolve) => setTimeout(resolve, 0));

  assert.equal(executed.length, 1);
});

test('openPalette actions show the palette without executing anything', async () => {
  const { runner, executed, shown } = createRunner();

  await runner.executeShortcutAction({
    kind: 'extension-root-item',
    rootAction: { type: 'openPalette' },
  });

  assert.equal(shown.length, 1);
  assert.equal(executed.length, 0);
});
