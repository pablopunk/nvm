import assert from 'node:assert/strict';
import test from 'node:test';
import { diagnosticRecordsView } from './diagnostics';

test('recent errors uses list items and truthful reporting labels', () => {
  const ctx = {
    ui: { list: (view: unknown) => view, item: (item: unknown) => item },
    actions: {
      copyText: (text: string, title: string) => ({
        type: 'copyText',
        text,
        title,
      }),
    },
  };
  const view = diagnosticRecordsView(ctx as never, [
    {
      reference: 'a'.repeat(32),
      bootId: 'b'.repeat(32),
      timestamp: new Date().toISOString(),
      build: 'test',
      operation: 'view.render',
      stage: 'render',
      outcome: 'failed',
      process: 'renderer',
      reporting: 'capture_requested',
    },
  ]) as any;
  assert.equal(view.items[0].primaryAction.title, 'Copy Error Reference');
  assert.match(view.items[0].accessories[0].text, /availability unknown/);
  assert.ok(!JSON.stringify(view).includes('delivered'));
});
