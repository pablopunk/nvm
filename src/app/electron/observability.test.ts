import assert from 'node:assert/strict';
import test from 'node:test';
import {
  configureObservability,
  currentOperation,
  operationOutcome,
  recordOperationFailure,
  runOperation,
} from './observability';

test('operation context survives awaits and isolates concurrent windows without tracing', async () => {
  configureObservability({ build: 'test' });
  const contexts = await Promise.all(
    [1, 2].map((windowId) =>
      runOperation(
        'command.execute',
        async () => {
          const context = currentOperation()!;
          await Promise.resolve();
          assert.equal(currentOperation(), context);
          return context;
        },
        { windowId },
      ),
    ),
  );
  assert.notEqual(contexts[0]?.journeyId, contexts[1]?.journeyId);
  assert.equal(contexts[0]?.windowId, 1);
  assert.equal(contexts[1]?.windowId, 2);
  assert.equal(currentOperation(), undefined);
});

test('caught and rethrown failures have one owner and do not become success', async () => {
  const captures: unknown[] = [];
  configureObservability({
    build: 'test',
    capture: (error) => {
      captures.push(error);
    },
  });
  let outcome;
  await runOperation('command.execute', async () => {
    try {
      await runOperation('view.action', async () => {
        throw new Error('private message');
      });
    } catch (error) {
      recordOperationFailure(error);
    }
    outcome = currentOperation()?.outcome;
  });
  assert.equal(outcome, 'failed');
  assert.equal(captures.length, 1);
  await runOperation('command.execute', () => {
    recordOperationFailure(new Error('private message'));
  });
  assert.equal(captures.length, 2);
});

test('cancellation closes once without an error capture', () => {
  const captures: unknown[] = [];
  configureObservability({
    build: 'test',
    capture: (error) => {
      captures.push(error);
    },
  });
  const context = runOperation('ai.stream', () => {
    recordOperationFailure(
      Object.assign(new Error('cancelled'), { name: 'AbortError' }),
    );
    return currentOperation()!;
  });
  assert.equal(context.outcome, 'cancelled');
  assert.equal(captures.length, 0);
  operationOutcome('success');
  assert.equal(context.outcome, 'cancelled');
});
