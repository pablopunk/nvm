// biome-ignore-all lint: Test context double mirrors dynamic extension payloads.
import assert from 'node:assert/strict';
import test from 'node:test';
import { createFloatingNotesExtension } from './floating-notes';

function createContext() {
  const values = new Map<string, unknown>();
  const commits: Array<{ key: string; version: unknown }> = [];
  const discards: string[] = [];
  const ctx = {
    storage: {
      get: async (key: string, fallback: unknown) =>
        values.get(key) ?? fallback,
      set: async (key: string, value: unknown) => {
        values.set(key, value);
        return value;
      },
    },
    drafts: {
      commit: async (key: string, version: unknown) => {
        commits.push({ key, version });
        return true;
      },
      discard: async (key: string) => {
        discards.push(key);
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
        title: 'Floating Notes',
        view,
        windowOptions: options,
      }),
    },
  };
  return { ctx, values, commits, discards };
}

test('Floating Notes command opens a persistent floating window with an autosaving editor', async () => {
  const { ctx, values } = createContext();
  const extension = createFloatingNotesExtension();

  const action = await extension.commands[0].run(ctx as any);
  assert.equal(action.type, 'createWindow');
  assert.equal(action.windowOptions.id, 'floating-notes');
  assert.equal(action.windowOptions.restoreKey, 'floating-notes');
  assert.equal(action.windowOptions.persistent, true);
  assert.equal(action.windowOptions.remembersFrame, true);
  assert.equal(action.windowOptions.alwaysOnTop, true);
  assert.equal(action.windowOptions.titleBar, 'hidden');

  const view = action.view;
  assert.equal(view.type, 'editor');
  assert.equal(view.format, 'markdown');
  assert.equal(view.titleFromContent, true);
  assert.equal(view.actionPanelPresentation, 'compact');
  assert.equal(view.submitAction, undefined);
  assert.equal(view.draft.autosave.debounceMs, 400);
  assert.equal(typeof view.draft.autosave.action.__handler, 'function');
  assert.equal(typeof view.draft.onConflict.__handler, 'function');

  const notes = values.get('notes') as Array<{ id: string; updatedAt: number }>;
  assert.equal(notes.length, 1);
  assert.equal(view.draft.key, notes[0].id);
  assert.equal(view.draft.version, notes[0].updatedAt);

  const reopened = await extension.commands[0].run(ctx as any);
  assert.equal(reopened.view.draft.key, notes[0].id);
  assert.equal((values.get('notes') as unknown[]).length, 1);
});

test('autosave writes content, bumps the version, and commits the draft', async () => {
  const { ctx, values, commits } = createContext();
  const extension = createFloatingNotesExtension();
  const action = await extension.commands[0].run(ctx as any);
  const view = action.view;

  await view.draft.autosave.action.__handler(ctx, {
    editorContent: '# A durable idea',
  });
  const notes = values.get('notes') as Array<{
    id: string;
    title: string;
    content: string;
    updatedAt: number;
  }>;
  assert.equal(notes[0].content, '# A durable idea');
  assert.equal(notes[0].title, 'A durable idea');
  assert.ok(notes[0].updatedAt >= view.draft.version);
  assert.deepEqual(commits, [
    { key: notes[0].id, version: notes[0].updatedAt },
  ]);
});

test('conflict action offers restore and reset resolutions that keep versions consistent', async () => {
  const { ctx } = createContext();
  const extension = createFloatingNotesExtension();
  const action = await extension.commands[0].run(ctx as any);
  const view = action.view;
  const noteId = view.draft.key;

  const conflictView = await view.draft.onConflict.__handler(ctx, {
    draftConflict: {
      key: noteId,
      storedVersion: 1,
      storedContent: 'unsaved work',
      currentVersion: view.draft.version,
      currentContent: '',
    },
  });
  assert.equal(conflictView.view.type, 'list');
  const [restore, keep] = conflictView.view.items.map(
    (item: any) => item.primaryAction,
  );
  assert.equal(restore.title, 'Restore unsaved changes');
  assert.equal(keep.title, 'Keep saved version');

  const restored = await restore.__handler(ctx);
  assert.equal(restored.type, 'draftResolution');
  assert.equal(restored.resolution, 'restore-old');
  assert.equal(restored.view.content, 'unsaved work');
  assert.equal(restored.view.draft.version, view.draft.version);

  const kept = await keep.__handler(ctx);
  assert.equal(kept.type, 'draftResolution');
  assert.equal(kept.resolution, 'reset');
  assert.equal(kept.view.type, 'editor');
});

test('collection lists notes with CRUD operations and deletes discard drafts', async () => {
  const { ctx, values, discards } = createContext();
  const extension = createFloatingNotesExtension();
  const collection = await extension.commands[1].run(ctx as any);
  assert.equal(collection.title, 'Floating Notes');
  assert.equal(collection.items.length, 0);

  await collection.add.__handler(ctx);
  await collection.add.__handler(ctx);
  const refreshed = await extension.commands[1].run(ctx as any);
  assert.equal(refreshed.items.length, 2);

  const item = refreshed.items[0];
  const opened = await item.preview.__handler(ctx);
  assert.equal(opened.view.type, 'editor');
  assert.equal(item.edit, undefined);

  const noteId = item.id.replace('floating-note:', '');
  const deleted = await item.remove.__handler(ctx);
  assert.equal(deleted.toast.message, 'Note deleted');
  assert.deepEqual(discards, [noteId]);
  assert.equal((values.get('notes') as unknown[]).length, 1);
});

test('deleting the open note moves to the next note or a fresh one', async () => {
  const { ctx, values } = createContext();
  const extension = createFloatingNotesExtension();
  const action = await extension.commands[0].run(ctx as any);
  const remove = action.view.actions.find(
    (candidate: any) => candidate.title === 'Delete note',
  );
  const result = await remove.__handler(ctx);
  assert.equal(result.view.type, 'editor');
  assert.equal(result.toast.message, 'Note deleted');
  const notes = values.get('notes') as Array<{ id: string }>;
  assert.equal(notes.length, 1);
  assert.notEqual(notes[0].id, action.view.draft.key);
});

test('restoreWindow rebuilds the floating window with the most recent note', async () => {
  const { ctx } = createContext();
  const extension = createFloatingNotesExtension();
  assert.equal(await extension.restoreWindow(ctx as any, 'other'), null);

  const action = await extension.commands[0].run(ctx as any);
  const descriptor = await extension.restoreWindow(
    ctx as any,
    'floating-notes',
  );
  assert.equal(descriptor.options.restoreKey, 'floating-notes');
  assert.equal(descriptor.options.persistent, true);
  assert.equal(descriptor.view.draft.key, action.view.draft.key);
});
