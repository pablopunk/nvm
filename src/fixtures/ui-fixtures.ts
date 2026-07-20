import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import type {
  ExtensionAction,
  ExtensionContext,
  ExtensionFile,
  ExtensionFormValue,
  ExtensionOcrResult,
  NevermindExtension,
} from '../resources/nevermind-extension-api';

const WATCH_FIXTURE_ROOT = path.join(
  os.tmpdir(),
  'nvm-extension-watch-fixture',
);

function valuesMarkdown(values: Record<string, ExtensionFormValue> = {}) {
  const rows = Object.entries(values).map(
    ([key, value]) =>
      `- **${key}**: ${Array.isArray(value) ? value.join(', ') : String(value)}`,
  );
  return rows.length ? rows.join('\n') : 'No values submitted.';
}

function formView(ctx: ExtensionContext) {
  return ctx.ui.form({
    id: 'dev-ui-form',
    title: 'Dev UI · Form Fields',
    subtitle: 'Exercises every host-rendered form field type',
    fields: [
      {
        id: 'intro',
        type: 'description',
        description:
          'This dev-only extension is loaded only in electron-vite dev mode.',
      },
      {
        id: 'name',
        label: 'Text',
        type: 'text',
        value: 'Nevermind',
        placeholder: 'Name',
        required: true,
        description: 'Plain single-line text.',
      },
      {
        id: 'notes',
        label: 'Textarea',
        type: 'textarea',
        value: 'Longer editable text',
        rows: 5,
        description: 'Multi-line input for notes, templates, and prompts.',
      },
      {
        id: 'password',
        label: 'Password',
        type: 'password',
        placeholder: 'Secret',
      },
      { id: 'url', label: 'URL', type: 'url', value: 'https://nvm.fyi' },
      { id: 'number', label: 'Number', type: 'number', value: '42' },
      { id: 'date', label: 'Date', type: 'date' },
      {
        id: 'enabled',
        label: 'Checkbox',
        type: 'checkbox',
        value: true,
        description: 'Boolean form value.',
      },
      { id: 'separator', type: 'separator' },
      {
        id: 'color',
        label: 'Dropdown',
        type: 'dropdown',
        value: 'blue',
        options: [
          { title: 'Blue', value: 'blue' },
          { title: 'Green', value: 'green' },
          { title: 'Purple', value: 'purple' },
        ],
      },
      {
        id: 'tags',
        label: 'Multiselect',
        type: 'multiselect',
        value: ['templates', 'forms'],
        options: [
          { title: 'Templates', value: 'templates' },
          { title: 'Forms', value: 'forms' },
          { title: 'Files', value: 'files' },
        ],
      },
      {
        id: 'inputFile',
        label: 'File picker',
        type: 'file',
        placeholder: 'Choose one source file',
        description: 'Single path value from the host native file picker.',
      },
      {
        id: 'inputFiles',
        label: 'Files picker',
        type: 'files',
        placeholder: 'Choose one or more images',
        extensions: ['png', 'jpg', 'jpeg', 'webp'],
        filterName: 'Images',
        description: 'Array value from a multi-select native file picker.',
      },
      {
        id: 'outputFolder',
        label: 'Folder picker',
        type: 'folder',
        placeholder: 'Choose an output folder',
        description: 'Folder path value from the host native folder picker.',
      },
      {
        id: 'invalid',
        label: 'Error State',
        type: 'text',
        value: 'Bad value',
        error: 'Example field-level error copy.',
      },
    ],
    submitAction: ctx.actions.run('Show Submitted Values', (_ctx, action) =>
      ctx.ui.preview({
        title: 'Submitted Form Values',
        content: `# Submitted Form Values\n\n${valuesMarkdown(action.formValues)}`,
      }),
    ),
  });
}

const FLOATING_WINDOW_ID = 'dev-ui-floating-window';
const FLOATING_WINDOW_OPTIONS = {
  id: FLOATING_WINDOW_ID,
  title: 'Nevermind Floating Note',
  titleBar: 'hidden' as const,
  chrome: 'none' as const,
  width: 560,
  height: 520,
  alwaysOnTop: true,
  visibleOnAllSpaces: true,
};

function floatingNoteEditorView(ctx: ExtensionContext) {
  return ctx.ui.editor({
    id: FLOATING_WINDOW_ID,
    title: 'Floating Note',
    subtitle: 'Editable host-rendered note',
    format: 'text',
    placeholder: 'Write a floating note…',
    content:
      'Floating Note\n\nEdit this note in a real independent window.\n\n- Always on top\n- Reuses ctx.ui.editor(...)\n- No palette chrome, preview, or action menu',
  });
}

function floatingWindowToggleAction(ctx: ExtensionContext) {
  return ctx.action({
    id: 'toggle-floating-note',
    title: 'Toggle Floating Note',
    subtitle:
      'Persistent action: creates the note if needed, then shows or hides it',
    icon: 'panel-top-open',
    keywords: ['window', 'note', 'shortcut', 'persistent action'],
    action: ctx.windows.toggle(
      floatingNoteEditorView(ctx),
      FLOATING_WINDOW_OPTIONS,
    ),
  });
}

function backgroundJobFixtureAction(ctx: ExtensionContext) {
  return ctx.action({
    id: 'background-job-fixture',
    title: 'Background Job Fixture',
    subtitle: 'Dev fixture for extension-owned host jobs and diagnostics',
    icon: 'activity',
    mode: 'background',
    triggers: [
      { type: 'startup', delayMs: 750 },
      { type: 'app.frontmost.changed', debounceMs: 1000 },
      { type: 'wake' },
      { type: 'login' },
    ],
    async run(innerCtx) {
      const count =
        (await innerCtx.storage.get<number>('backgroundJobRuns', 0)) || 0;
      await innerCtx.storage.set('backgroundJobRuns', count + 1);
      return innerCtx.ui.toast({
        message: `Background fixture run #${count + 1}`,
      });
    },
  });
}

function folderWatchFixtureAction(ctx: ExtensionContext) {
  fs.mkdirSync(WATCH_FIXTURE_ROOT, { recursive: true });
  return ctx.action({
    id: 'folder-watch-fixture',
    title: 'Folder Watch Fixture Job',
    subtitle: 'Watches a temp folder and records ctx.launch.changedPaths/files',
    icon: 'folder-sync',
    mode: 'background',
    triggers: [
      {
        type: 'files.changed',
        roots: [WATCH_FIXTURE_ROOT],
        debounceMs: 250,
        extensions: ['txt', 'png'],
        ignore: ['ignored*'],
      },
    ],
    async run(innerCtx) {
      const count =
        (await innerCtx.storage.get<number>('folderWatchRuns', 0)) || 0;
      await innerCtx.storage.set('folderWatchRuns', count + 1);
      await innerCtx.storage.set('folderWatchLaunch', innerCtx.launch || null);
      return innerCtx.ui.toast({
        message: `Folder watch fixture run #${count + 1}`,
      });
    },
  });
}

async function folderWatchView(ctx: ExtensionContext) {
  fs.mkdirSync(WATCH_FIXTURE_ROOT, { recursive: true });
  const count = (await ctx.storage.get<number>('folderWatchRuns', 0)) || 0;
  const launch = await ctx.storage.get<any>('folderWatchLaunch', null);
  const touchAction = ctx.actions.run(
    'Touch Watched File',
    async (innerCtx) => {
      fs.mkdirSync(WATCH_FIXTURE_ROOT, { recursive: true });
      const filePath = path.join(
        WATCH_FIXTURE_ROOT,
        `changed-${Date.now()}.txt`,
      );
      fs.writeFileSync(
        filePath,
        `Nevermind folder watch fixture ${new Date().toISOString()}\n`,
      );
      return innerCtx.ui.toast({
        message: `Touched ${path.basename(filePath)}`,
      });
    },
  );
  return ctx.ui.preview({
    id: 'dev-ui-folder-watch',
    title: 'Dev UI · Folder Watch',
    subtitle: WATCH_FIXTURE_ROOT,
    content: `# Folder Watch Fixture\n\nRoot: \`${WATCH_FIXTURE_ROOT}\`\n\nRuns: **${count}**\n\nLast launch context:\n\n\`\`\`json\n${JSON.stringify(launch, null, 2) || 'null'}\n\`\`\`\n\nUse the action panel to touch a watched .txt file, then reopen this view or check Background Tasks.`,
    actions: [touchAction],
    actionPanel: { sections: [{ actions: [touchAction] }] },
  });
}

function floatingWindowToggleItem(ctx: ExtensionContext) {
  return ctx.ui.item({
    id: 'toggle-floating-note',
    title: 'Toggle Floating Note',
    subtitle:
      'References the persistent action, so aliases and shortcuts attach to the durable action id',
    icon: 'panel-top-open',
    keywords: ['window', 'note', 'shortcut', 'persistent action'],
    primaryAction: ctx.actions.ref(
      'toggle-floating-note',
      'Toggle Floating Note',
    ),
  });
}

function floatingWindowView(ctx: ExtensionContext) {
  return ctx.ui.list({
    id: 'dev-ui-floating-window-controls',
    title: 'Dev UI · Floating Window',
    subtitle:
      'Open, toggle, and close an independent host-rendered extension window',
    items: [
      floatingWindowToggleItem(ctx),
      ctx.ui.item({
        id: 'close',
        title: 'Close Floating Note',
        subtitle: 'Closes the existing window',
        icon: 'x',
        primaryAction: ctx.windows.close(
          FLOATING_WINDOW_ID,
          'Close Floating Note',
        ),
      }),
    ],
  });
}

function renderingPolishView(ctx: ExtensionContext) {
  const openAction = ctx.actions.openUrl('https://nvm.fyi', 'Open Link');
  return ctx.ui.list({
    id: 'dev-ui-rendering-polish',
    title: 'Dev UI · Rendering Polish',
    subtitle:
      'Accessories, metadata details, image descriptors, and side inspector panes',
    selectedItemId: 'details',
    detail: { visible: true, placement: 'side' },
    items: [
      ctx.ui.item({
        id: 'details',
        title: 'Detailed Result',
        subtitle: 'Selected row renders an inspector pane',
        icon: 'sparkles',
        accessories: [
          { text: 'Ready', tone: 'success' },
          { text: 'AI', tone: 'accent', tooltip: 'Generated-friendly host UI' },
        ],
        image: {
          src: 'data:image/svg+xml,%3Csvg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64"%3E%3Crect width="64" height="64" rx="16" fill="%23202232"/%3E%3Cpath d="M18 40L29 18l6 16 11-10-10 24-7-16z" fill="%23ffd84d"/%3E%3C/svg%3E',
          shape: 'rounded',
          alt: 'Nevermind fixture mark',
        },
        detail: {
          title: 'Detailed Result',
          subtitle: 'Host-rendered metadata and markdown',
          markdown:
            'Use `detail.markdown`, `detail.metadata`, toned accessories, and detail actions instead of jumping to a webview for common inspector layouts.',
          metadata: [
            { label: 'Status', value: 'Ready', type: 'tag', tone: 'success' },
            { label: 'Kind', value: 'Fixture' },
            {
              type: 'link',
              label: 'Docs',
              value: 'nvm.fyi',
              url: 'https://nvm.fyi',
            },
            { type: 'separator' },
            { label: 'Updated', value: new Date().toLocaleString() },
          ],
          actions: [openAction],
        },
        primaryAction: openAction,
        actions: [openAction],
      }),
      ctx.ui.item({
        id: 'warning',
        title: 'Warning Accessory',
        subtitle: 'Foreground colors title and icon together',
        icon: 'alert-triangle',
        appearance: { foreground: 'orange' },
        accessories: [
          { text: 'Warning', tone: 'warning' },
          { text: 'Muted', tone: 'muted' },
        ],
        detail: {
          title: 'Warning Accessory',
          markdown:
            '`appearance.foreground` colors both the row title and the Lucide icon while accessories keep their own tones.',
          metadata: [
            { type: 'tag', label: 'Tone', value: 'warning', tone: 'warning' },
          ],
        },
      }),
      ctx.ui.item({
        id: 'danger',
        title: 'Danger Accessory',
        subtitle: 'Danger foreground matches icon and title',
        icon: 'shield-alert',
        appearance: { foreground: 'red' },
        accessories: [{ text: 'Danger', tone: 'danger' }],
        detail: {
          title: 'Danger Accessory',
          markdown:
            'Foreground styling uses existing semantic color tokens and remains compact.',
        },
      }),
    ],
  });
}

function clipboardView(ctx: ExtensionContext) {
  const clipboardApi = ctx.desktop.clipboard;
  const writeHtml = ctx.actions.run(
    'Write HTML Clipboard',
    async (innerCtx) => {
      innerCtx.desktop.clipboard?.write(
        {
          type: 'html',
          html: '<strong>Nevermind HTML Clipboard</strong>',
          text: 'Nevermind HTML Clipboard',
        },
        { concealed: true },
      );
      return innerCtx.ui.toast({
        message: 'Wrote concealed HTML clipboard content',
      });
    },
  );
  const writeFiles = ctx.actions.run(
    'Write Files Clipboard',
    async (innerCtx) => {
      fs.mkdirSync(WATCH_FIXTURE_ROOT, { recursive: true });
      const filePath = path.join(WATCH_FIXTURE_ROOT, 'clipboard-file.txt');
      fs.writeFileSync(filePath, 'Nevermind clipboard file fixture\n');
      innerCtx.desktop.clipboard?.writeFiles([filePath], { concealed: true });
      return innerCtx.ui.toast({
        message: `Wrote ${path.basename(filePath)} as clipboard file`,
      });
    },
  );
  const read = ctx.actions.run('Read Clipboard', async (innerCtx) => {
    const value = await innerCtx.desktop.clipboard?.read({
      formats: ['files', 'html', 'text', 'image'],
    });
    return innerCtx.ui.preview({
      title: 'Clipboard Read Result',
      content: `\`\`\`json\n${JSON.stringify(value, null, 2)}\n\`\`\``,
    });
  });
  const pasteHtml = ctx.actions.paste(
    {
      type: 'html',
      html: '<em>Nevermind pasted HTML</em>',
      text: 'Nevermind pasted HTML',
    },
    'Paste HTML + Restore',
    { restoreClipboard: true, concealed: true, dismissAfterRun: 'auto' },
  );
  return ctx.ui.list({
    id: 'dev-ui-clipboard',
    title: 'Dev UI · Clipboard',
    subtitle: clipboardApi
      ? 'Read/write text, HTML, files, and generic paste content'
      : 'Clipboard history host API unavailable',
    items: [
      ctx.ui.item({
        id: 'write-html',
        title: 'Write concealed HTML',
        subtitle: 'Writes HTML with a plain-text fallback',
        icon: 'clipboard',
        primaryAction: writeHtml,
        actions: [writeHtml],
      }),
      ctx.ui.item({
        id: 'write-files',
        title: 'Write files to clipboard',
        subtitle: WATCH_FIXTURE_ROOT,
        icon: 'files',
        primaryAction: writeFiles,
        actions: [writeFiles],
      }),
      ctx.ui.item({
        id: 'read',
        title: 'Read current clipboard',
        subtitle: 'Prefers files, image, HTML, then text',
        icon: 'clipboard-list',
        primaryAction: read,
        actions: [read],
      }),
      ctx.ui.item({
        id: 'paste-html',
        title: 'Paste HTML and restore clipboard',
        subtitle: 'Uses ctx.actions.paste(...) with concealed restore behavior',
        icon: 'clipboard-paste',
        primaryAction: pasteHtml,
        actions: [pasteHtml],
      }),
    ],
  });
}

function textInputView(ctx: ExtensionContext) {
  return ctx.ui.list({
    id: 'dev-ui-text-input',
    title: 'Dev UI · Text Input',
    subtitle: 'Paste/type primitives for snippet and selected-text workflows',
    items: [
      ctx.ui.item({
        id: 'paste-concealed-restore',
        title: 'Paste concealed text and restore clipboard',
        subtitle:
          'Paste into the frontmost app without keeping this text in clipboard history',
        icon: 'clipboard-paste',
        primaryAction: ctx.actions.pasteText(
          'Nevermind concealed restored paste',
          'Paste Concealed + Restore',
          { restoreClipboard: true, concealed: true, dismissAfterRun: 'auto' },
        ),
        actions: [
          ctx.actions.pasteText(
            'Nevermind concealed restored paste',
            'Paste Concealed + Restore',
            {
              restoreClipboard: true,
              concealed: true,
              dismissAfterRun: 'auto',
            },
          ),
        ],
      }),
      ctx.ui.item({
        id: 'paste-keep-open',
        title: 'Paste text and keep palette open',
        subtitle: 'Useful for repeated snippet insertion tests',
        icon: 'panel-top-open',
        primaryAction: ctx.actions.pasteText(
          'Nevermind keep-open paste',
          'Paste and Keep Open',
          { keepPaletteOpen: true, dismissAfterRun: 'auto' },
        ),
        actions: [
          ctx.actions.pasteText(
            'Nevermind keep-open paste',
            'Paste and Keep Open',
            { keepPaletteOpen: true, dismissAfterRun: 'auto' },
          ),
        ],
      }),
      ctx.ui.item({
        id: 'type-text',
        title: 'Type text without clipboard',
        subtitle: 'Uses host keyboard typing when supported',
        icon: 'keyboard',
        primaryAction: ctx.actions.typeText(
          'Nevermind typed text',
          'Type Text',
          { dismissAfterRun: 'auto' },
        ),
        actions: [
          ctx.actions.typeText('Nevermind typed text', 'Type Text', {
            dismissAfterRun: 'auto',
          }),
        ],
      }),
    ],
  });
}

function promptView(ctx: ExtensionContext) {
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
        subtitle: 'Opens a form and then runs the wrapped action',
        icon: 'text-cursor-input',
        primaryAction: prompt,
        actions: [prompt],
      }),
    ],
  });
}

function editorView(ctx: ExtensionContext) {
  return ctx.ui.editor({
    id: 'dev-ui-editor',
    title: 'Dev UI · Editor',
    subtitle:
      'Editable markdown with a host-rendered preview and submit action',
    format: 'markdown',
    placeholder: 'Write markdown…',
    content:
      '# Release Note Draft\n\n- Built with host-owned editor UI.\n- Supports **markdown preview**.\n- Submit injects `editorContent` into the action.',
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

function fileIndexControlsAction(ctx: ExtensionContext) {
  return ctx.actions.run('Show File Index Snapshot', async (innerCtx) => {
    const files = innerCtx.desktop.files;
    if (!files)
      return innerCtx.ui.preview({
        title: 'File Index Unavailable',
        content: 'The `desktop.files` host API is not available.',
      });
    const roots = files.indexedRoots();
    const snapshot = files.indexSnapshot({
      roots: ['~/Downloads'],
      extensions: ['png', 'jpg', 'jpeg', 'mp4', 'mov'],
      limit: 8,
      ignore: ['*.tmp'],
    });
    return innerCtx.ui.preview({
      title: 'File Index Snapshot',
      content: [
        '# File Index Snapshot',
        '',
        `Default roots: ${roots.join(', ')}`,
        '',
        ...snapshot.map(
          (file) =>
            `- **${file.name}** · ${file.kind || 'file'} · ${file.displayPath || file.path}`,
        ),
      ].join('\n'),
    });
  });
}

function reindexDownloadsAction(ctx: ExtensionContext) {
  return ctx.actions.run('Reindex Downloads Media', async (innerCtx) => {
    const files = innerCtx.desktop.files;
    if (!files)
      return innerCtx.ui.preview({
        title: 'File Index Unavailable',
        content: 'The `desktop.files` host API is not available.',
      });
    const result = await files.reindex({
      roots: ['~/Downloads'],
      kind: 'media',
      depth: 2,
      limit: 500,
      ignore: ['*.tmp'],
    });
    return innerCtx.ui.preview({
      title: 'Reindex Complete',
      content: `# Reindex Complete\n\nIndexed ${result.count} files from ${result.roots.join(', ')}.`,
    });
  });
}

function listView(ctx: ExtensionContext) {
  const confirm = ctx.ui.confirm({
    title: 'Confirm Dev Action',
    message: 'This confirms host-owned action UI still renders correctly.',
    confirmLabel: 'Show Toast',
    onConfirm: ctx.actions.run('Show Toast', () =>
      ctx.ui.toast({ message: 'Confirmed from dev UI fixture' }),
    ),
  });
  return ctx.ui.list({
    id: 'dev-ui-list',
    title: 'Dev UI · List',
    subtitle:
      'List rows, accessories, sections, action panels, confirmation, and navigation',
    searchBarPlaceholder: 'Filter fixture rows',
    sections: [
      {
        title: 'Rows',
        items: [
          ctx.ui.item({
            id: 'form',
            title: 'Open Form Fixture',
            subtitle: 'Textarea, dropdowns, errors, descriptions',
            icon: 'list-checks',
            accessories: [{ text: 'form' }],
            primaryAction: ctx.actions.push('Open Form', formView(ctx)),
          }),
          ctx.ui.item({
            id: 'root-navigation',
            title: 'Open Root Navigation Fixture',
            subtitle: 'Replaces the stack; Escape returns to the palette root',
            icon: 'corner-up-left',
            accessories: [{ text: 'root' }],
            primaryAction: ctx.actions.root(
              'Open Root Navigation',
              ctx.ui.preview({
                id: 'dev-ui-root-navigation',
                title: 'Dev UI · Root Navigation',
                content:
                  '# Root Navigation\n\nThis view intentionally resets the palette navigation stack.',
              }),
            ),
          }),
          ctx.ui.item({
            id: 'text-input',
            title: 'Open Text Input Fixture',
            subtitle: 'Paste/type actions for snippets and transforms',
            icon: 'keyboard',
            accessories: [{ text: 'text' }],
            primaryAction: ctx.actions.push(
              'Open Text Input',
              textInputView(ctx),
            ),
          }),
          ctx.ui.item({
            id: 'floating-window',
            title: 'Open Floating Window Fixture',
            subtitle: 'Independent host-rendered extension window',
            icon: 'panel-top-open',
            accessories: [{ text: 'window' }],
            primaryAction: ctx.actions.push(
              'Open Floating Window',
              floatingWindowView(ctx),
            ),
          }),
          ctx.ui.item({
            id: 'prompt',
            title: 'Open Prompt Fixture',
            subtitle: 'Prompted arguments before an action runs',
            icon: 'text-cursor-input',
            accessories: [{ text: 'prompt' }],
            primaryAction: ctx.actions.push('Open Prompt', promptView(ctx)),
          }),
          ctx.ui.item({
            id: 'editor',
            title: 'Open Editor Fixture',
            subtitle: 'Editable markdown, preview, submit payload',
            icon: 'file-pen-line',
            accessories: [{ text: 'editor' }],
            primaryAction: ctx.actions.push('Open Editor', editorView(ctx)),
          }),
          ctx.ui.item({
            id: 'preview',
            title: 'Open Preview Fixture',
            subtitle: 'Markdown/text preview',
            icon: 'file-text',
            accessories: [{ text: 'preview' }],
            primaryAction: ctx.actions.push('Open Preview', previewView(ctx)),
          }),
          ctx.ui.item({
            id: 'file-index',
            title: 'File Index Controls',
            subtitle:
              'Snapshot and bounded reindex helpers for generated file searchers',
            icon: 'folder-search',
            accessories: [{ text: 'files' }],
            primaryAction: fileIndexControlsAction(ctx),
            actions: [
              fileIndexControlsAction(ctx),
              reindexDownloadsAction(ctx),
            ],
          }),
          ctx.ui.item({
            id: 'ocr',
            title: 'OCR Fixture',
            subtitle: 'Image, screen, and region text recognition helpers',
            icon: 'scan-text',
            accessories: [{ text: 'ocr' }],
            primaryAction: ctx.actions.push('Open OCR', ocrFixtureView(ctx)),
          }),
          ctx.ui.item({
            id: 'confirm',
            title: 'Confirmation Fixture',
            subtitle: 'Host-owned confirm step',
            icon: 'shield-check',
            accessories: [{ text: 'confirm' }],
            primaryAction: confirm,
            actionPanel: {
              sections: [
                {
                  actions: [
                    confirm,
                    ctx.actions.copyText(
                      'copied from dev fixture',
                      'Copy Fixture Text',
                    ),
                  ],
                },
              ],
            },
          }),
        ],
      },
    ],
  });
}

function fileMetadataMarkdown(file: ExtensionFile) {
  return [
    `# ${file.name}`,
    '',
    `- Path: ${file.displayPath || file.path}`,
    `- Kind: ${file.kind || 'file'}`,
    `- MIME: ${file.mimeType || 'unknown'}`,
    `- Size: ${file.size || 0} bytes`,
    file.width && file.height
      ? `- Dimensions: ${file.width} × ${file.height}`
      : '',
    file.mtime ? `- Modified: ${file.mtime}` : '',
  ]
    .filter(Boolean)
    .join('\n');
}

function ocrResultMarkdown(result: ExtensionOcrResult, source: string) {
  const text = result.text?.trim() || '_No text recognized._';
  const confidence =
    typeof result.confidence === 'number'
      ? `${Math.round(result.confidence * 100)}%`
      : 'unknown';
  return [
    '# OCR Result',
    '',
    `- Source: ${source}`,
    `- Confidence: ${confidence}`,
    `- Blocks: ${result.blocks?.length || 0}`,
    '',
    '## Text',
    '',
    text,
  ].join('\n');
}

function ocrUnavailableView(ctx: ExtensionContext) {
  return ctx.ui.preview({
    title: 'OCR Unavailable',
    content:
      '# OCR Unavailable\n\nDeclare the `ocr` capability for review and check `ctx.system.capabilities.has("ocr")` before using OCR helpers.',
  });
}

function ocrErrorView(ctx: ExtensionContext, error: unknown) {
  return ctx.ui.preview({
    title: 'OCR Error',
    content: `# OCR Error\n\n${error instanceof Error ? error.message : String(error)}`,
  });
}

async function runOcrImage(
  ctx: ExtensionContext,
  input: string | ExtensionFile,
  source: string,
) {
  if (!(ctx.ocr && ctx.system.capabilities.has('ocr')))
    return ocrUnavailableView(ctx);
  try {
    const result = await ctx.ocr.image(input);
    const image =
      typeof input === 'string'
        ? ctx.desktop.files?.thumbnail(input) ||
          ctx.desktop.files?.toFileUrl(input)
        : input.thumbnailUrl || input.url;
    return ctx.ui.preview({
      title: 'OCR Result',
      content: ocrResultMarkdown(result, source),
      image,
    });
  } catch (error) {
    return ocrErrorView(ctx, error);
  }
}

function ocrFixtureView(ctx: ExtensionContext) {
  const chooseImage = ctx.input.prompt({
    title: 'Choose Image for OCR',
    fields: [
      {
        id: 'imagePath',
        label: 'Image',
        type: 'file',
        extensions: ['png', 'jpg', 'jpeg', 'webp', 'tiff', 'heic'],
        filterName: 'Images',
      },
    ],
    action: ctx.actions.run('OCR Chosen Image', (innerCtx, action) =>
      runOcrImage(
        innerCtx,
        String(action.formValues?.imagePath || ''),
        'chosen image',
      ),
    ),
    submitTitle: 'Recognize Text',
  });
  const recentImage = ctx.actions.run('OCR Recent Image', async (innerCtx) => {
    const images =
      (await innerCtx.desktop.files?.findImages(
        ['~/Downloads', '~/Desktop', '~/Pictures'],
        { limit: 1, depth: 2, sortBy: 'recent' },
      )) || [];
    if (!images.length)
      return innerCtx.ui.preview({
        title: 'No Image Found',
        content:
          '# No Image Found\n\nAdd an image to Downloads, Desktop, or Pictures, or use **Choose Image for OCR**.',
      });
    const file = await innerCtx.desktop.files!.metadata(images[0].path);
    return runOcrImage(innerCtx, file, file.displayPath || file.path);
  });
  const screenOcr = ctx.actions.run('OCR Screen', async (innerCtx) => {
    if (!(innerCtx.ocr && innerCtx.system.capabilities.has('ocr')))
      return ocrUnavailableView(innerCtx);
    try {
      const result = await innerCtx.ocr.screen();
      return innerCtx.ui.preview({
        title: 'Screen OCR Result',
        content: ocrResultMarkdown(result, 'screen capture'),
      });
    } catch (error) {
      return ocrErrorView(innerCtx, error);
    }
  });
  const regionOcr = ctx.actions.run('OCR Top-left Region', async (innerCtx) => {
    if (!(innerCtx.ocr && innerCtx.system.capabilities.has('ocr')))
      return ocrUnavailableView(innerCtx);
    try {
      const result = await innerCtx.ocr.region({
        x: 0,
        y: 0,
        width: 900,
        height: 600,
      });
      return innerCtx.ui.preview({
        title: 'Region OCR Result',
        content: ocrResultMarkdown(result, 'screen region 0,0,900×600'),
      });
    } catch (error) {
      return ocrErrorView(innerCtx, error);
    }
  });
  return ctx.ui.list({
    id: 'dev-ui-ocr',
    title: 'Dev UI · OCR',
    subtitle: 'Generic OCR helpers for images, screenshots, and regions',
    items: [
      ctx.ui.item({
        id: 'recent-image',
        title: 'OCR Recent Image',
        subtitle: 'Recognize text in the newest local image',
        icon: 'scan-text',
        primaryAction: recentImage,
        actions: [recentImage, chooseImage],
      }),
      ctx.ui.item({
        id: 'choose-image',
        title: 'Choose Image for OCR',
        subtitle: 'Uses a file picker plus ctx.ocr.image(...)',
        icon: 'file-image',
        primaryAction: chooseImage,
        actions: [chooseImage],
      }),
      ctx.ui.item({
        id: 'screen',
        title: 'OCR Screen',
        subtitle: 'Captures the screen, then recognizes visible text',
        icon: 'monitor',
        primaryAction: screenOcr,
        actions: [screenOcr],
      }),
      ctx.ui.item({
        id: 'region',
        title: 'OCR Top-left Region',
        subtitle:
          'Captures a fixed 900×600 region for dogfooding ctx.ocr.region(...)',
        icon: 'scan-line',
        primaryAction: regionOcr,
        actions: [regionOcr],
      }),
    ],
  });
}

async function gridView(ctx: ExtensionContext) {
  const media = ctx.desktop.files
    ? await ctx.desktop.files.findMedia(
        ['~/Pictures', '~/Desktop', '~/Downloads'],
        { limit: 6, depth: 2, sortBy: 'recent' },
      )
    : [];
  if (media.length) {
    const files = await Promise.all(
      media.map((file) => ctx.desktop.files!.metadata(file.path)),
    );
    return ctx.ui.grid({
      id: 'dev-ui-grid',
      title: 'Dev UI · Grid',
      subtitle:
        'Grid tiles using ctx.desktop.files.metadata(...) and thumbnail(...)',
      layout: 'wide',
      aspectRatio: '16 / 9',
      columns: 3,
      sections: [
        {
          title: 'Recent Media',
          items: files.map((file) => ({
            id: file.path,
            title: file.name,
            subtitle:
              file.width && file.height
                ? `${file.kind} · ${file.width} × ${file.height}`
                : file.displayPath,
            icon: file.kind === 'video' ? 'video' : 'image',
            image: ctx.desktop.files!.thumbnail(file.path) || file.url,
            accessories: [{ text: file.extension || file.kind || 'file' }],
            primaryAction: ctx.actions.push(
              'Show Metadata',
              ctx.ui.preview({
                title: file.name,
                content: fileMetadataMarkdown(file),
                image: file.thumbnailUrl || file.url,
              }),
            ),
            actions: [
              ctx.actions.revealPath(file.path, 'Reveal File'),
              ctx.actions.copyText(file.path, 'Copy Path'),
            ],
          })),
        },
      ],
    });
  }

  const colors = [
    'yellow',
    'blue',
    'purple',
    'green',
    'red',
    'orange',
  ] as const;
  return ctx.ui.grid({
    id: 'dev-ui-grid',
    title: 'Dev UI · Grid',
    subtitle: 'Grid tiles, sections, layout, aspect ratio, and action hints',
    layout: 'wide',
    aspectRatio: '16 / 9',
    columns: 3,
    sections: [
      {
        title: 'Fallback Tiles',
        items: colors.map((color) => ({
          id: color,
          title: `${color[0].toUpperCase()}${color.slice(1)} Tile`,
          subtitle:
            'Generated placeholder tile; add media to Pictures/Desktop/Downloads to exercise file helpers',
          icon: 'image',
          image: `data:image/svg+xml,${encodeURIComponent(`<svg xmlns="http://www.w3.org/2000/svg" width="640" height="360"><rect width="100%" height="100%" rx="28" fill="${color}"/><text x="50%" y="54%" text-anchor="middle" font-family="system-ui" font-size="42" fill="black">${color}</text></svg>`)}`,
          appearance: { foreground: color },
          primaryAction: ctx.actions.copyText(color, `Copy ${color}`),
          actions: [ctx.actions.copyText(color, `Copy ${color}`)],
        })),
      },
    ],
  });
}

function previewView(ctx: ExtensionContext) {
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

type ChatMessage = { role: 'system' | 'user' | 'assistant'; content: string };
const CHAT_STORAGE_KEY = 'dev-ui-chat-messages';

async function chatView(ctx: ExtensionContext) {
  const messages =
    (await ctx.storage.get<ChatMessage[]>(CHAT_STORAGE_KEY, [
      {
        role: 'system',
        content: 'Interactive chat fixture. No AI backend — just local echo.',
      },
      {
        role: 'user',
        content: 'Can generated extensions render chat bubbles?',
      },
      {
        role: 'assistant',
        content:
          'Yes — this is a host-rendered chat view. Type a message below to test the interactive fixture.',
      },
    ])) || [];
  const sendAction = ctx.actions.run(
    'Send Chat Message',
    async (innerCtx, action) => {
      const userMessage = String(action.formValues?.message || '').trim();
      if (!userMessage) return;
      const current =
        (await innerCtx.storage.get<ChatMessage[]>(CHAT_STORAGE_KEY, [])) || [];
      const updated: ChatMessage[] = [
        ...current,
        { role: 'user', content: userMessage },
        { role: 'assistant', content: `Echo: ${userMessage}` },
      ];
      await innerCtx.storage.set(CHAT_STORAGE_KEY, updated);
      return innerCtx.ui.chat({
        id: 'dev-ui-chat',
        title: 'Dev UI · Chat',
        messages: updated,
        submitAction: sendAction,
        placeholder: 'Type a message…',
      });
    },
  );
  return ctx.ui.chat({
    id: 'dev-ui-chat',
    title: 'Dev UI · Chat',
    messages,
    submitAction: sendAction,
    placeholder: 'Type a message…',
  });
}

function progressView(ctx: ExtensionContext) {
  return ctx.ui.progress({
    id: 'dev-ui-progress',
    title: 'Dev UI · Progress',
    status: 'Rendering fixture preview…',
    value: 2,
    total: 3,
    steps: [
      { title: 'Design API primitive', status: 'Done' },
      { title: 'Render fixture', status: 'In progress' },
      { title: 'Dogfood locally', status: 'Next' },
    ],
  });
}

function webviewView(ctx: ExtensionContext) {
  return ctx.ui.webview({
    id: 'dev-ui-webview',
    title: 'Dev UI · Webview',
    html: '<style>:root{color-scheme:dark;--surface-webview:Canvas;--text-webview:CanvasText}body{margin:0;background:var(--surface-webview);color:var(--text-webview);font-family:system-ui}main{min-height:100vh;padding:24px}</style><main><h1>Sandboxed Webview Fixture</h1><p>No Node access. Use only when host primitives do not fit.</p><button>Focusable button</button></main>',
  });
}

function crudCollectionView(ctx: ExtensionContext) {
  const preview = ctx.actions.run('Preview draft', () =>
    ctx.ui.preview({
      id: 'dev-ui-crud-preview',
      title: 'Release notes draft',
      content:
        '# Release notes draft\n\nA generic collection record can open any host-rendered preview.',
    }),
  );
  const edit = ctx.input.prompt({
    title: 'Edit draft',
    fields: [
      {
        id: 'title',
        label: 'Title',
        type: 'text',
        value: 'Release notes draft',
      },
    ],
    action: ctx.actions.run('Save draft', (_innerCtx, action) =>
      ctx.ui.toast({
        message: `Saved ${String(action.formValues?.title || 'draft')}`,
        tone: 'success',
      }),
    ),
  });
  const remove = ctx.ui.confirm({
    title: 'Remove draft',
    message: 'Remove this example record?',
    confirmLabel: 'Remove',
    destructive: true,
    onConfirm: ctx.actions.run('Remove draft', () =>
      ctx.ui.toast({ message: 'Draft removed', tone: 'success' }),
    ),
  });
  return ctx.ui.collection({
    id: 'dev-ui-crud-collection',
    title: 'Dev UI · CRUD Collection',
    subtitle: 'Generic add, preview, edit, and remove composition',
    searchBarPlaceholder: 'Find a record',
    emptyView: {
      title: 'No records',
      subtitle: 'Use Add record to exercise the empty state.',
    },
    add: ctx.actions.run('Add record', () =>
      ctx.ui.toast({ message: 'Add action invoked', tone: 'success' }),
    ),
    items: [
      {
        id: 'release-notes',
        title: 'Release notes draft',
        subtitle: 'A reusable CRUD record fixture',
        icon: 'file-pen-line',
        preview,
        edit,
        remove,
      },
    ],
  });
}

const extension: NevermindExtension = {
  id: 'dev.ui-fixtures',
  title: 'Dev UI Fixtures',
  subtitle: 'Dev-only extension API fixtures',
  capabilities: [
    'camera',
    'desktop.files',
    'desktop.apps',
    'clipboard.history',
    'ocr',
  ],
  actions(ctx) {
    return [
      floatingWindowToggleAction(ctx),
      backgroundJobFixtureAction(ctx),
      folderWatchFixtureAction(ctx),
    ];
  },
  commands: [
    {
      id: 'list',
      title: 'Dev UI: List',
      icon: 'list',
      run: (ctx) => listView(ctx),
    },
    {
      id: 'crud-collection',
      title: 'Dev UI: CRUD Collection',
      icon: 'list-plus',
      run: (ctx) => crudCollectionView(ctx),
    },
    {
      id: 'rendering-polish',
      title: 'Dev UI: Rendering Polish',
      icon: 'sparkles',
      run: (ctx) => renderingPolishView(ctx),
    },
    {
      id: 'grid',
      title: 'Dev UI: Grid',
      icon: 'grid',
      run: (ctx) => gridView(ctx),
    },
    {
      id: 'ocr',
      title: 'Dev UI: OCR',
      icon: 'scan-text',
      run: (ctx) => ocrFixtureView(ctx),
    },
    {
      id: 'preview',
      title: 'Dev UI: Preview',
      icon: 'file-text',
      run: (ctx) => previewView(ctx),
    },
    {
      id: 'form',
      title: 'Dev UI: Form',
      icon: 'list-checks',
      run: (ctx) => formView(ctx),
    },
    {
      id: 'text-input',
      title: 'Dev UI: Text Input',
      icon: 'keyboard',
      run: (ctx) => textInputView(ctx),
    },
    {
      id: 'clipboard',
      title: 'Dev UI: Clipboard',
      icon: 'clipboard',
      run: (ctx) => clipboardView(ctx),
    },
    {
      id: 'folder-watch',
      title: 'Dev UI: Folder Watch',
      icon: 'folder-sync',
      run: (ctx) => folderWatchView(ctx),
    },
    {
      id: 'floating-window',
      title: 'Dev UI: Floating Window',
      icon: 'panel-top-open',
      run: (ctx) => floatingWindowView(ctx),
    },
    {
      id: 'prompt',
      title: 'Dev UI: Prompt',
      icon: 'text-cursor-input',
      run: (ctx) => promptView(ctx),
    },
    {
      id: 'editor',
      title: 'Dev UI: Editor',
      icon: 'file-pen-line',
      run: (ctx) => editorView(ctx),
    },
    {
      id: 'chat',
      title: 'Dev UI: Chat',
      icon: 'message-circle',
      run: (ctx) => chatView(ctx),
    },
    {
      id: 'progress',
      title: 'Dev UI: Progress',
      icon: 'loader',
      run: (ctx) => progressView(ctx),
    },
    {
      id: 'webview',
      title: 'Dev UI: Webview',
      icon: 'globe',
      run: (ctx) => webviewView(ctx),
    },
    {
      id: 'camera',
      title: 'Dev UI: Camera',
      icon: 'camera',
      run: (ctx) =>
        ctx.ui.camera({
          id: 'dev-ui-camera',
          title: 'Dev UI · Camera',
          showDeviceSwitcher: true,
          controls: true,
        }),
    },
  ],
};

export default extension;
