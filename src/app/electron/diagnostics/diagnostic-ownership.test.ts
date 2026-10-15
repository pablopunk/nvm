import assert from 'node:assert/strict';
import test from 'node:test';
import {
  configureObservability,
  currentOperation,
  operationPending,
  recordOperationFailure,
  recordServerOutcome,
  runOperation,
} from './observability';

test('an optional backend failure does not own an unrelated client exception', () => {
  const captures: unknown[] = [];
  configureObservability({
    build: 'test',
    capture: (error) => {
      captures.push(error);
    },
  });
  runOperation('command.execute', () => {
    recordServerOutcome(404, 'optional-request');
    recordOperationFailure(new Error('extension failed'));
  });
  assert.equal(captures.length, 1);
});

test('fresh roots do not inherit a previous job or closed action outcome', () => {
  configureObservability({ build: 'test' });
  runOperation('command.execute', () => {
    const outer = currentOperation()!;
    const job = runOperation(
      'job.run',
      () => {
        recordOperationFailure(new Error('job failed'));
        return currentOperation()!;
      },
      { freshJourney: true },
    );
    assert.notEqual(job.journeyId, outer.journeyId);
    assert.equal(outer.failure, undefined);
    assert.equal(outer.outcome, 'unknown');
  });
});

test('declared error feedback is a failed local outcome without an exception event', () => {
  const captures: unknown[] = [];
  configureObservability({
    build: 'test',
    capture: (error) => {
      captures.push(error);
    },
  });
  const context = runOperation('command.execute', () => {
    recordOperationFailure(undefined, 'invoke', 'local');
    return currentOperation()!;
  });
  assert.equal(context.outcome, 'failed');
  assert.equal(context.failure?.reporting, 'local_only');
  assert.equal(captures.length, 0);
});

test('queued acceptance does not prove completion', () => {
  configureObservability({ build: 'test' });
  const context = runOperation('command.execute', () => {
    operationPending();
    return currentOperation()!;
  });
  assert.equal(context.outcome, 'unknown');
});
