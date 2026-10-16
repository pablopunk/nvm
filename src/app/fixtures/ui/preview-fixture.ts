import type { ExtensionContext } from '../../resources/nevermind-extension-api';

export function previewView(ctx: ExtensionContext) {
  const templateAction = ctx.actions.run(
    'Expand Template',
    async (innerCtx) => {
      const output = await innerCtx.text.template(
        'Today is {date} at {time}. 6 * 7 = {calculator:6*7}. Selection: {selectedText}',
        { name: 'Nevermind' },
      );
      const cursor = await innerCtx.text.template(
        'Snippet for {name}: {cursor}done. Missing {argument:topic}',
        {
          variables: { name: 'Nevermind' },
          returnCursor: true,
          promptMissing: true,
        },
      );
      return innerCtx.ui.preview({
        title: 'Template Output',
        content: `# Template Output\n\n${output}\n\nCursor-aware result:\n\n\`\`\`json\n${JSON.stringify(cursor, null, 2)}\n\`\`\``,
      });
    },
  );
  return ctx.ui.preview({
    id: 'dev-ui-preview',
    title: 'Dev UI · Preview',
    content:
      '# Preview Fixture\n\nThis exercises markdown/text preview, action panels, and `ctx.text.template(...)`.',
    actions: [templateAction],
    actionPanel: { sections: [{ actions: [templateAction] }] },
  });
}
