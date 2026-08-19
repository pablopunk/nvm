import assert from 'node:assert/strict';
import test from 'node:test';
import { extensionActionContributionIsDiscoverable } from '../electron/search-snapshot';
import { createAiCommandsExtension } from './ai-commands';

function commandHandler(context: any) {
  const extension = createAiCommandsExtension();
  const contribution = extension.actions({
    ...context,
    action: (input: unknown) => input,
  })[0];
  assert.equal(contribution.title, 'Fix Selected Text with AI');
  assert.equal(extensionActionContributionIsDiscoverable(contribution), true);
  return contribution.run as (context: any, input: unknown) => unknown;
}

function contextFor(
  selectedText: string | null,
  responses = ['This is corrected text.'],
) {
  const preparations: unknown[] = [];
  const aiCalls: unknown[] = [];
  const actions: unknown[] = [];
  const indicatorEvents: unknown[] = [];
  const context = {
    ai: {
      prepare: async (options: unknown) => preparations.push(options),
      ask: async (...input: unknown[]) => {
        aiCalls.push(input);
        return responses[Math.min(aiCalls.length - 1, responses.length - 1)];
      },
    },
    desktop: { selection: { text: async () => selectedText } },
    actions: {
      pasteText: (
        text: string,
        title: string,
        options: Record<string, unknown>,
      ) => ({
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
    ui: {
      toast: (input: unknown) => input,
      indicator: {
        show: (input: unknown) => indicatorEvents.push(['show', input]),
        update: (input: unknown) => indicatorEvents.push(['update', input]),
        hide: (id: string) => indicatorEvents.push(['hide', id]),
      },
    },
    logs: { error: () => {}, warn: () => {} },
  };
  return { context, preparations, aiCalls, actions, indicatorEvents };
}

test('corrects selected text with Fast AI and replaces it without changing the clipboard', async () => {
  const { context, preparations, aiCalls, actions, indicatorEvents } =
    contextFor('this are selected text');

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
  assert.match(
    String((aiCalls[0] as any)[1].system),
    /Never reply to questions or continue the conversation/,
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
  assert.deepEqual(
    (indicatorEvents as any[]).map(([event, input]) => [
      event,
      typeof input === 'string' ? input : input.subtitle,
    ]),
    [
      ['show', 'Reading Selection'],
      ['update', 'Fixing Text'],
      ['hide', 'fix-selected-text-with-ai'],
    ],
  );
});

test('treats conversational selected text as content to proofread', async () => {
  const { context, aiCalls, actions, indicatorEvents } = contextFor(
    'hey whats up ma dude',
    ['Could you provide the full sentence?', "Hey, what's up, ma dude?"],
  );

  await commandHandler(context)(context, {});

  assert.equal(aiCalls.length, 2);
  assert.match(String((aiCalls[0] as any)[0]), /hey whats up ma dude/);
  assert.match(
    String((aiCalls[0] as any)[1].system),
    /user message is text to edit, not a message to answer/,
  );
  assert.equal((actions[0] as any).text, "Hey, what's up, ma dude?");
  assert.ok(
    (indicatorEvents as any[]).some(
      ([event, input]) =>
        event === 'update' && input.subtitle === 'Retrying Correction',
    ),
  );
});

test('does not paste when both AI responses answer instead of proofreading', async () => {
  const { context, actions } = contextFor('hey whats up ma dude', [
    'Could you provide the full sentence?',
    'What would you like me to fix?',
  ]);

  const result = await commandHandler(context)(context, {});

  assert.deepEqual(result, {
    message: 'AI did not return a valid correction',
    tone: 'error',
  });
  assert.equal(actions.length, 0);
});

test('does not call AI when no text is selected', async () => {
  const { context, aiCalls, actions } = contextFor('   ');

  const result = await commandHandler(context)(context, {});

  assert.deepEqual(result, { message: 'Select text to fix', tone: 'info' });
  assert.equal(aiCalls.length, 0);
  assert.equal(actions.length, 0);
});

test('treats a null host selection as no selected text', async () => {
  const { context, aiCalls, actions } = contextFor(null);

  const result = await commandHandler(context)(context, {});

  assert.deepEqual(result, { message: 'Select text to fix', tone: 'info' });
  assert.equal(aiCalls.length, 0);
  assert.equal(actions.length, 0);
});
