import assert from 'node:assert/strict';
import test from 'node:test';
import { createClipboardHistory } from './clipboard-history';

type TestDeps = Parameters<typeof createClipboardHistory>[0];

function noop() {}
function asyncNoop() {
  return Promise.resolve();
}

function createFakes(overrides: Partial<TestDeps> = {}) {
  const history: any[] = [];
  const suppressed = new Map<string, number>();
  let imagesDir = '/tmp/clipboard-images';
  const files: Map<string, Buffer> = new Map();
  const clipboardState: Record<string, any> = {
    text: '',
    html: '',
    rtf: '',
    image: null,
    bookmark: null,
  };
  const writtenData: any[] = [];

  const defaultClipboard = {
    readText: () => clipboardState.text,
    readHTML: () => clipboardState.html,
    readRTF: () => clipboardState.rtf,
    readBookmark: () => clipboardState.bookmark,
    readImage: () => clipboardState.image || { isEmpty: () => true },
    readBuffer: () => Buffer.from(''),
    write: (data: any) => {
      writtenData.push(data);
      Object.assign(clipboardState, data);
    },
    writeText: (text: string) => {
      writtenData.push({ text });
      clipboardState.text = text;
    },
    writeImage: (image: any) => {
      writtenData.push({ image });
      clipboardState.image = image;
    },
    writeBuffer: (format: string, buffer: Buffer) => {
      writtenData.push({ format, buffer });
    },
    clear: () => {
      clipboardState.text = '';
      clipboardState.html = '';
      clipboardState.rtf = '';
      clipboardState.image = null;
      clipboardState.bookmark = null;
    },
  };

  const defaultNativeImage = {
    createFromDataURL: (url: string) => ({
      isEmpty: () => false,
      toPNG: () => Buffer.from('fake-png'),
      toDataURL: () => url,
    }),
    createFromPath: (p: string) => ({
      isEmpty: () => false,
      toPNG: () => Buffer.from('fake-png'),
    }),
    createEmpty: () => ({ isEmpty: () => true, toPNG: () => Buffer.from('') }),
  };

  const scheduled: string[] = [];
  const rendererMessages: Array<{ channel: string; args: any[] }> = [];
  const patches: Array<{ viewId: string; patch: any }> = [];
  const logs: Array<{ message: string; data?: unknown }> = [];
  const measures: Array<{ name: string; data: Record<string, unknown> }> = [];

  const deps: TestDeps = {
    getHistory: () => history,
    setHistory: (h: any[]) => {
      history.length = 0;
      history.push(...h);
    },
    getSuppressedItemIds: () => suppressed,
    getImagesDir: () => imagesDir,
    clipboard: defaultClipboard as any,
    nativeImage: defaultNativeImage as any,
    ensureDir: async () => {
      /* noop */
    },
    writeFile: async (filePath: string, data: Buffer) => {
      files.set(filePath, data);
    },
    hashValue: (value: unknown) => {
      const str =
        typeof value === 'string'
          ? value
          : Buffer.isBuffer(value)
            ? value.toString('base64').slice(0, 12)
            : String(value);
      let hash = 0;
      for (let i = 0; i < str.length; i++) {
        hash = ((hash << 5) - hash + str.charCodeAt(i)) | 0;
      }
      return `hash:${Math.abs(hash)}`;
    },
    fileUrlForPath: (p: string) => `file://${p}`,
    thumbnailUrlForPath: (p: string) => `thumb://${p}`,
    isVideoPath: (p: string) => p.endsWith('.mp4') || p.endsWith('.mov'),
    expandUserPath: (p: string) => p,
    isImagePath: (p: string) => /\.(png|jpg|jpeg|gif|webp|bmp)$/i.test(p),
    extensionForPath: (p: string) => p.split('.').pop() || '',
    pathJoin: (...segs: string[]) => segs.join('/'),
    pathBasename: (p: string, ext?: string) => {
      const base = p.split('/').pop() || p;
      return ext ? base.replace(new RegExp(`${ext}$`), '') : base;
    },
    pathToFileURL: (filePath: string) => ({
      href: `file://${filePath}`,
    }),
    logWarn: (message: string, data?: unknown) => {
      logs.push({ message, data });
    },
    measureSync: <T>(
      name: string,
      data: Record<string, unknown>,
      fn: () => T,
    ) => {
      measures.push({ name, data });
      return fn();
    },
    scheduleSaveState: () => {
      scheduled.push('state.save');
    },
    invalidateExtensionRootItems: noop,
    emitChanged: noop,
    sendToRenderer: (channel: string, ...args: any[]) => {
      rendererMessages.push({ channel, args });
    },
    patchOpenView: (viewId: string, patch: any) => {
      patches.push({ viewId, patch });
    },
    pasteIntoFrontmostApp: noop,
    getSetting: () => undefined,
    buildPreviewItemAction: (opts: Record<string, unknown>) => ({
      type: 'previewClipboardItem',
      ...opts,
    }),
    rankAction: (item: any, query: string) => {
      const title = String(item.title || '');
      const subtitle = String(item.subtitle || '');
      return (
        title.toLowerCase().includes(query.toLowerCase()) ||
        subtitle.toLowerCase().includes(query.toLowerCase())
      );
    },
    fileToExtensionFile: async (p: string) => ({
      id: `file:${p}`,
      path: p,
      name: p.split('/').pop(),
    }),
    findFiles: async () => [],
    selectedFilePaths: async () => [],
    selectedExtensionFiles: async () => [],
    selectedText: async () => '',
    selectedFiles: async () => [],
    frontmostApp: async () => null,
    readDesktopSelection: async () => ({ type: 'empty' }),
    CLIPBOARD_LIMIT: 300,
    CLIPBOARD_POLL_INTERVAL_MS: 1000,
    CLIPBOARD_LAST_HOUR_MS: 60 * 60_000,
    CLIPBOARD_LAST_DAY_MS: 24 * 60 * 60_000,
    ...overrides,
  };

  return {
    deps,
    clipboardHistory: createClipboardHistory(deps),
    getHistory: () => history,
    getSuppressed: () => suppressed,
    getFiles: () => files,
    getWrittenData: () => writtenData,
    getRendererMessages: () => rendererMessages,
    getLogs: () => logs,
    getMeasures: () => measures,
    getScheduled: () => scheduled,
    getPatches: () => patches,
    setClipboardText: (text: string) => {
      clipboardState.text = text;
      clipboardState.html = '';
      clipboardState.image = null;
    },
    setClipboardImage: (dataUrl: string) => {
      clipboardState.text = '';
      clipboardState.html = '';
      clipboardState.image = {
        isEmpty: () => false,
        toPNG: () => Buffer.from(`image:${dataUrl}`),
      };
    },
    setImagesDir: (dir: string) => {
      imagesDir = dir;
    },
  };
}

// ═══════════════════════════════════════════════════════════
// Phase 1: persistClipboardImage — async, returns null on failure
// ═══════════════════════════════════════════════════════════

test('persistClipboardImage writes file and returns path', async () => {
  const { clipboardHistory, getFiles } = createFakes();
  const png = Buffer.from('test-png-data');
  const hash = 'abc123';

  const result = await clipboardHistory.persistClipboardImage(png, hash);

  assert.ok(result, 'should return a path');
  assert.ok(result!.includes('abc123.png'), 'path should contain hash');
  assert.ok(getFiles().has(result!), 'file should be written');
});

test('persistClipboardImage returns null on write failure', async () => {
  const { deps } = createFakes();
  deps.writeFile = async () => {
    throw new Error('disk full');
  };
  const ch = createClipboardHistory(deps);
  const png = Buffer.from('test-png-data');
  const hash = 'abc123';

  const result = await ch.persistClipboardImage(png, hash);

  assert.equal(result, null, 'should return null on failure');
});

// ═══════════════════════════════════════════════════════════
// readClipboardItem
// ═══════════════════════════════════════════════════════════

test('readClipboardItem reads text from clipboard', async () => {
  const { clipboardHistory, setClipboardText } = createFakes();
  setClipboardText('hello world');

  const item = await clipboardHistory.readClipboardItem();

  assert.equal(item?.type, 'text');
  assert.equal(item?.text, 'hello world');
  assert.ok(item?.id.startsWith('text:'));
});

test('readClipboardItem returns null for empty clipboard', async () => {
  const { clipboardHistory } = createFakes();

  const item = await clipboardHistory.readClipboardItem();

  assert.equal(item, null);
});

test('readClipboardItem reads image from clipboard', async () => {
  const { clipboardHistory, setClipboardImage } = createFakes();
  setClipboardImage('data:image/png;base64,abc');

  const item = await clipboardHistory.readClipboardItem();

  assert.equal(item?.type, 'image');
  assert.ok(item?.imagePath, 'should have image path');
});

// ═══════════════════════════════════════════════════════════
// rememberClipboardItem
// ═══════════════════════════════════════════════════════════

test('rememberClipboardItem prepends item to history', () => {
  const { clipboardHistory, getHistory } = createFakes();

  const item = {
    id: 'text:hash123',
    type: 'text',
    text: 'hello',
    createdAt: Date.now(),
  };
  clipboardHistory.rememberClipboardItem(item);

  const history = getHistory();
  assert.equal(history.length, 1);
  assert.equal(history[0].id, 'text:hash123');
});

test('rememberClipboardItem deduplicates by id', () => {
  const { clipboardHistory, getHistory } = createFakes();

  const item = {
    id: 'text:hash123',
    type: 'text',
    text: 'hello',
    createdAt: 1000,
  };
  clipboardHistory.rememberClipboardItem(item);
  const item2 = {
    id: 'text:hash123',
    type: 'text',
    text: 'hello updated',
    createdAt: 2000,
  };
  clipboardHistory.rememberClipboardItem(item2);

  const history = getHistory();
  assert.equal(history.length, 1);
  assert.equal(history[0].text, 'hello updated');
});

test('rememberClipboardItem ignores null/undefined', () => {
  const { clipboardHistory, getHistory } = createFakes();

  clipboardHistory.rememberClipboardItem(null);
  clipboardHistory.rememberClipboardItem(undefined);

  assert.equal(getHistory().length, 0);
});

// ═══════════════════════════════════════════════════════════
// clipboardHistorySnapshot
// ═══════════════════════════════════════════════════════════

test('clipboardHistorySnapshot returns all entries', () => {
  const { clipboardHistory, getHistory } = createFakes();
  getHistory().push(
    { id: 'text:a', type: 'text', text: 'a', createdAt: 1 },
    { id: 'text:b', type: 'text', text: 'b', createdAt: 2 },
    { id: 'image:c', type: 'image', imagePath: '/tmp/c.png', createdAt: 3 },
  );

  const snapshot = clipboardHistory.clipboardHistorySnapshot();

  assert.equal(snapshot.length, 3);
  assert.equal(snapshot[0].id, 'text:a');
  assert.equal(snapshot[2].id, 'image:c');
});

test('clipboardHistorySnapshot filters by type', () => {
  const { clipboardHistory, getHistory } = createFakes();
  getHistory().push(
    { id: 'text:a', type: 'text', text: 'a', createdAt: 1 },
    { id: 'image:b', type: 'image', imagePath: '/tmp/b.png', createdAt: 2 },
  );

  const snapshot = clipboardHistory.clipboardHistorySnapshot({
    types: ['image'],
  });

  assert.equal(snapshot.length, 1);
  assert.equal(snapshot[0].id, 'image:b');
});

test('clipboardHistorySnapshot filters by query', () => {
  const { clipboardHistory, getHistory } = createFakes();
  getHistory().push(
    { id: 'text:a', type: 'text', text: 'hello world', createdAt: 1 },
    { id: 'text:b', type: 'text', text: 'goodbye', createdAt: 2 },
  );

  const snapshot = clipboardHistory.clipboardHistorySnapshot({
    query: 'hello',
  });

  assert.equal(snapshot.length, 1);
  assert.equal(snapshot[0].id, 'text:a');
});

test('clipboardHistorySnapshot respects limit', () => {
  const { clipboardHistory, getHistory } = createFakes();
  for (let i = 0; i < 10; i++)
    getHistory().push({
      id: `text:${i}`,
      type: 'text',
      text: `item ${i}`,
      createdAt: i,
    });

  const snapshot = clipboardHistory.clipboardHistorySnapshot({ limit: 3 });

  assert.equal(snapshot.length, 3);
});

// ═══════════════════════════════════════════════════════════
// removeClipboardHistoryByAction
// ═══════════════════════════════════════════════════════════

test('removeClipboardHistoryByAction removes a single item', () => {
  const { clipboardHistory, getHistory } = createFakes();
  getHistory().push(
    { id: 'text:a', type: 'text', text: 'a', createdAt: 1 },
    { id: 'text:b', type: 'text', text: 'b', createdAt: 2 },
  );

  const removed = clipboardHistory.removeClipboardHistoryByAction({
    clipboardHistoryRange: 'item',
    clipboardHistoryItemId: 'text:a',
  });

  assert.equal(removed, 1);
  assert.equal(getHistory().length, 1);
  assert.equal(getHistory()[0].id, 'text:b');
});

test('removeClipboardHistoryByAction removes by ids', () => {
  const { clipboardHistory, getHistory } = createFakes();
  getHistory().push(
    { id: 'text:a', type: 'text', text: 'a', createdAt: 1 },
    { id: 'text:b', type: 'text', text: 'b', createdAt: 2 },
    { id: 'text:c', type: 'text', text: 'c', createdAt: 3 },
  );

  const removed = clipboardHistory.removeClipboardHistoryByAction({
    clipboardHistoryRange: 'ids',
    clipboardHistoryItemIds: ['text:a', 'text:c'],
  });

  assert.equal(removed, 2);
  assert.equal(getHistory().length, 1);
  assert.equal(getHistory()[0].id, 'text:b');
});

test('removeClipboardHistoryByAction removes all', () => {
  const { clipboardHistory, getHistory } = createFakes();
  getHistory().push(
    { id: 'text:a', type: 'text', text: 'a', createdAt: 1 },
    { id: 'text:b', type: 'text', text: 'b', createdAt: 2 },
  );

  const removed = clipboardHistory.removeClipboardHistoryByAction({
    clipboardHistoryRange: 'all',
  });

  assert.equal(removed, 2);
  assert.equal(getHistory().length, 0);
});

test('removeClipboardHistoryByAction no-ops on no match', () => {
  const { clipboardHistory, getHistory } = createFakes();

  const removed = clipboardHistory.removeClipboardHistoryByAction({
    clipboardHistoryRange: 'item',
    clipboardHistoryItemId: 'text:nonexistent',
  });

  assert.equal(removed, 0);
  assert.equal(getHistory().length, 0);
});

// ═══════════════════════════════════════════════════════════
// clipboardHistoryIdForText / suppressClipboardHistoryId
// ═══════════════════════════════════════════════════════════

test('clipboardHistoryIdForText produces stable id', () => {
  const { clipboardHistory } = createFakes();

  const id1 = clipboardHistory.clipboardHistoryIdForText('hello world');
  const id2 = clipboardHistory.clipboardHistoryIdForText('hello world');

  assert.ok(id1.startsWith('text:'));
  assert.equal(id1, id2, 'same text should produce same id');
});

test('clipboardHistoryIdForText returns empty for empty text', () => {
  const { clipboardHistory } = createFakes();

  assert.equal(clipboardHistory.clipboardHistoryIdForText(''), '');
  assert.equal(clipboardHistory.clipboardHistoryIdForText('  '), '');
});

test('suppressClipboardHistoryId adds suppression', () => {
  const { clipboardHistory, getSuppressed } = createFakes();

  clipboardHistory.suppressClipboardHistoryId('text:abc', 5000);
  const suppressed = getSuppressed();

  assert.ok(suppressed.has('text:abc'));
  assert.ok(
    (suppressed.get('text:abc') || 0) > Date.now(),
    'suppressUntil should be in the future',
  );
});

// ═══════════════════════════════════════════════════════════
// clipboardSnapshot / restoreClipboardSnapshot
// ═══════════════════════════════════════════════════════════

test('clipboardSnapshot captures current clipboard state', () => {
  const { deps } = createFakes();
  (deps.clipboard as any).readText = () => 'snap text';
  (deps.clipboard as any).readHTML = () => '<b>snap</b>';
  const ch = createClipboardHistory(deps);

  const snap = ch.clipboardSnapshot();

  assert.equal(snap.text, 'snap text');
  assert.equal(snap.html, '<b>snap</b>');
  assert.equal(snap.image, null);
});

test('restoreClipboardSnapshot restores state', () => {
  const { clipboardHistory, deps } = createFakes();
  const snap = {
    text: 'restored',
    html: '<b>restored</b>',
    rtf: '',
    bookmark: null as any,
    image: null as any,
  };

  clipboardHistory.restoreClipboardSnapshot(snap);

  assert.equal((deps.clipboard as any).readText(), 'restored');
  assert.equal((deps.clipboard as any).readHTML(), '<b>restored</b>');
});

test('restoreClipboardSnapshot clears on empty snapshot', () => {
  const { clipboardHistory, deps } = createFakes();
  (deps.clipboard as any).write({
    text: 'before',
  });

  clipboardHistory.restoreClipboardSnapshot({
    text: '',
    html: '',
    rtf: '',
    bookmark: null as any,
    image: null as any,
  });

  assert.equal((deps.clipboard as any).readText(), '');
});

// ═══════════════════════════════════════════════════════════
// clipboardHistoryView
// ═══════════════════════════════════════════════════════════

test('clipboardHistoryView returns view with items', () => {
  const { clipboardHistory, getHistory } = createFakes();
  getHistory().push({
    id: 'text:a',
    type: 'text',
    text: 'hello',
    createdAt: Date.now(),
  });

  const view = clipboardHistory.clipboardHistoryView();

  assert.equal(view.type, 'list');
  assert.equal(view.id, 'clipboard-history');
  assert.equal(view.title, 'Clipboard History');
  assert.ok(Array.isArray(view.items));
  assert.equal(view.items.length, 1);
});

test('clipboardHistoryView shows empty state when no history', () => {
  const { clipboardHistory } = createFakes();

  const view = clipboardHistory.clipboardHistoryView();

  assert.equal(view.items.length, 0);
  assert.ok(view.emptyView);
  assert.equal(view.emptyView.title, 'No clipboard items found.');
});

// ═══════════════════════════════════════════════════════════
// createClipboardExtension
// ═══════════════════════════════════════════════════════════

test('createClipboardExtension returns extension descriptor', () => {
  const { clipboardHistory } = createFakes();

  const extension = clipboardHistory.createClipboardExtension();

  assert.equal(extension.id, 'nevermind.clipboard');
  assert.equal(extension.title, 'Clipboard');
  assert.ok(Array.isArray(extension.commands));
  assert.equal(extension.commands.length, 1);
  assert.equal(extension.commands[0].id, 'clipboard-history');
});

// ═══════════════════════════════════════════════════════════
// clipboardHistoryItem (UI builder)
// ═══════════════════════════════════════════════════════════

test('clipboardHistoryItem builds text item with paste action', () => {
  const { clipboardHistory } = createFakes();
  const item = {
    id: 'text:abc',
    type: 'text',
    text: 'hello world',
    createdAt: Date.now(),
  };

  const result = clipboardHistory.clipboardHistoryItem(item);

  assert.equal(result.id, 'clipboard:text:abc');
  assert.equal(result.title, 'hello world');
  assert.ok(result.actionPanel);
  const actions = result.actionPanel.sections[0].actions;
  assert.ok(actions.some((a: any) => a.type === 'pasteText'));
});

test('clipboardHistoryItem builds image item without paste action', () => {
  const { clipboardHistory } = createFakes();
  const item = {
    id: 'image:abc',
    type: 'image',
    imagePath: '/tmp/img.png',
    createdAt: Date.now(),
  };

  const result = clipboardHistory.clipboardHistoryItem(item);

  const actions = result.actionPanel.sections[0].actions;
  assert.ok(!actions.some((a: any) => a && a.type === 'pasteText'));
});

// ═══════════════════════════════════════════════════════════
// clipboardRemovedMessage
// ═══════════════════════════════════════════════════════════

test('clipboardHistoryRemovedMessage singular vs plural', () => {
  const { clipboardHistory } = createFakes();

  assert.equal(
    clipboardHistory.clipboardHistoryRemovedMessage(1),
    'Removed 1 clipboard item',
  );
  assert.equal(
    clipboardHistory.clipboardHistoryRemovedMessage(5),
    'Removed 5 clipboard items',
  );
});

// ═══════════════════════════════════════════════════════════
// startClipboardWatcher
// ═══════════════════════════════════════════════════════════

test('startClipboardWatcher returns job descriptor', async () => {
  const { clipboardHistory } = createFakes();

  const job = await clipboardHistory.startClipboardWatcher();

  assert.equal(job.id, 'clipboard.poll');
  assert.equal(job.owner, 'host');
  assert.equal(job.scope, 'clipboard');
  assert.ok(Array.isArray(job.triggers));
  assert.equal(job.triggers[0].type, 'interval');
  assert.equal(typeof job.run, 'function');
});

// ═══════════════════════════════════════════════════════════
// readDesktopClipboard
// ═══════════════════════════════════════════════════════════

test('readDesktopClipboard returns text when clipboard has text', async () => {
  const { deps } = createFakes();
  (deps.clipboard as any).readText = () => 'hello clipboard';
  const ch = createClipboardHistory(deps);

  const result = await ch.readDesktopClipboard();

  assert.equal(result.type, 'text');
  assert.equal(result.text, 'hello clipboard');
});

test('readDesktopClipboard returns empty when clipboard is empty', async () => {
  const { deps } = createFakes();
  const ch = createClipboardHistory(deps);

  const result = await ch.readDesktopClipboard();

  assert.equal(result.type, 'empty');
});

// ═══════════════════════════════════════════════════════════
// writeDesktopClipboard
// ═══════════════════════════════════════════════════════════

test('writeDesktopClipboard writes text', () => {
  const { clipboardHistory, deps } = createFakes();

  clipboardHistory.writeDesktopClipboard({ type: 'text', text: 'written' });

  assert.equal((deps.clipboard as any).readText(), 'written');
});

test('writeDesktopClipboard suppresses history for concealed content', () => {
  const { clipboardHistory, getSuppressed } = createFakes();

  clipboardHistory.writeDesktopClipboard(
    { type: 'text', text: 'secret' },
    { concealed: true },
  );

  const suppressed = getSuppressed();
  assert.ok(suppressed.size > 0, 'should have suppressed items');
});

// ═══════════════════════════════════════════════════════════
// clipboardHistoryGet
// ═══════════════════════════════════════════════════════════

test('clipboardHistoryGet retrieves item by id', () => {
  const { clipboardHistory, getHistory } = createFakes();
  getHistory().push(
    { id: 'text:a', type: 'text', text: 'a', createdAt: 1 },
    { id: 'text:b', type: 'text', text: 'b', createdAt: 2 },
  );

  const entry = clipboardHistory.clipboardHistoryGet('text:a');

  assert.ok(entry);
  assert.equal(entry.id, 'text:a');
});

test('clipboardHistoryGet returns null for unknown id', () => {
  const { clipboardHistory } = createFakes();

  const entry = clipboardHistory.clipboardHistoryGet('text:none');

  assert.equal(entry, null);
});
