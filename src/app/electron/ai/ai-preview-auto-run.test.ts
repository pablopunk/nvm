import assert from 'node:assert/strict';
import test from 'node:test';
import { createPreviewAutoRunner } from './ai-preview-auto-run';

const commandPreview = (commands: unknown[]) => ({
  preview: { extensionId: 'dev.summarizer', rootItems: [], commands },
});

test('an extension without commands is not auto-run', async () => {
  let executions = 0;
  const run = createPreviewAutoRunner({
    executeAction: () => {
      executions += 1;
      return Promise.resolve({});
    },
  });

  assert.equal(await run(commandPreview([])), undefined);
  assert.equal(executions, 0);
});

test('only the first command runs and its result is handed back for the preview', async () => {
  const executed: unknown[] = [];
  const run = createPreviewAutoRunner({
    executeAction: (action) => {
      executed.push(action);
      return Promise.resolve({ view: { id: 'summary' } });
    },
  });

  const outcome = await run(
    commandPreview([{ id: 'first' }, { id: 'second' }]),
  );

  assert.deepEqual(executed, [{ id: 'first' }]);
  assert.deepEqual(outcome, { result: { view: { id: 'summary' } } });
});

test('a command failure is surfaced with its stack even though the host returns an error view', async () => {
  const failure = new TypeError('text.trim is not a function');
  const run = createPreviewAutoRunner({
    executeAction: (_action, { onFailure }) => {
      onFailure(failure);
      return Promise.resolve({ view: { id: 'extension-action-error' } });
    },
  });

  const outcome = await run(commandPreview([{ id: 'summarize' }]));

  assert.match(outcome?.failureMessage ?? '', /text\.trim is not a function/);
  assert.deepEqual(outcome?.result, { view: { id: 'extension-action-error' } });
});

test('non-Error failures are stringified', async () => {
  const run = createPreviewAutoRunner({
    executeAction: (_action, { onFailure }) => {
      onFailure('boom');
      return Promise.resolve({});
    },
  });

  const outcome = await run(commandPreview([{ id: 'summarize' }]));

  assert.equal(outcome?.failureMessage, 'boom');
});

test('a command that never settles times out without claiming failure', async () => {
  const run = createPreviewAutoRunner({
    executeAction: () => new Promise(() => {}),
    timeoutMs: 5,
  });

  const outcome = await run(commandPreview([{ id: 'hang' }]));

  assert.deepEqual(outcome, { result: {}, timedOut: true });
});
