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

test('legacy ui.preview keeps unique-shape routing for items, files, and plain views', () => {
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

const HELLO_TEXT_PREVIEW = {
  kind: 'text',
  text: 'hello',
  truncated: false,
  sizeBytes: 'hello'.length,
} as const;

function createUiWithTextReader() {
  const reads: string[] = [];
  const ui = createExtensionUiApi({
    buildPreviewItemAction: (item) => ({ previewItem: item }),
    readTextFilePreview: (filePath) => {
      reads.push(filePath);
      return Promise.resolve(HELLO_TEXT_PREVIEW);
    },
    progressView: (input) => ({ type: 'progress', ...(input as object) }),
    buildConfirmAction: (input) => ({ type: 'confirm', input }),
  });
  return { ui, reads };
}

const TEXT_FILE = {
  path: '/tmp/notes.md',
  name: 'notes.md',
  displayPath: '~/notes.md',
  fileUrl: 'nvm-file://notes',
  url: 'nvm-file://notes',
  kind: 'file',
};
const IMAGE_FILE = {
  path: '/tmp/shot.png',
  name: 'shot.png',
  displayPath: '~/shot.png',
  url: 'nvm-thumb://shot',
  thumbnailUrl: 'nvm-thumb://shot',
  kind: 'image',
};

test('previewView stamps an explicit preview view', () => {
  const ui = createUi();
  assert.deepEqual(ui.previewView({ title: 'Plain', kind: 'image' }), {
    title: 'Plain',
    kind: 'image',
    type: 'preview',
  });
});

test('previewFile renders bounded text contents for non-media files', async () => {
  const { ui, reads } = createUiWithTextReader();
  const view = await ui.previewFile(TEXT_FILE);
  assert.deepEqual(reads, ['/tmp/notes.md']);
  assert.equal(view.type, 'preview');
  assert.equal(view.title, 'notes.md');
  assert.equal(view.subtitle, '~/notes.md');
  assert.equal(view.content, '```md\nhello\n```');
  assert.equal(view.image, undefined);
  assert.deepEqual(structuredClone(view), view);
});

test('previewFile keeps media files and explicit content without reading', async () => {
  const { ui, reads } = createUiWithTextReader();
  const media = await ui.previewFile(IMAGE_FILE);
  assert.equal(media.image, 'nvm-thumb://shot');
  const custom = await ui.previewFile(TEXT_FILE, { content: 'Metadata' });
  assert.equal(custom.content, 'Metadata');
  assert.deepEqual(reads, []);
});

test('previewFile falls back to the path when text reading is unavailable', async () => {
  const ui = createUi();
  const view = await ui.previewFile(TEXT_FILE);
  assert.equal(view.content, '~/notes.md');
});

test('previewFile rejects inputs that are not files', async () => {
  const ui = createUi();
  await assert.rejects(
    () => ui.previewFile({ kind: 'image', title: 'Clipboard' }),
    TypeError,
  );
});

test('previewAction maps files and descriptors to clone-safe inline actions', () => {
  const ui = createUi();
  const fromFile = ui.previewAction(IMAGE_FILE, { title: 'Preview' });
  assert.deepEqual(fromFile, {
    previewItem: {
      kind: 'image',
      title: 'Preview',
      shortcut: undefined,
      filePath: '/tmp/shot.png',
      videoUrl: undefined,
      thumbnailUrl: 'nvm-thumb://shot',
    },
  });
  assert.deepEqual(ui.previewAction({ kind: 'text', text: 'Copied' }), {
    previewItem: { kind: 'text', text: 'Copied' },
  });
  assert.deepEqual(structuredClone(fromFile), fromFile);
});

test('extension ui api keeps utility helpers stable', () => {
  const ui = createUi();

  assert.deepEqual(ui.camera({ title: 'Desk', muted: false }), {
    title: 'Desk',
    size: 'large',
    muted: false,
    type: 'camera',
  });
  assert.deepEqual(
    ui.camera({ title: 'Mirror', mirrored: true, framing: 'contain' }),
    {
      title: 'Mirror',
      size: 'large',
      muted: true,
      mirrored: true,
      framing: 'contain',
      type: 'camera',
    },
  );
  assert.deepEqual(ui.empty('Empty', 'Try again'), {
    type: 'preview',
    title: 'Empty',
    content: '# Empty\n\nTry again',
  });
  assert.deepEqual(ui.loading('Loading files'), {
    type: 'list',
    id: 'extension-loading:Loading files',
    title: 'Loading files',
    searchBarPlaceholder: 'Loading files',
    contentSizing: 'fit',
    isLoading: true,
    items: [],
  });
  const error = ui.error('Uninstall unavailable', 'Try another app.');
  assert.equal(error.type, 'list');
  assert.equal(error.items[0].disabled, true);
  assert.equal(error.items[0].subtitle, 'Try another app.');
  assert.equal(error.items[1].primaryAction.type, 'popView');
});

test('indicator helpers delegate passive status lifecycle to the host', () => {
  const calls: [string, unknown][] = [];
  const ui = createExtensionUiApi({
    buildPreviewItemAction: (item) => ({ previewItem: item }),
    progressView: (input) => ({ type: 'progress', ...(input as object) }),
    buildConfirmAction: (input) => ({ type: 'confirm', input }),
    showIndicator: (input) => calls.push(['show', input]),
    updateIndicator: (input) => calls.push(['update', input]),
    hideIndicator: (id) => calls.push(['hide', id]),
  });

  ui.indicator.show({ id: 'dictation', title: 'Listening' });
  ui.indicator.update({ id: 'dictation', title: 'Transcribing' });
  ui.indicator.hide('dictation');
  assert.equal(
    (ui as unknown as { toast(input: unknown): unknown }).toast({
      message: 'Legacy feedback',
      tone: 'error',
    }),
    undefined,
  );
  assert.deepEqual(calls, [
    ['show', { id: 'dictation', title: 'Listening' }],
    ['update', { id: 'dictation', title: 'Transcribing' }],
    ['hide', 'dictation'],
    [
      'show',
      {
        id: 'feedback',
        title: 'Nevermind',
        subtitle: 'Legacy feedback',
        status: 'error',
        durationMs: 4000,
      },
    ],
  ]);
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
