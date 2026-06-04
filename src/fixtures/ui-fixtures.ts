import type { NevermindExtension, ExtensionAction, ExtensionContext, ExtensionFile, ExtensionFormValue } from '../resources/nevermind-extension-api'

function valuesMarkdown(values: Record<string, ExtensionFormValue> = {}) {
  const rows = Object.entries(values).map(([key, value]) => `- **${key}**: ${Array.isArray(value) ? value.join(', ') : String(value)}`)
  return rows.length ? rows.join('\n') : 'No values submitted.'
}

function formView(ctx: ExtensionContext) {
  return ctx.ui.form({
    id: 'dev-ui-form',
    title: 'Dev UI · Form Fields',
    subtitle: 'Exercises every host-rendered form field type',
    fields: [
      { id: 'intro', type: 'description', description: 'This dev-only extension is loaded only in electron-vite dev mode.' },
      { id: 'name', label: 'Text', type: 'text', value: 'Nevermind', placeholder: 'Name', required: true, description: 'Plain single-line text.' },
      { id: 'notes', label: 'Textarea', type: 'textarea', value: 'Longer editable text', rows: 5, description: 'Multi-line input for notes, templates, and prompts.' },
      { id: 'password', label: 'Password', type: 'password', placeholder: 'Secret' },
      { id: 'url', label: 'URL', type: 'url', value: 'https://nvm.fyi' },
      { id: 'number', label: 'Number', type: 'number', value: '42' },
      { id: 'date', label: 'Date', type: 'date' },
      { id: 'enabled', label: 'Checkbox', type: 'checkbox', value: true, description: 'Boolean form value.' },
      { id: 'separator', type: 'separator' },
      { id: 'color', label: 'Dropdown', type: 'dropdown', value: 'blue', options: [{ title: 'Blue', value: 'blue' }, { title: 'Green', value: 'green' }, { title: 'Purple', value: 'purple' }] },
      { id: 'tags', label: 'Multiselect', type: 'multiselect', value: ['templates', 'forms'], options: [{ title: 'Templates', value: 'templates' }, { title: 'Forms', value: 'forms' }, { title: 'Files', value: 'files' }] },
      { id: 'inputFile', label: 'File picker', type: 'file', placeholder: 'Choose one source file', description: 'Single path value from the host native file picker.' },
      { id: 'inputFiles', label: 'Files picker', type: 'files', placeholder: 'Choose one or more images', extensions: ['png', 'jpg', 'jpeg', 'webp'], filterName: 'Images', description: 'Array value from a multi-select native file picker.' },
      { id: 'outputFolder', label: 'Folder picker', type: 'folder', placeholder: 'Choose an output folder', description: 'Folder path value from the host native folder picker.' },
      { id: 'invalid', label: 'Error State', type: 'text', value: 'Bad value', error: 'Example field-level error copy.' },
    ],
    submitAction: ctx.actions.run('Show Submitted Values', (_ctx, action) => ctx.ui.preview({
      title: 'Submitted Form Values',
      content: `# Submitted Form Values\n\n${valuesMarkdown(action.formValues)}`,
    })),
  })
}

const FLOATING_WINDOW_ID = 'dev-ui-floating-window'
const FLOATING_WINDOW_OPTIONS = { id: FLOATING_WINDOW_ID, title: 'Nevermind Floating Note', titleBar: 'hidden' as const, chrome: 'none' as const, width: 560, height: 520, alwaysOnTop: true, visibleOnAllSpaces: true }

function floatingNoteEditorView(ctx: ExtensionContext) {
  return ctx.ui.editor({
    id: FLOATING_WINDOW_ID,
    title: 'Floating Note',
    subtitle: 'Editable host-rendered note',
    format: 'text',
    placeholder: 'Write a floating note…',
    content: 'Floating Note\n\nEdit this note in a real independent window.\n\n- Always on top\n- Reuses ctx.ui.editor(...)\n- No palette chrome, preview, or action menu',
  })
}

function floatingWindowToggleAction(ctx: ExtensionContext) {
  return ctx.action({
    id: 'toggle-floating-note',
    title: 'Toggle Floating Note',
    subtitle: 'Persistent action: creates the note if needed, then shows or hides it',
    icon: 'panel-top-open',
    keywords: ['window', 'note', 'shortcut', 'persistent action'],
    action: ctx.windows.toggle(floatingNoteEditorView(ctx), FLOATING_WINDOW_OPTIONS),
  })
}

function floatingWindowToggleItem(ctx: ExtensionContext) {
  return ctx.ui.item({
    id: 'toggle-floating-note',
    title: 'Toggle Floating Note',
    subtitle: 'References the persistent action, so aliases and shortcuts attach to the durable action id',
    icon: 'panel-top-open',
    keywords: ['window', 'note', 'shortcut', 'persistent action'],
    primaryAction: ctx.actions.ref('toggle-floating-note', 'Toggle Floating Note'),
  })
}

function floatingWindowView(ctx: ExtensionContext) {
  return ctx.ui.list({ 
    id: 'dev-ui-floating-window-controls',
    title: 'Dev UI · Floating Window',
    subtitle: 'Open, toggle, and close an independent host-rendered extension window',
    items: [
      floatingWindowToggleItem(ctx),
      ctx.ui.item({ id: 'close', title: 'Close Floating Note', subtitle: 'Closes the existing window', icon: 'x', primaryAction: ctx.windows.close(FLOATING_WINDOW_ID, 'Close Floating Note') }),
    ],
  })
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
        subtitle: 'Paste into the frontmost app without keeping this text in clipboard history',
        icon: 'clipboard-paste',
        primaryAction: ctx.actions.pasteText('Nevermind concealed restored paste', 'Paste Concealed + Restore', { restoreClipboard: true, concealed: true, dismissAfterRun: 'auto' }),
        actions: [ctx.actions.pasteText('Nevermind concealed restored paste', 'Paste Concealed + Restore', { restoreClipboard: true, concealed: true, dismissAfterRun: 'auto' })],
      }),
      ctx.ui.item({
        id: 'paste-keep-open',
        title: 'Paste text and keep palette open',
        subtitle: 'Useful for repeated snippet insertion tests',
        icon: 'panel-top-open',
        primaryAction: ctx.actions.pasteText('Nevermind keep-open paste', 'Paste and Keep Open', { keepPaletteOpen: true, dismissAfterRun: 'auto' }),
        actions: [ctx.actions.pasteText('Nevermind keep-open paste', 'Paste and Keep Open', { keepPaletteOpen: true, dismissAfterRun: 'auto' })],
      }),
      ctx.ui.item({
        id: 'type-text',
        title: 'Type text without clipboard',
        subtitle: 'Uses host keyboard typing when supported',
        icon: 'keyboard',
        primaryAction: ctx.actions.typeText('Nevermind typed text', 'Type Text', { dismissAfterRun: 'auto' }),
        actions: [ctx.actions.typeText('Nevermind typed text', 'Type Text', { dismissAfterRun: 'auto' })],
      }),
    ],
  })
}

function promptView(ctx: ExtensionContext) {
  const prompt = ctx.input.prompt({ 
    title: 'Create Quicklink URL',
    message: 'Prompt for lightweight arguments before running an action.',
    fields: [
      { id: 'query', label: 'Search query', type: 'text', placeholder: 'never mind extension api', required: true },
      { id: 'site', label: 'Site', type: 'dropdown', value: 'github.com', options: [{ title: 'GitHub', value: 'github.com' }, { title: 'Docs', value: 'docs' }, { title: 'Web', value: 'web' }] },
    ],
    submitTitle: 'Build URL',
    action: ctx.actions.run('Show Prompt Values', (_ctx, action) => {
      const query = String(action.formValues?.query || '')
      const site = String(action.formValues?.site || '')
      const url = `https://www.google.com/search?q=${encodeURIComponent(site && site !== 'web' ? `site:${site} ${query}` : query)}`
      return _ctx.ui.preview({ title: 'Prompt Result', content: `# Prompt Result\n\n- Query: ${query || '_empty_'}\n- Site: ${site || '_empty_'}\n- URL: ${url}` })
    }),
  })
  return ctx.ui.list({
    id: 'dev-ui-prompt',
    title: 'Dev UI · Prompt',
    subtitle: 'Host-owned lightweight argument prompt before an action runs',
    items: [ctx.ui.item({ id: 'prompt', title: 'Prompt for Quicklink Arguments', subtitle: 'Opens a form and then runs the wrapped action', icon: 'text-cursor-input', primaryAction: prompt, actions: [prompt] })],
  })
}

function editorView(ctx: ExtensionContext) {
  return ctx.ui.editor({ 
    id: 'dev-ui-editor',
    title: 'Dev UI · Editor',
    subtitle: 'Editable markdown with a host-rendered preview and submit action',
    format: 'markdown',
    placeholder: 'Write markdown…',
    content: '# Release Note Draft\n\n- Built with host-owned editor UI.\n- Supports **markdown preview**.\n- Submit injects `editorContent` into the action.',
    submitAction: ctx.actions.run('Preview Draft', (_ctx, action) => ctx.ui.preview({
      title: 'Submitted Editor Content',
      content: `# Submitted Editor Content\n\n${action.editorContent || '_Empty draft_'}`,
    })),
    actions: [ctx.actions.copyText('Copied from editor fixture', 'Copy Sample Text')],
  })
}

function fileIndexControlsAction(ctx: ExtensionContext) {
  return ctx.actions.run('Show File Index Snapshot', async (innerCtx) => {
    const files = innerCtx.desktop.files
    if (!files) return innerCtx.ui.preview({ title: 'File Index Unavailable', content: 'The `desktop.files` permission is not available.' })
    const roots = files.indexedRoots()
    const snapshot = files.indexSnapshot({ roots: ['~/Downloads'], extensions: ['png', 'jpg', 'jpeg', 'mp4', 'mov'], limit: 8, ignore: ['*.tmp'] })
    return innerCtx.ui.preview({
      title: 'File Index Snapshot',
      content: [`# File Index Snapshot`, '', `Default roots: ${roots.join(', ')}`, '', ...snapshot.map((file) => `- **${file.name}** · ${file.kind || 'file'} · ${file.displayPath || file.path}`)].join('\n'),
    })
  })
}

function reindexDownloadsAction(ctx: ExtensionContext) {
  return ctx.actions.run('Reindex Downloads Media', async (innerCtx) => {
    const files = innerCtx.desktop.files
    if (!files) return innerCtx.ui.preview({ title: 'File Index Unavailable', content: 'The `desktop.files` permission is not available.' })
    const result = await files.reindex({ roots: ['~/Downloads'], kind: 'media', depth: 2, limit: 500, ignore: ['*.tmp'] })
    return innerCtx.ui.preview({ title: 'Reindex Complete', content: `# Reindex Complete\n\nIndexed ${result.count} files from ${result.roots.join(', ')}.` })
  })
}

function listView(ctx: ExtensionContext) {
  const confirm = ctx.ui.confirm({
    title: 'Confirm Dev Action',
    message: 'This confirms host-owned action UI still renders correctly.',
    confirmLabel: 'Show Toast',
    onConfirm: ctx.actions.run('Show Toast', () => ctx.ui.toast({ message: 'Confirmed from dev UI fixture' })),
  })
  return ctx.ui.list({
    id: 'dev-ui-list',
    title: 'Dev UI · List',
    subtitle: 'List rows, accessories, sections, action panels, confirmation, and navigation',
    searchBarPlaceholder: 'Filter fixture rows',
    sections: [{
      title: 'Rows',
      items: [
        ctx.ui.item({ id: 'form', title: 'Open Form Fixture', subtitle: 'Textarea, dropdowns, errors, descriptions', icon: 'list-checks', accessories: [{ text: 'form' }], primaryAction: ctx.actions.push('Open Form', formView(ctx)) }),
        ctx.ui.item({ id: 'text-input', title: 'Open Text Input Fixture', subtitle: 'Paste/type actions for snippets and transforms', icon: 'keyboard', accessories: [{ text: 'text' }], primaryAction: ctx.actions.push('Open Text Input', textInputView(ctx)) }),
        ctx.ui.item({ id: 'floating-window', title: 'Open Floating Window Fixture', subtitle: 'Independent host-rendered extension window', icon: 'panel-top-open', accessories: [{ text: 'window' }], primaryAction: ctx.actions.push('Open Floating Window', floatingWindowView(ctx)) }),
        ctx.ui.item({ id: 'prompt', title: 'Open Prompt Fixture', subtitle: 'Prompted arguments before an action runs', icon: 'text-cursor-input', accessories: [{ text: 'prompt' }], primaryAction: ctx.actions.push('Open Prompt', promptView(ctx)) }),
        ctx.ui.item({ id: 'editor', title: 'Open Editor Fixture', subtitle: 'Editable markdown, preview, submit payload', icon: 'file-pen-line', accessories: [{ text: 'editor' }], primaryAction: ctx.actions.push('Open Editor', editorView(ctx)) }),
        ctx.ui.item({ id: 'preview', title: 'Open Preview Fixture', subtitle: 'Markdown/text preview', icon: 'file-text', accessories: [{ text: 'preview' }], primaryAction: ctx.actions.push('Open Preview', previewView(ctx)) }),
        ctx.ui.item({ id: 'file-index', title: 'File Index Controls', subtitle: 'Snapshot and bounded reindex helpers for generated file searchers', icon: 'folder-search', accessories: [{ text: 'files' }], primaryAction: fileIndexControlsAction(ctx), actions: [fileIndexControlsAction(ctx), reindexDownloadsAction(ctx)] }),
        ctx.ui.item({ id: 'confirm', title: 'Confirmation Fixture', subtitle: 'Host-owned confirm step', icon: 'shield-check', accessories: [{ text: 'confirm' }], primaryAction: confirm, actionPanel: { sections: [{ actions: [confirm, ctx.actions.copyText('copied from dev fixture', 'Copy Fixture Text')] }] } }),
      ],
    }],
  })
}

function fileMetadataMarkdown(file: ExtensionFile) {
  return [`# ${file.name}`, '', `- Path: ${file.displayPath || file.path}`, `- Kind: ${file.kind || 'file'}`, `- MIME: ${file.mimeType || 'unknown'}`, `- Size: ${file.size || 0} bytes`, file.width && file.height ? `- Dimensions: ${file.width} × ${file.height}` : '', file.mtime ? `- Modified: ${file.mtime}` : ''].filter(Boolean).join('\n')
}

async function gridView(ctx: ExtensionContext) {
  const media = ctx.desktop.files ? await ctx.desktop.files.findMedia(['~/Pictures', '~/Desktop', '~/Downloads'], { limit: 6, depth: 2, sortBy: 'recent' }) : []
  if (media.length) {
    const files = await Promise.all(media.map((file) => ctx.desktop.files!.metadata(file.path)))
    return ctx.ui.grid({
      id: 'dev-ui-grid',
      title: 'Dev UI · Grid',
      subtitle: 'Grid tiles using ctx.desktop.files.metadata(...) and thumbnail(...)',
      layout: 'wide',
      aspectRatio: '16 / 9',
      columns: 3,
      sections: [{
        title: 'Recent Media',
        items: files.map((file) => ({
          id: file.path,
          title: file.name,
          subtitle: file.width && file.height ? `${file.kind} · ${file.width} × ${file.height}` : file.displayPath,
          icon: file.kind === 'video' ? 'video' : 'image',
          image: ctx.desktop.files!.thumbnail(file.path) || file.url,
          accessories: [{ text: file.extension || file.kind || 'file' }],
          primaryAction: ctx.actions.push('Show Metadata', ctx.ui.preview({ title: file.name, content: fileMetadataMarkdown(file), image: file.thumbnailUrl || file.url })),
          actions: [ctx.actions.revealPath(file.path, 'Reveal File'), ctx.actions.copyText(file.path, 'Copy Path')],
        })),
      }],
    })
  }

  const colors = ['yellow', 'blue', 'purple', 'green', 'red', 'orange'] as const
  return ctx.ui.grid({
    id: 'dev-ui-grid',
    title: 'Dev UI · Grid',
    subtitle: 'Grid tiles, sections, layout, aspect ratio, and action hints',
    layout: 'wide',
    aspectRatio: '16 / 9',
    columns: 3,
    sections: [{
      title: 'Fallback Tiles',
      items: colors.map((color) => ({
        id: color,
        title: `${color[0].toUpperCase()}${color.slice(1)} Tile`,
        subtitle: 'Generated placeholder tile; add media to Pictures/Desktop/Downloads to exercise file helpers',
        icon: 'image',
        image: `data:image/svg+xml,${encodeURIComponent(`<svg xmlns="http://www.w3.org/2000/svg" width="640" height="360"><rect width="100%" height="100%" rx="28" fill="${color}"/><text x="50%" y="54%" text-anchor="middle" font-family="system-ui" font-size="42" fill="black">${color}</text></svg>`)}`,
        appearance: { foreground: color },
        primaryAction: ctx.actions.copyText(color, `Copy ${color}`),
        actions: [ctx.actions.copyText(color, `Copy ${color}`)],
      })),
    }],
  })
}

function previewView(ctx: ExtensionContext) {
  const templateAction = ctx.actions.run('Expand Template', async (innerCtx) => {
    const output = await innerCtx.text.template('Today is {date} at {time}. 6 * 7 = {calculator:6*7}. Selection: {selectedText}', { name: 'Nevermind' })
    return innerCtx.ui.preview({ title: 'Template Output', content: `# Template Output\n\n${output}` })
  })
  return ctx.ui.preview({
    id: 'dev-ui-preview',
    title: 'Dev UI · Preview',
    content: '# Preview Fixture\n\nThis exercises markdown/text preview, action panels, and `ctx.text.template(...)`.',
    actions: [templateAction],
    actionPanel: { sections: [{ actions: [templateAction] }] },
  })
}

function chatView(ctx: ExtensionContext) {
  return ctx.ui.chat({
    id: 'dev-ui-chat',
    title: 'Dev UI · Chat',
    messages: [
      { role: 'system', content: 'Static chat fixture.' },
      { role: 'user', content: 'Can generated extensions render chat bubbles?' },
      { role: 'assistant', content: 'Yes — this is a host-rendered chat view.' },
    ],
  })
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
  })
}

function webviewView(ctx: ExtensionContext) {
  return ctx.ui.webview({
    id: 'dev-ui-webview',
    title: 'Dev UI · Webview',
    html: '<main style="font-family: system-ui; color: white; padding: 24px"><h1>Sandboxed Webview Fixture</h1><p>No Node access. Use only when host primitives do not fit.</p><button>Focusable button</button></main>',
  })
}

const extension: NevermindExtension = {
  id: 'dev.ui-fixtures',
  title: 'Dev UI Fixtures',
  subtitle: 'Dev-only extension API fixtures',
  permissions: ['camera', 'desktop.files'],
  actions(ctx) {
    return [floatingWindowToggleAction(ctx)]
  },
  commands: [
    { id: 'list', title: 'Dev UI: List', icon: 'list', run: (ctx) => listView(ctx) },
    { id: 'grid', title: 'Dev UI: Grid', icon: 'grid', run: (ctx) => gridView(ctx) },
    { id: 'preview', title: 'Dev UI: Preview', icon: 'file-text', run: (ctx) => previewView(ctx) },
    { id: 'form', title: 'Dev UI: Form', icon: 'list-checks', run: (ctx) => formView(ctx) },
    { id: 'text-input', title: 'Dev UI: Text Input', icon: 'keyboard', run: (ctx) => textInputView(ctx) },
    { id: 'floating-window', title: 'Dev UI: Floating Window', icon: 'panel-top-open', run: (ctx) => floatingWindowView(ctx) },
    { id: 'prompt', title: 'Dev UI: Prompt', icon: 'text-cursor-input', run: (ctx) => promptView(ctx) },
    { id: 'editor', title: 'Dev UI: Editor', icon: 'file-pen-line', run: (ctx) => editorView(ctx) },
    { id: 'chat', title: 'Dev UI: Chat', icon: 'message-circle', run: (ctx) => chatView(ctx) },
    { id: 'progress', title: 'Dev UI: Progress', icon: 'loader', run: (ctx) => progressView(ctx) },
    { id: 'webview', title: 'Dev UI: Webview', icon: 'globe', run: (ctx) => webviewView(ctx) },
    { id: 'camera', title: 'Dev UI: Camera', icon: 'camera', run: (ctx) => ctx.ui.camera({ id: 'dev-ui-camera', title: 'Dev UI · Camera', showDeviceSwitcher: true, controls: true }) },
  ],
}

export default extension
