type FloatingNote = {
  id: string;
  title: string;
  content: string;
  createdAt: number;
  updatedAt: number;
};

const NOTES_STORAGE_KEY = 'notes';
const MAX_NOTES = 500;

function noteId() {
  return `note-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`;
}

function noteTitle(value: unknown, fallback = 'Untitled note') {
  const title = String(value || '')
    .trim()
    .replace(/\s+/g, ' ');
  return title.slice(0, 120) || fallback;
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
    .map((note) => ({
      id: String(note.id || ''),
      title: noteTitle(note.title),
      content: noteContent(note.content),
      createdAt: Number(note.createdAt) || Date.now(),
      updatedAt: Number(note.updatedAt) || Date.now(),
    }))
    .filter((note) => /^note-[a-z0-9-]+$/i.test(note.id))
    .slice(0, MAX_NOTES);
}

async function readNotes(ctx: any) {
  return notesFrom(await ctx.storage.get(NOTES_STORAGE_KEY, []));
}

async function writeNotes(ctx: any, notes: FloatingNote[]) {
  await ctx.storage.set(NOTES_STORAGE_KEY, notes.slice(0, MAX_NOTES));
}

function preview(content: string) {
  const line = content.split(/\r?\n/).find((value) => value.trim());
  return line ? line.replace(/^#{1,6}\s*/, '').slice(0, 100) : 'Empty note';
}

function relativeTime(timestamp: number) {
  const seconds = Math.max(0, Math.floor((Date.now() - timestamp) / 1000));
  if (seconds < 60) return 'just now';
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`;
  if (seconds < 86_400) return `${Math.floor(seconds / 3600)}h ago`;
  return `${Math.floor(seconds / 86_400)}d ago`;
}

function findNote(notes: FloatingNote[], id: string) {
  return notes.find((note) => note.id === id) || null;
}

function saveAction(noteId: string) {
  return {
    type: 'runExtensionAction',
    title: 'Save note',
    shortcut: 'Command+S',
    __handler: async (ctx: any, action: any) => {
      const notes = await readNotes(ctx);
      const existing = findNote(notes, noteId);
      if (!existing)
        return ctx.ui.toast({
          message: 'Note no longer exists',
          tone: 'error',
        });
      const next = {
        ...existing,
        content: noteContent(action?.editorContent),
        updatedAt: Date.now(),
      };
      await writeNotes(
        ctx,
        notes.map((note) => (note.id === noteId ? next : note)),
      );
      return {
        view: noteEditor(ctx, next),
        navigation: 'replace',
        toast: { message: 'Saved', tone: 'success' },
      };
    },
  };
}

function renameAction(noteId: string) {
  return {
    type: 'promptAction',
    title: 'Rename note',
    message: 'Give this note a clear, memorable title.',
    fields: [{ id: 'title', label: 'Title', type: 'text', required: true }],
    submitTitle: 'Rename',
    targetAction: {
      type: 'runExtensionAction',
      title: 'Rename note',
      __handler: async (ctx: any, action: any) => {
        const notes = await readNotes(ctx);
        const existing = findNote(notes, noteId);
        if (!existing)
          return ctx.ui.toast({
            message: 'Note no longer exists',
            tone: 'error',
          });
        const next = {
          ...existing,
          title: noteTitle(action?.formValues?.title),
          updatedAt: Date.now(),
        };
        await writeNotes(
          ctx,
          notes.map((note) => (note.id === noteId ? next : note)),
        );
        return {
          view: noteEditor(ctx, next),
          navigation: 'replace',
          toast: { message: 'Renamed', tone: 'success' },
        };
      },
    },
  };
}

function deleteAction(note: FloatingNote) {
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
      return {
        view: await notesView(ctx),
        navigation: 'root',
        toast: { message: 'Note deleted', tone: 'success' },
      };
    },
  };
}

function createAction() {
  return {
    type: 'runExtensionAction',
    title: 'New note',
    shortcut: 'Command+N',
    __handler: async (ctx: any) => {
      const now = Date.now();
      const note: FloatingNote = {
        id: noteId(),
        title: 'Untitled note',
        content: '',
        createdAt: now,
        updatedAt: now,
      };
      await writeNotes(ctx, [note, ...(await readNotes(ctx))]);
      return {
        view: noteEditor(ctx, note),
        navigation: 'push',
        toast: { message: 'New note ready', tone: 'success' },
      };
    },
  };
}

function openFloatingWindowAction(ctx: any, note: FloatingNote) {
  return ctx.windows.create(noteEditor(ctx, note, false), {
    id: `floating-note-${note.id}`,
    restoreKey: `floating-note-${note.id}`,
    title: note.title,
    titleBar: 'hidden',
    width: 760,
    height: 580,
    alwaysOnTop: true,
    persistent: true,
    remembersFrame: true,
  });
}

function noteEditor(ctx: any, note: FloatingNote, canFloat = true) {
  const save = saveAction(note.id);
  const rename = renameAction(note.id);
  const remove = deleteAction(note);
  const create = createAction();
  const openWindow = canFloat ? openFloatingWindowAction(ctx, note) : null;
  return ctx.ui.editor({
    id: `floating-note:${note.id}`,
    title: note.title,
    subtitle: `Markdown note · Edited ${relativeTime(note.updatedAt)}`,
    content: note.content,
    format: 'markdown',
    placeholder: 'Start with a thought, a checklist, or a tiny manifesto…',
    submitAction: save,
    actions: [save, rename, openWindow, create, remove].filter(Boolean),
    actionPanel: {
      title: 'Note actions',
      sections: [
        { title: 'Writing', actions: [save, rename] },
        {
          title: 'Notes',
          actions: [openWindow, create, remove].filter(Boolean),
        },
      ],
    },
  });
}

async function notesView(ctx: any) {
  const notes = (await readNotes(ctx)).sort(
    (a, b) => b.updatedAt - a.updatedAt,
  );
  const create = createAction();
  return ctx.ui.list({
    id: 'floating-notes',
    title: 'Floating Notes',
    subtitle: `${notes.length} ${notes.length === 1 ? 'note' : 'notes'} · Markdown, local, and yours`,
    searchBarPlaceholder: 'Find a note',
    emptyView: {
      title: 'A clear space for your next idea',
      subtitle: 'Create a Markdown note and keep it close at hand.',
    },
    actions: [create],
    actionPanel: { sections: [{ actions: [create] }] },
    items: notes.map((note) => ({
      id: `floating-note:${note.id}`,
      title: note.title,
      subtitle: preview(note.content),
      icon: 'notebook-pen',
      accessories: [{ text: relativeTime(note.updatedAt) }],
      primaryAction: {
        type: 'runExtensionAction',
        title: 'Open note',
        __handler: async (innerCtx: any) => {
          const current = findNote(await readNotes(innerCtx), note.id);
          return current
            ? { view: noteEditor(innerCtx, current), navigation: 'push' }
            : innerCtx.ui.toast({
                message: 'Note no longer exists',
                tone: 'error',
              });
        },
      },
      actions: [
        openFloatingWindowAction(ctx, note),
        renameAction(note.id),
        deleteAction(note),
      ],
    })),
  });
}

export function createFloatingNotesExtension() {
  return {
    id: 'nevermind.floating-notes',
    title: 'Floating Notes',
    commands: [
      {
        id: 'floating-notes',
        actionId: 'floating-notes',
        title: 'Floating Notes',
        subtitle: 'Create, edit, and keep Markdown notes close',
        aliases: ['notes', 'note'],
        icon: 'notebook-pen',
        score: 18,
        run: (ctx: any) => notesView(ctx),
      },
      {
        id: 'new-floating-note',
        actionId: 'new-floating-note',
        title: 'New Floating Note',
        subtitle: 'Start a fresh local Markdown note',
        aliases: ['new note'],
        icon: 'square-pen',
        score: 17,
        run: (ctx: any) => createAction().__handler(ctx),
      },
    ],
  };
}
