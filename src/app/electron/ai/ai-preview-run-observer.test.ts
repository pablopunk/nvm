import assert from 'node:assert/strict';
import test from 'node:test';
import { createPreviewRunObserver } from './ai-preview-run-observer';

const command = { id: 'summarize', extensionFile: 'summarizer.ts' };

test('reports a failure of the preview run with the underlying error', async () => {
  const observer = createPreviewRunObserver({ waitMs: 20 });
  observer.expectRun('summarizer.ts', 'summarize');

  observer
    .observeRun(command)
    ?.failed(new TypeError('text.trim is not a function'));

  const outcome = await observer.outcomeFor('summarizer.ts');
  assert.equal(outcome.status, 'failed');
  assert.match(
    outcome.status === 'failed' ? outcome.message : '',
    /text\.trim is not a function/,
  );
});

test('reports a clean run only after the preview ran the command', async () => {
  const observer = createPreviewRunObserver({ waitMs: 20 });
  observer.expectRun('summarizer.ts', 'summarize');

  observer.observeRun(command)?.succeeded();

  assert.deepEqual(await observer.outcomeFor('summarizer.ts'), {
    status: 'ran',
  });
});

test('never claims a run when the preview did not execute the command', async () => {
  const observer = createPreviewRunObserver({ waitMs: 10 });
  observer.expectRun('summarizer.ts', 'summarize');

  assert.deepEqual(await observer.outcomeFor('summarizer.ts'), {
    status: 'not-run',
  });
  assert.deepEqual(await observer.outcomeFor('never-written.ts'), {
    status: 'not-run',
  });
});

test('a waiting reader receives a run that finishes while it waits', async () => {
  const observer = createPreviewRunObserver({ waitMs: 1000 });
  observer.expectRun('summarizer.ts', 'summarize');
  const observation = observer.observeRun(command);

  const waiting = observer.outcomeFor('summarizer.ts');
  setTimeout(() => observation?.succeeded(), 5);

  assert.deepEqual(await waiting, { status: 'ran' });
});

test('a result that arrives after the reader gave up is still readable', async () => {
  const observer = createPreviewRunObserver({ waitMs: 5 });
  observer.expectRun('summarizer.ts', 'summarize');
  const observation = observer.observeRun(command);
  assert.equal((await observer.outcomeFor('summarizer.ts')).status, 'not-run');

  observation?.failed(new Error('late failure'));

  assert.equal((await observer.outcomeFor('summarizer.ts')).status, 'failed');
});

test('only the first outcome of the expected command counts', async () => {
  const observer = createPreviewRunObserver({ waitMs: 20 });
  observer.expectRun('summarizer.ts', 'summarize');
  const first = observer.observeRun(command);
  const second = observer.observeRun(command);

  first?.succeeded();
  second?.failed(new Error('duplicate run failed'));

  assert.deepEqual(await observer.outcomeFor('summarizer.ts'), {
    status: 'ran',
  });
  assert.equal(observer.observeRun(command), null);
});

test('ignores other commands, other files, and aborted runs', async () => {
  const observer = createPreviewRunObserver({ waitMs: 10 });
  observer.expectRun('summarizer.ts', 'summarize');

  assert.equal(
    observer.observeRun({ id: 'other', extensionFile: 'summarizer.ts' }),
    null,
  );
  assert.equal(
    observer.observeRun({ id: 'summarize', extensionFile: 'other.ts' }),
    null,
  );
  assert.equal(observer.observeRun({ id: 'summarize' }), null);
  const abort = new Error('aborted');
  abort.name = 'AbortError';
  observer.observeRun(command)?.failed(abort);

  assert.equal((await observer.outcomeFor('summarizer.ts')).status, 'not-run');
});

test('a rewrite discards the previous run and its in-flight observation', async () => {
  const observer = createPreviewRunObserver({ waitMs: 1000 });
  observer.expectRun('summarizer.ts', 'summarize');
  const staleObservation = observer.observeRun(command);
  const staleWaiter = observer.outcomeFor('summarizer.ts');

  observer.expectRun('summarizer.ts', 'summarize');
  staleObservation?.failed(new Error('old code failed'));

  assert.equal((await staleWaiter).status, 'not-run');
  observer.observeRun(command)?.succeeded();
  assert.deepEqual(await observer.outcomeFor('summarizer.ts'), {
    status: 'ran',
  });
});

test('tracks a bounded number of files', async () => {
  const observer = createPreviewRunObserver({ waitMs: 5 });
  for (let index = 0; index < 60; index += 1)
    observer.expectRun(`file-${index}.ts`, 'run');

  assert.equal(
    observer.observeRun({ id: 'run', extensionFile: 'file-0.ts' }),
    null,
  );
  assert.ok(observer.observeRun({ id: 'run', extensionFile: 'file-59.ts' }));
});
