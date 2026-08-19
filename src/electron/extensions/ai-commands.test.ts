import assert from 'node:assert/strict';
import test from 'node:test';
import { extensionActionContributionIsDiscoverable } from '../search-snapshot';
import { createAiCommandsExtension } from './ai-commands';

function commandHandler(context: any) {
  const extension = createAiCommandsExtension();
  const contribution = extension.actions({
    ...context,
    action: (input: unknown) => input,
  })[0];
  assert.equal(contribution.title, 'Fix Selected Text with AI');
  assert.equal(extensionActionContributionIsDiscoverable(contribution), true);
  return contribution.run;
}

function contextFor(selectedText: string) {
  const preparations: unknown[] = [];
  const aiCalls: unknown[] = [];
  const actions: unknown[] = [];
  const context = {
    ai: {
      prepare: async (options: unknown) => preparations.push(options),
      ask: async (...input: unknown[]) => {
        aiCalls.push(input);
        return 'This is corrected text.';
      },
    },
    desktop: { selection: { text: async () => selectedText } },
    actions: {
      pasteText: (text: string, title: string, options: unknown) => ({
        type: 'pasteText',
        text,
        title,
        ...options,
      }),
    },
    navigation: {
      run: (action: unknown) => {
        actions.push(action);
        return action;
      },
    },
    ui: { toast: (input: unknown) => input },
  };
  return { context, preparations, aiCalls, actions };
}

test('corrects selected text with Fast AI and replaces it without changing the clipboard', async () => {
  const { context, preparations, aiCalls, actions } = contextFor(
    'this are selected text',
  );

  await commandHandler(context)(context, {});

  assert.equal(preparations.length, 1);
  assert.deepEqual((preparations[0] as any).model, 'fast');
  assert.equal(aiCalls.length, 1);
  assert.match(String((aiCalls[0] as any)[0]), /this are selected text/);
  assert.deepEqual((aiCalls[0] as any)[1].model, 'fast');
  assert.match(
    String((aiCalls[0] as any)[1].system),
    /Return only the corrected text/,
  );
  assert.deepEqual(actions, [
    {
      type: 'pasteText',
      text: 'This is corrected text.',
      title: 'Replace Selected Text',
      concealed: true,
      restoreClipboard: true,
      dismissAfterRun: 'auto',
    },
  ]);
});

test('does not call AI when no text is selected', async () => {
  const { context, aiCalls, actions } = contextFor('   ');

  const result = await commandHandler(context)(context, {});

  assert.deepEqual(result, { message: 'Select text to fix', tone: 'info' });
  assert.equal(aiCalls.length, 0);
  assert.equal(actions.length, 0);
});
