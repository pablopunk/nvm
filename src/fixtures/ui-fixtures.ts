import type { NevermindExtension, ExtensionAction, ExtensionContext, ExtensionFormValue } from '../resources/nevermind-extension-api'

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
      { id: 'invalid', label: 'Error State', type: 'text', value: 'Bad value', error: 'Example field-level error copy.' },
    ],
    submitAction: ctx.actions.run('Show Submitted Values', (_ctx, action) => ctx.ui.preview({
      title: 'Submitted Form Values',
      content: `# Submitted Form Values\n\n${valuesMarkdown(action.formValues)}`,
    })),
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
        ctx.ui.item({ id: 'preview', title: 'Open Preview Fixture', subtitle: 'Markdown/text preview', icon: 'file-text', accessories: [{ text: 'preview' }], primaryAction: ctx.actions.push('Open Preview', previewView(ctx)) }),
        ctx.ui.item({ id: 'confirm', title: 'Confirmation Fixture', subtitle: 'Host-owned confirm step', icon: 'shield-check', accessories: [{ text: 'confirm' }], primaryAction: confirm, actionPanel: { sections: [{ actions: [confirm, ctx.actions.copyText('copied from dev fixture', 'Copy Fixture Text')] }] } }),
      ],
    }],
  })
}

function gridView(ctx: ExtensionContext) {
  const colors = ['yellow', 'blue', 'purple', 'green', 'red', 'orange'] as const
  return ctx.ui.grid({
    id: 'dev-ui-grid',
    title: 'Dev UI · Grid',
    subtitle: 'Grid tiles, sections, layout, aspect ratio, and action hints',
    layout: 'wide',
    aspectRatio: '16 / 9',
    columns: 3,
    sections: [{
      title: 'Tiles',
      items: colors.map((color) => ({
        id: color,
        title: `${color[0].toUpperCase()}${color.slice(1)} Tile`,
        subtitle: 'Generated placeholder tile',
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
  permissions: ['camera'],
  commands: [
    { id: 'list', title: 'Dev UI: List', icon: 'list', run: (ctx) => listView(ctx) },
    { id: 'grid', title: 'Dev UI: Grid', icon: 'grid', run: (ctx) => gridView(ctx) },
    { id: 'preview', title: 'Dev UI: Preview', icon: 'file-text', run: (ctx) => previewView(ctx) },
    { id: 'form', title: 'Dev UI: Form', icon: 'list-checks', run: (ctx) => formView(ctx) },
    { id: 'chat', title: 'Dev UI: Chat', icon: 'message-circle', run: (ctx) => chatView(ctx) },
    { id: 'progress', title: 'Dev UI: Progress', icon: 'loader', run: (ctx) => progressView(ctx) },
    { id: 'webview', title: 'Dev UI: Webview', icon: 'globe', run: (ctx) => webviewView(ctx) },
    { id: 'camera', title: 'Dev UI: Camera', icon: 'camera', run: (ctx) => ctx.ui.camera({ id: 'dev-ui-camera', title: 'Dev UI · Camera', showDeviceSwitcher: true, controls: true }) },
  ],
}

export default extension
