import type { ExtensionContext } from '../resources/nevermind-extension-api';

export function classifyImageAction(ctx: ExtensionContext) {
  return ctx.input.prompt({
    title: 'Classify Image with AI',
    fields: [
      {
        id: 'image',
        label: 'Image',
        type: 'file',
        extensions: ['png', 'jpg', 'jpeg'],
        filterName: 'Images',
      },
    ],
    submitTitle: 'Classify Image',
    action: ctx.actions.run('Classify Image', async (innerCtx, action) => {
      if (!innerCtx.ai) throw new Error('AI is unavailable');
      const path = String(action.formValues?.image || '');
      const result = await innerCtx.ai.decide({
        input: 'Classify this image.',
        attachments: [innerCtx.ai.attachments.image(path)],
        questions: [
          {
            id: 'category',
            type: 'choice',
            question: 'Which category best describes this image?',
            options: [
              'photograph',
              'screenshot',
              'document',
              'illustration',
              'other',
            ],
          },
        ],
        signal: AbortSignal.timeout(30_000),
      });
      const category = result.answers.category;
      if (category?.type !== 'choice')
        throw new Error('Image classification failed');
      return innerCtx.ui.list({
        title: 'Image Classification',
        items: [
          {
            id: 'category',
            title: category.value,
            subtitle: 'Copy classification',
            primaryAction: innerCtx.actions.copyText(category.value),
          },
        ],
        emptyView: { title: 'No classification' },
      });
    }),
  });
}
