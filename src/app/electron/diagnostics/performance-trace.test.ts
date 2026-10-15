import assert from 'node:assert/strict';
import test from 'node:test';
import { createPerformanceTraceService } from './performance-trace';

test('performance traces correlate nested work and redact unknown attributes', async () => {
  const entries: Array<Record<string, unknown>> = [];
  const traces = createPerformanceTraceService({
    log: (entry) => entries.push(entry as unknown as Record<string, unknown>),
  });
  const root = traces.start('action.root', {
    actionType: 'run',
    secret: 'nope',
  });
  await traces.run(
    'action.handler',
    { extensionId: 'example', messageLength: 3 },
    async () => undefined,
    { traceId: root.traceId, parentSpanId: root.spanId },
  );
  traces.finish(root);

  assert.equal(entries.length, 2);
  assert.equal(entries[0].traceId, entries[1].traceId);
  assert.equal(entries[0].parentSpanId, root.spanId);
  assert.equal(
    (entries[0].attributes as Record<string, unknown>).secret,
    undefined,
  );
  assert.equal(typeof entries[0].durationMs, 'number');
});

test('performance traces inherit ambient parent spans', async () => {
  const entries: Array<Record<string, unknown>> = [];
  const traces = createPerformanceTraceService({
    log: (entry) => entries.push(entry as unknown as Record<string, unknown>),
  });

  await traces.run('action.root', {}, async () => {
    await traces.run('action.nested', {}, async () => undefined);
  });

  assert.equal(entries.length, 2);
  assert.equal(entries[0].parentSpanId, entries[1].spanId);
});

test('performance trace event records errors without throwing', () => {
  const entries: Array<Record<string, unknown>> = [];
  const traces = createPerformanceTraceService({
    log: (entry) => entries.push(entry as unknown as Record<string, unknown>),
  });
  traces.event('os.dispatch', { status: 'error' });
  assert.equal(entries[0].status, 'error');
});

test('performance trace records preserve explicit durations', () => {
  const entries: Array<Record<string, unknown>> = [];
  const traces = createPerformanceTraceService({
    log: (entry) => entries.push(entry as unknown as Record<string, unknown>),
  });

  traces.record({
    traceId: 'trace-1',
    operation: 'os.dispatch',
    durationMs: 42,
    queueMs: 3,
  });

  assert.equal(entries[0].durationMs, 42);
  assert.equal((entries[0].attributes as Record<string, unknown>).queueMs, 3);
});

test('performance trace hashes identifier attributes', () => {
  const entries: Array<Record<string, unknown>> = [];
  const traces = createPerformanceTraceService({
    log: (entry) => entries.push(entry as unknown as Record<string, unknown>),
  });

  traces.event('extension.action', {
    extensionId: '/Users/pablo/secret-extension.ts',
    commandId: 'private-command',
  });

  const attributes = entries[0].attributes as Record<string, unknown>;
  assert.notEqual(attributes.extensionId, '/Users/pablo/secret-extension.ts');
  assert.notEqual(attributes.commandId, 'private-command');
  assert.equal(typeof attributes.extensionId, 'string');
});

test('performance trace ignores malformed events', () => {
  const entries: Array<Record<string, unknown>> = [];
  const traces = createPerformanceTraceService({
    log: (entry) => entries.push(entry as unknown as Record<string, unknown>),
  });

  traces.record(null);
  traces.record({ traceId: 'trace-1', operation: 'bad' });

  assert.equal(entries.length, 0);
});
