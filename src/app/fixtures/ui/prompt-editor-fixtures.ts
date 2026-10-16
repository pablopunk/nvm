import type { ExtensionContext } from '../../resources/nevermind-extension-api';

export function promptView(ctx: ExtensionContext) {
  const prompt = ctx.input.prompt({
    title: 'Create Quicklink URL',
    message: 'Prompt for lightweight arguments before running an action.',
    fields: [
      {
        id: 'query',
        label: 'Search query',
        type: 'text',
        placeholder: 'never mind extension api',
        required: true,
      },
      {
        id: 'site',
        label: 'Site',
        type: 'dropdown',
        value: 'github.com',
        options: [
          { title: 'GitHub', value: 'github.com' },
          { title: 'Docs', value: 'docs' },
          { title: 'Web', value: 'web' },
        ],
      },
    ],
    submitTitle: 'Build URL',
    action: ctx.actions.run('Show Prompt Values', async (_ctx, action) => {
      const query = String(action.formValues?.query || '');
      const site = String(action.formValues?.site || '');
      const url = `https://www.google.com/search?q=${encodeURIComponent(site && site !== 'web' ? `site:${site} ${query}` : query)}`;
      const template = await _ctx.text.template(
        'Open {argument:site} for {argument:query}{cursor}',
        { variables: action.formValues, returnCursor: true },
      );
      return _ctx.ui.preview({
        title: 'Prompt Result',
        content: `# Prompt Result\n\n- Query: ${query || '_empty_'}\n- Site: ${site || '_empty_'}\n- URL: ${url}\n\nTemplate result:\n\n\`\`\`json\n${JSON.stringify(template, null, 2)}\n\`\`\``,
      });
    }),
  });
  return ctx.ui.list({
    id: 'dev-ui-prompt',
    title: 'Dev UI · Prompt',
    subtitle: 'Host-owned lightweight argument prompt before an action runs',
    items: [
      ctx.ui.item({
        id: 'prompt',
        title: 'Prompt for Quicklink Arguments',
        subtitle: 'Collects each argument through the palette input',
        icon: 'text-cursor-input',
        primaryAction: prompt,
        actions: [prompt],
      }),
    ],
  });
}

export function editorView(ctx: ExtensionContext) {
  return ctx.ui.editor({
    id: 'dev-ui-editor',
    title: 'Dev UI · Editor',
    subtitle:
      'Editable markdown with a host-rendered preview and submit action',
    format: 'markdown',
    preview: true,
    placeholder: 'Write markdown…',
    content:
      '# Release Note Draft\n\n- Built with host-owned editor UI.\n- Supports **markdown preview**.\n- Submit injects `editorContent` into the action.\n- Draft autosave recovers unsaved content on reopen.',
    draft: {
      key: 'dev-ui-editor-draft',
      version: 1,
      autosave: { debounceMs: 500 },
    },
    submitAction: ctx.actions.run('Preview Draft', (_ctx, action) =>
      ctx.ui.preview({
        title: 'Submitted Editor Content',
        content: `# Submitted Editor Content\n\n${action.editorContent || '_Empty draft_'}`,
      }),
    ),
    actions: [
      ctx.actions.copyText('Copied from editor fixture', 'Copy Sample Text'),
    ],
  });
}
