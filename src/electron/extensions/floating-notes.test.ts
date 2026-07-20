import assert from 'node:assert/strict';
import test from 'node:test';
import { createFloatingNotesExtension } from './floating-notes';

function createContext() {
  const values = new Map<string, unknown>();
  const ctx = {
    storage: {
      get: async (key: string, fallback: unknown) =>
        values.get(key) ?? fallback,
      set: async (key: string, value: unknown) => {
        values.set(key, value);
        return value;
      },
    },
    ui: {
      list: (view: any) => ({ ...view, type: 'list' }),
      collection: (view: any) => ({ ...view, type: 'list' }),
      editor: (view: any) => ({ ...view, type: 'editor' }),
      toast: (toast: any) => ({ toast }),
    },
    windows: {
      create: (view: unknown, options: unknown) => ({
        type: 'createWindow',
        title: 'Open floating note',
        view,
        windowOptions: options,
      }),
    },
  };
  return { ctx, values };
}

test('Floating Notes creates, saves Markdown, renames, lists, and deletes notes', async () => {
  const { ctx, values } = createContext();
  const extension = createFloatingNotesExtension();
  const list = await extension.commands[0].run(ctx as any);
  assert.equal(list.title, 'Floating Notes');
  assert.equal(list.items.length, 0);

  const created = await list.add.__handler(ctx);
  assert.equal(created.view.type, 'editor');
  assert.equal(created.view.title, 'Untitled note');
  const save = created.view.submitAction;
  const saved = await save.__handler(ctx, {
    editorContent: '# A durable idea\n\nTwo independent notes stay local.',
  });
  assert.equal(saved.toast.message, 'Saved');
  assert.equal(
    saved.view.content,
    '# A durable idea\n\nTwo independent notes stay local.',
  );

  const rename = saved.view.actions.find(
    (action: any) => action.title === 'Rename note',
  );
  const renamed = await rename.targetAction.__handler(ctx, {
    formValues: { title: 'Launch notes' },
  });
  assert.equal(renamed.view.title, 'Launch notes');

  const refreshed = await extension.commands[0].run(ctx as any);
  assert.equal(refreshed.items.length, 1);
  assert.equal(refreshed.items[0].title, 'Launch notes');
  assert.equal(refreshed.items[0].subtitle, 'A durable idea');
  const opened = await refreshed.items[0].preview.__handler(ctx);
  const floating = opened.view.actions.find(
    (action: any) => action.type === 'createWindow',
  );
  assert.equal(floating.type, 'createWindow');
  assert.match(floating.windowOptions.id, /^floating-note-note-/);
  assert.equal(floating.windowOptions.restoreKey, floating.windowOptions.id);

  const deleted = await refreshed.items[0].remove.__handler(ctx);
  assert.equal(deleted.toast.message, 'Note deleted');
  assert.deepEqual(values.get('notes'), []);
});

test('Floating Notes preserves independent note records', async () => {
  const { ctx, values } = createContext();
  const command = createFloatingNotesExtension().commands[1];
  await command.run(ctx as any);
  await command.run(ctx as any);
  const notes = values.get('notes') as Array<{ id: string; title: string }>;
  assert.equal(notes.length, 2);
  assert.notEqual(notes[0].id, notes[1].id);
  assert.equal(notes[0].title, 'Untitled note');
});
