import assert from 'node:assert/strict';
import test from 'node:test';
import { createExtensionUiApi } from './extension-ui-api';

function createUi() {
  return createExtensionUiApi({
    buildPreviewItemAction: (item) => ({ previewItem: item }),
    progressView: (input) => ({ type: 'progress', ...(input as object) }),
    buildConfirmAction: (input) => ({ type: 'confirm', input }),
  });
}

test('extension ui api stamps view helper types', () => {
  const ui = createUi();

  assert.deepEqual(ui.list({ title: 'List' }), { title: 'List', type: 'list' });
  assert.deepEqual(ui.grid({ title: 'Grid' }), { title: 'Grid', type: 'grid' });
  assert.deepEqual(ui.chat({ title: 'Chat' }), { title: 'Chat', type: 'chat' });
  assert.deepEqual(ui.webview({ title: 'HTML' }), {
    title: 'HTML',
    type: 'webview',
  });
});

test('extension ui preview handles previewable items, file metadata, and plain views', () => {
  const ui = createUi();

  assert.deepEqual(ui.preview({ kind: 'image', title: 'Image' }), {
    previewItem: { kind: 'image', title: 'Image' },
  });
  assert.deepEqual(
    ui.preview({
      name: 'Report',
      displayPath: '~/Report.pdf',
      fileUrl: 'file://report',
      thumbnailUrl: 'thumb://report',
    }),
    {
      type: 'preview',
      presentation: 'preview',
      title: 'Report',
      subtitle: '~/Report.pdf',
      content: '~/Report.pdf',
      image: 'thumb://report',
      video: undefined,
    },
  );
  assert.deepEqual(ui.preview({ title: 'Plain', content: 'Markdown' }), {
    title: 'Plain',
    content: 'Markdown',
    type: 'preview',
  });
});

test('extension ui api keeps utility helpers stable', () => {
  const ui = createUi();

  assert.deepEqual(ui.camera({ title: 'Desk', muted: false }), {
    title: 'Desk',
    size: 'large',
    muted: false,
    type: 'camera',
  });
  assert.deepEqual(ui.empty('Empty', 'Try again'), {
    type: 'preview',
    title: 'Empty',
    content: '# Empty\n\nTry again',
  });
  assert.deepEqual(ui.loading('Loading files'), {
    type: 'progress',
    title: 'Loading files',
    label: 'Loading files',
  });
  assert.deepEqual(ui.toast({ message: 123, tone: 'error' }), {
    toast: { message: '123', tone: 'error' },
  });
  const error = ui.error('Uninstall unavailable', 'Try another app.');
  assert.equal(error.type, 'list');
  assert.equal(error.items[0].disabled, true);
  assert.equal(error.items[0].subtitle, 'Try another app.');
  assert.equal(error.items[1].primaryAction.type, 'popView');
});

test('extension ui collection gives records consistent CRUD actions', () => {
  const ui = createUi();
  const add = { title: 'Add task' };
  const preview = { title: 'Preview task' };
  const edit = { title: 'Edit task' };
  const remove = { title: 'Remove task', style: 'destructive' };
  const view = ui.collection({
    id: 'tasks',
    title: 'Tasks',
    add,
    items: [{ id: 'one', title: 'Write API', preview, edit, remove }],
  });
  assert.equal(view.type, 'list');
  assert.deepEqual(view.actions, [add]);
  assert.equal(view.items[0].title, 'Add task');
  assert.equal(view.items[0].subtitle, 'Create a new item');
  assert.equal(view.items[0].primaryAction, add);
  assert.deepEqual(view.items[0].actions, [add]);
  assert.equal(view.items[1].primaryAction, preview);
  assert.deepEqual(view.items[1].actions, [preview, edit, remove]);
});
