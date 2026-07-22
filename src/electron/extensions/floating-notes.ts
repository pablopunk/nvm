// biome-ignore-all lint: First-party extension mirrors dynamic extension context payloads and bounded note limits.
import { titleFromFirstContentLine } from '../../editor-title';

type FloatingNote = {
  id: string;
  title: string;
  content: string;
  createdAt: number;
  updatedAt: number;
};

const NOTES_STORAGE_KEY = 'notes';
const MAX_NOTES = 500;
const FLOATING_WINDOW_ID = 'floating-notes';

function noteId() {
  return `note-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`;
}

function noteContent(value: unknown) {
  return String(value || '').slice(0, 524_288);
}

function notesFrom(value: unknown): FloatingNote[] {
  if (!Array.isArray(value)) return [];
  return value
    .filter((note): note is Record<string, unknown> =>
      Boolean(note && typeof note === 'object'),
    )
    .map((note) => {
      const content = noteContent(note.content);
      return {
        id: String(note.id || ''),
        title: titleFromFirstContentLine(content),
        content,
        createdAt: Number(note.createdAt) || Date.now(),
        updatedAt: Number(note.updatedAt) || Date.now(),
      };
    })
    .filter((note) => /^note-[a-z0-9-]+$/i.test(note.id))
    .slice(0, MAX_NOTES);
}

async function readNotes(ctx: any) {
  return notesFrom(await ctx.storage.get(NOTES_STORAGE_KEY, []));
}

async function writeNotes(ctx: any, notes: FloatingNote[]) {
  await ctx.storage.set(NOTES_STORAGE_KEY, notes.slice(0, MAX_NOTES));
}

async function updateNote(
  ctx: any,
  id: string,
  patch: Partial<FloatingNote>,
  { bumpVersion = true } = {},
) {
  const notes = await readNotes(ctx);
  const existing = notes.find((note) => note.id === id);
  if (!existing) return null;
  const content = noteContent(patch.content ?? existing.content);
  const next: FloatingNote = {
    ...existing,
    ...patch,
    title: titleFromFirstContentLine(content),
    content,
    updatedAt: bumpVersion
      ? Date.now()
      : (patch.updatedAt ?? existing.updatedAt),
  };
  await writeNotes(
    ctx,
    notes.map((note) => (note.id === id ? next : note)),
  );
  if (bumpVersion) await ctx.drafts.commit(id, next.updatedAt);
  return next;
}

function preview(content: string) {
  const lines = content
    .split(/\r?\n/)
    .map((line) => line.replace(/^#{1,6}\s*/, '').trim())
    .filter(Boolean);
  return lines[1]?.slice(0, 100) || (lines[0] ? 'Markdown note' : 'Empty note');
}

function relativeTime(timestamp: number) {
  const seconds = Math.max(0, Math.floor((Date.now() - timestamp) / 1000));
  if (seconds < 60) return 'just now';
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`;
  if (seconds < 86_400) return `${Math.floor(seconds / 3600)}h ago`;
  return `${Math.floor(seconds / 86_400)}d ago`;
}

async function mostRecentNote(ctx: any) {
  const notes = (await readNotes(ctx)).sort(
    (a, b) => b.updatedAt - a.updatedAt,
  );
  return notes[0] || null;
}

async function createNote(ctx: any) {
  const now = Date.now();
  const note: FloatingNote = {
    id: noteId(),
    title: 'Untitled note',
    content: '',
    createdAt: now,
    updatedAt: now,
  };
  await writeNotes(ctx, [note, ...(await readNotes(ctx))]);
  return note;
}

function autosaveAction(noteId: string) {
  return {
    type: 'runExtensionAction',
    title: 'Autosave note',
    __handler: async (ctx: any, action: any) => {
      await updateNote(ctx, noteId, {
        content: noteContent(action?.editorContent),
      });
    },
  };
}

function draftConflictAction(noteId: string) {
  return {
    type: 'runExtensionAction',
    title: 'Resolve unsaved changes',
    __handler: (ctx: any, action: any) => {
      const conflict = action?.draftConflict;
      if (!conflict) return;
      const restoreOld = {
        type: 'runExtensionAction',
        title: 'Restore unsaved changes',
        subtitle: preview(String(conflict.storedContent || '')),
        icon: 'history',
        __handler: async (innerCtx: any) => {
          await updateNote(
            innerCtx,
            noteId,
            {
              content: noteContent(conflict.storedContent),
              updatedAt: Number(conflict.currentVersion) || Date.now(),
            },
            { bumpVersion: false },
          );
          const note = (await readNotes(innerCtx)).find(
            (item) => item.id === noteId,
          );
          return {
            type: 'draftResolution',
            key: conflict.key,
            resolution: 'restore-old',
            view: note
              ? noteEditor(innerCtx, note)
              : await notesCollection(innerCtx),
            navigation: 'replace',
            toast: { message: 'Unsaved changes restored', tone: 'success' },
          };
        },
      };
      const keepSaved = {
        type: 'runExtensionAction',
        title: 'Keep saved version',
        subtitle: preview(String(conflict.currentContent || '')),
        icon: 'check',
        __handler: async (innerCtx: any) => {
          const note = (await readNotes(innerCtx)).find(
            (item) => item.id === noteId,
          );
          return {
            type: 'draftResolution',
            key: conflict.key,
            resolution: 'reset',
            view: note
              ? noteEditor(innerCtx, note)
              : await notesCollection(innerCtx),
            navigation: 'replace',
          };
        },
      };
      return {
        view: ctx.ui.list({
          id: `floating-note-conflict:${noteId}`,
          title: 'Unsaved changes found',
          subtitle: 'This note has unsaved changes from another session.',
          items: [restoreOld, keepSaved].map((choice, index) => ({
            id: `choice-${index}`,
            title: choice.title,
            subtitle: choice.subtitle,
            icon: choice.icon,
            primaryAction: choice,
            actions: [choice],
          })),
        }),
        navigation: 'replace',
      };
    },
  };
}

async function nextEditorResult(ctx: any) {
  const next = (await mostRecentNote(ctx)) || (await createNote(ctx));
  return { view: noteEditor(ctx, next), navigation: 'replace' };
}

function deleteFromEditorAction(note: FloatingNote) {
  return {
    type: 'runExtensionAction',
    title: 'Delete note',
    style: 'destructive',
    requiresConfirmation: true,
    confirmMessage: `Delete “${note.title}”? This cannot be undone.`,
    confirmLabel: 'Delete note',
    __handler: async (ctx: any) => {
      const notes = await readNotes(ctx);
      await writeNotes(
        ctx,
        notes.filter((item) => item.id !== note.id),
      );
      await ctx.drafts.discard(note.id);
      const result = await nextEditorResult(ctx);
      return {
        ...result,
        toast: { message: 'Note deleted', tone: 'success' },
      };
    },
  };
}

function deleteFromCollectionAction(note: FloatingNote) {
  return {
    type: 'runExtensionAction',
    title: 'Delete note',
    __handler: async (ctx: any) => {
      const notes = await readNotes(ctx);
      await writeNotes(
        ctx,
        notes.filter((item) => item.id !== note.id),
      );
      await ctx.drafts.discard(note.id);
      return {
        view: await notesCollection(ctx),
        navigation: 'replace',
        toast: { message: 'Note deleted', tone: 'success' },
      };
    },
  };
}

function newNoteAction() {
  return {
    type: 'runExtensionAction',
    title: 'New note',
    subtitle: 'Start a fresh Markdown note',
    icon: 'square-pen',
    shortcut: 'Command+N',
    __handler: async (ctx: any) => {
      const note = await createNote(ctx);
      return { view: noteEditor(ctx, note), navigation: 'replace' };
    },
  };
}

function allNotesAction() {
  return {
    type: 'runExtensionAction',
    title: 'All notes',
    subtitle: 'Search and switch notes',
    icon: 'list',
    shortcut: 'Command+O',
    __handler: async (ctx: any) => ({
      view: await notesCollection(ctx),
      navigation: 'replace',
    }),
  };
}

function noteEditor(ctx: any, note: FloatingNote) {
  const remove = deleteFromEditorAction(note);
  const create = newNoteAction();
  const allNotes = allNotesAction();
  return ctx.ui.editor({
    id: `floating-note:${note.id}`,
    title: note.title,
    subtitle: `Markdown · Edited ${relativeTime(note.updatedAt)}`,
    content: note.content,
    format: 'markdown',
    titleFromContent: true,
    placeholder: 'Start with a thought, a checklist, or a tiny manifesto…',
    draft: {
      key: note.id,
      version: note.updatedAt,
      autosave: { debounceMs: 400, action: autosaveAction(note.id) },
      onConflict: draftConflictAction(note.id),
    },
    actions: [create, allNotes, remove],
    actionPanel: {
      title: 'Note actions',
      sections: [
        { title: 'Notes', actions: [create, allNotes] },
        { title: 'This note', actions: [remove] },
      ],
    },
  });
}

async function notesCollection(ctx: any) {
  const notes = (await readNotes(ctx)).sort(
    (a, b) => b.updatedAt - a.updatedAt,
  );
  return ctx.ui.collection({
    id: 'floating-notes',
    title: 'Floating Notes',
    windowPresentation: 'compact',
    subtitle: `${notes.length} ${notes.length === 1 ? 'note' : 'notes'} · Markdown, local, and yours`,
    searchBarPlaceholder: 'Find a note',
    emptyView: {
      title: 'A clear space for your next idea',
      subtitle: 'Create a Markdown note and keep it close at hand.',
    },
    add: newNoteAction(),
    items: notes.map((note) => ({
      id: `floating-note:${note.id}`,
      title: note.title,
      subtitle: preview(note.content),
      icon: 'notebook-pen',
      accessories: [{ text: relativeTime(note.updatedAt) }],
      preview: {
        type: 'runExtensionAction',
        title: 'Open note',
        __handler: async (innerCtx: any) => {
          const current = (await readNotes(innerCtx)).find(
            (item) => item.id === note.id,
          );
          return current
            ? { view: noteEditor(innerCtx, current), navigation: 'replace' }
            : innerCtx.ui.toast({
                message: 'Note no longer exists',
                tone: 'error',
              });
        },
      },
      remove: deleteFromCollectionAction(note),
    })),
  });
}

function floatingWindowOptions() {
  return {
    id: FLOATING_WINDOW_ID,
    restoreKey: FLOATING_WINDOW_ID,
    title: 'Floating Notes',
    titleBar: 'hidden',
    width: 640,
    height: 460,
    alwaysOnTop: true,
    persistent: true,
    remembersFrame: true,
  };
}

async function openFloatingWindow(ctx: any) {
  const note = (await mostRecentNote(ctx)) || (await createNote(ctx));
  return ctx.windows.create(noteEditor(ctx, note), floatingWindowOptions());
}

export function createFloatingNotesExtension() {
  return {
    id: 'nevermind.floating-notes',
    title: 'Floating Notes',
    async restoreWindow(ctx: any, restoreKey: string) {
      if (restoreKey !== FLOATING_WINDOW_ID) return null;
      const note = (await mostRecentNote(ctx)) || (await createNote(ctx));
      return {
        view: noteEditor(ctx, note),
        options: floatingWindowOptions(),
      };
    },
    commands: [
      {
        id: 'floating-notes',
        actionId: 'floating-notes',
        title: 'Floating Notes',
        subtitle: 'Open your floating Markdown notes',
        aliases: ['notes', 'note'],
        icon: 'notebook-pen',
        score: 18,
        run: (ctx: any) => openFloatingWindow(ctx),
      },
      {
        id: 'search-floating-notes',
        actionId: 'search-floating-notes',
        title: 'Search Floating Notes',
        subtitle: 'Browse and delete your notes',
        aliases: ['notes', 'all notes'],
        icon: 'list',
        score: 17,
        run: (ctx: any) => notesCollection(ctx),
      },
      {
        id: 'new-floating-note',
        actionId: 'new-floating-note',
        title: 'New Floating Note',
        subtitle: 'Start a fresh local Markdown note',
        aliases: ['new note'],
        icon: 'square-pen',
        score: 16,
        run: async (ctx: any) => {
          const note = await createNote(ctx);
          return ctx.windows.create(noteEditor(ctx, note), {
            ...floatingWindowOptions(),
            title: note.title,
          });
        },
      },
    ],
  };
}
