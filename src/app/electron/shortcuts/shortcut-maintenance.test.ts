import assert from 'node:assert/strict';
import test, { mock } from 'node:test';
import { createMaintenanceActivity } from '../platform/maintenance-activity';

mock.module('../diagnostics/debug-performance', {
  namedExports: { performanceTraces: { event: () => {} } },
});
mock.module('../diagnostics/observability', {
  namedExports: {
    runOperation: (_name: string, task: () => unknown) => task(),
  },
});
mock.module('../platform/logger', { namedExports: { error: () => {} } });
const { createShortcutRunner } = await import('./shortcut-runner');

test('maintenance waits for shortcut work and reports a blocked shortcut', async () => {
  const activity = createMaintenanceActivity();
  let finish!: () => void;
  const target = new Promise<null>((resolve) => {
    finish = () => resolve(null);
  });
  const errors: unknown[] = [];
  const runner = createShortcutRunner({
    activity,
    captureFrontmostTarget: () => target,
    paletteWindow: { win: null, showPalette: () => {} },
    paneInvocations: { register: () => {}, invalidate: () => {} },
    paneInvocationWindows: new WeakSet(),
    currentActionForStoredShortcut: (action) => action,
    isClipboardHistoryAction: () => false,
    clipboardHistoryView: () => null,
    recordRecent: () => {},
    runInBackground: () => {},
    executeAction: async () => {},
    presentActionErrorFeedback: (error) => errors.push(error),
    normalizeHostViewResult: (result) => result,
    actionFailedFeedbackView: () => ({}),
  });
  const first = runner.executeShortcutAction({ background: true });
  let drained = false;
  const maintenance = activity.quiesce().then(() => {
    drained = true;
  });
  await runner.executeShortcutAction({ background: true });
  assert.equal(errors.length, 1);
  assert.match(String(errors[0]), /state backup/);
  await new Promise<void>((resolve) => setImmediate(resolve));
  assert.equal(drained, false);
  finish();
  await Promise.all([first, maintenance]);
  activity.resume();
});
