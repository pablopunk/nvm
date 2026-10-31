import assert from 'node:assert/strict';
import test from 'node:test';
import {
  AI_BUILDER_EXTENSION_ID,
  createAiBuilderApiFactory,
} from './extension-ai-builder';

test('startChat sends the described request as the builder chat first prompt', async () => {
  const views: Array<{ item: unknown; options: unknown }> = [];
  const createApi = createAiBuilderApiFactory({
    aiChatView: async (item, options) => {
      views.push({ item, options });
      return { type: 'chat' };
    },
    createDraftAiChat: (query) => ({ query, messages: [] }),
  } as never);
  const api = createApi({ id: AI_BUILDER_EXTENSION_ID }) as any;

  await api.startChat({ prompt: 'Summarize my clipboard' }).__handler();

  assert.deepEqual(views[0].options, {
    initialPrompt: 'Summarize my clipboard',
  });
});
