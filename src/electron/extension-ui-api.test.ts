import assert from 'node:assert/strict'
import test from 'node:test'
import { createExtensionUiApi } from './extension-ui-api'

function createUi() {
  return createExtensionUiApi({
    buildPreviewItemAction: (item) => ({ previewItem: item }),
    progressView: (input) => ({ type: 'progress', ...(input as object) }),
    buildConfirmAction: (input) => ({ type: 'confirm', input }),
  })
}

test('extension ui api stamps view helper types', () => {
  const ui = createUi()

  assert.deepEqual(ui.list({ title: 'List' }), { title: 'List', type: 'list' })
  assert.deepEqual(ui.grid({ title: 'Grid' }), { title: 'Grid', type: 'grid' })
  assert.deepEqual(ui.chat({ title: 'Chat' }), { title: 'Chat', type: 'chat' })
  assert.deepEqual(ui.webview({ title: 'HTML' }), { title: 'HTML', type: 'webview' })
})

test('extension ui preview handles previewable items, file metadata, and plain views', () => {
  const ui = createUi()

  assert.deepEqual(ui.preview({ kind: 'image', title: 'Image' }), { previewItem: { kind: 'image', title: 'Image' } })
  assert.deepEqual(ui.preview({ name: 'Report', displayPath: '~/Report.pdf', fileUrl: 'file://report', thumbnailUrl: 'thumb://report' }), {
    type: 'preview',
    presentation: 'preview',
    title: 'Report',
    subtitle: '~/Report.pdf',
    content: '~/Report.pdf',
    image: 'thumb://report',
    video: undefined,
  })
  assert.deepEqual(ui.preview({ title: 'Plain', content: 'Markdown' }), { title: 'Plain', content: 'Markdown', type: 'preview' })
})

test('extension ui api keeps utility helpers stable', () => {
  const ui = createUi()

  assert.deepEqual(ui.camera({ title: 'Desk', muted: false }), { title: 'Desk', size: 'large', muted: false, type: 'camera' })
  assert.deepEqual(ui.empty('Empty', 'Try again'), { type: 'preview', title: 'Empty', content: '# Empty\n\nTry again' })
  assert.deepEqual(ui.loading('Loading files'), { type: 'progress', title: 'Loading files', label: 'Loading files' })
  assert.deepEqual(ui.toast({ message: 123, tone: 'error' }), { toast: { message: '123', tone: 'error' } })
})
