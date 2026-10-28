import assert from 'node:assert/strict';
import { test } from 'node:test';
import { semanticFileSearch } from './files-semantic-search';

test('keeps local file results and adds AI matches using only file names', async () => {
  const recent = [
    { id: 'a', name: 'beach.jpg', path: '/private/beach.jpg' },
    { id: 'b', name: 'report.pdf', path: '/private/report.pdf' },
  ];
  let prompt: any;
  let decisionInput: any;
  const ctx: any = {
    input: {
      prompt(value: any) {
        prompt = value;
        return value;
      },
    },
    actions: {
      run(_title: string, handler: any) {
        return { handler };
      },
    },
    desktop: {
      files: { recent: () => recent, searchIndex: () => [recent[1]] },
    },
    ui: {
      list(value: any) {
        return value;
      },
    },
    ai: {
      async decide(value: any) {
        decisionInput = value;
        return {
          answers: {
            file_0: { type: 'predicate', probability: 0.9 },
            file_1: { type: 'predicate', probability: 0.1 },
          },
        };
      },
    },
  };
  semanticFileSearch(ctx, (file) => ({ id: file.id, title: file.name }));
  const view = prompt.action.handler(ctx, {
    formValues: { query: 'holiday photos' },
  });
  assert.deepEqual(
    view.items.map((item: any) => item.id),
    ['find-with-ai', 'b'],
  );
  const patch = await view.items[0].primaryAction.handler(ctx);
  assert.deepEqual(
    patch.patch.items.map((item: any) => item.id),
    ['b', 'a'],
  );
  assert.deepEqual(decisionInput.input.files, [
    { id: 'file_0', name: 'beach.jpg' },
    { id: 'file_1', name: 'report.pdf' },
  ]);
});

test('does not offer an AI call when there are no candidates', () => {
  let prompt: any;
  const ctx: any = {
    input: {
      prompt(value: any) {
        prompt = value;
        return value;
      },
    },
    actions: {
      run(_title: string, handler: any) {
        return { handler };
      },
    },
    desktop: { files: { recent: () => [], searchIndex: () => [] } },
    ui: {
      list(value: any) {
        return value;
      },
    },
  };
  semanticFileSearch(ctx, (file) => ({ id: file.id, title: file.name }));
  assert.deepEqual(
    prompt.action.handler(ctx, { formValues: { query: 'nothing' } }).items,
    [],
  );
});
