import assert from 'node:assert/strict';
import test from 'node:test';
import { createExtensionUiApi } from './extension-ui-api';

const RESERVED_PREFIX_ERROR = /must not start with/;

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
    windowPresentation: 'compact',
    add,
    items: [{ id: 'one', title: 'Write API', preview, edit, remove }],
  });
  assert.equal(view.type, 'list');
  assert.equal(view.windowPresentation, 'compact');
  assert.deepEqual(view.actions, [add]);
  assert.equal(view.items[0].id, '__nvm:collection-create');
  assert.equal(view.items[0].title, 'Add task');
  assert.equal(view.items[0].icon, 'plus');
  assert.equal(view.items[0].primaryAction, add);
  assert.deepEqual(view.items[0].actions, [add]);

  const record = view.items[1];
  assert.equal(record.primaryAction, preview);
  const [recordPreview, recordEdit, recordRemove] = record.actions;
  assert.equal(recordPreview, preview);
  assert.equal(recordEdit.shortcut, 'Command+E');
  assert.equal(recordEdit.title, 'Edit task');
  assert.equal(recordRemove.shortcut, 'Command+Backspace');
  assert.equal(recordRemove.requiresConfirmation, true);
  assert.equal(recordRemove.confirmMessage, 'Remove “Write API”?');
  assert.equal(recordRemove.confirmLabel, 'Remove');
  assert.equal(recordRemove.style, 'destructive');
});

test('extension ui collection respects explicit action overrides', () => {
  const ui = createUi();
  const edit = { title: 'Rename', shortcut: 'Command+R' };
  const remove = {
    title: 'Delete forever',
    confirmMessage: 'Custom prompt',
    requiresConfirmation: false,
  };
  const view = ui.collection({
    title: 'Notes',
    items: [{ id: 'one', title: 'Note', edit, remove }],
  });
  const [recordEdit, recordRemove] = view.items[0].actions;
  assert.equal(recordEdit.shortcut, 'Command+R');
  assert.equal(view.items[0].primaryAction, recordEdit);
  assert.equal(recordRemove.confirmMessage, 'Custom prompt');
  assert.equal(recordRemove.requiresConfirmation, false);
});

test('extension ui collection rejects reserved item id prefixes', () => {
  const ui = createUi();
  assert.throws(
    () =>
      ui.collection({
        title: 'Notes',
        items: [{ id: '__nvm:collection-create', title: 'Collision' }],
      }),
    RESERVED_PREFIX_ERROR,
  );
});
