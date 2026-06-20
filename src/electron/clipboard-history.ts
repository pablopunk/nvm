import type { Clipboard } from 'electron';
import {
  type ClipboardHistoryItem,
  clipboardFilePaths,
  clipboardItemSubtitle,
  clipboardItemTitle,
} from './clipboard-utils';

export type ClipboardHistoryDeps = {
  // ── State ──────────────────────────────────────────────
  getHistory: () => any[];
  setHistory: (history: any[]) => void;
  getSuppressedItemIds: () => Map<string, number>;
  getImagesDir: () => string;

  // ── Electron clipboard ─────────────────────────────────
  clipboard: Clipboard;
  nativeImage: {
    createFromDataURL: (dataUrl: string) => Electron.NativeImage;
    createFromPath: (path: string) => Electron.NativeImage;
    createEmpty: () => Electron.NativeImage;
  };

  // ── Filesystem ─────────────────────────────────────────
  ensureDir: (dir: string) => Promise<unknown>;
  writeFile: (filePath: string, data: Buffer) => Promise<void>;

  // ── Utilities ──────────────────────────────────────────
  hashValue: (value: unknown) => string;
  fileUrlForPath: (filePath: string) => string;
  thumbnailUrlForPath: (filePath: string) => string;
  isVideoPath: (filePath: string) => boolean;
  expandUserPath: (value: string) => string;
  isImagePath: (filePath: string) => boolean;
  extensionForPath: (filePath: string) => string;
  pathJoin: (...segments: string[]) => string;
  pathBasename: (p: string, ext?: string) => string;
  pathToFileURL: (filePath: string) => { href: string };

  // ── Logging / metrics ──────────────────────────────────
  logWarn: (message: string, data?: unknown, opts?: unknown) => void;
  measureSync: <T>(
    name: string,
    data: Record<string, unknown>,
    fn: () => T,
  ) => T;

  // ── Cross-cutting callbacks ────────────────────────────
  scheduleSaveState: () => void;
  invalidateExtensionRootItems: () => void;
  emitChanged: () => void;
  sendToRenderer: (channel: string, ...args: unknown[]) => void;
  patchOpenView: (viewId: string, patch: Record<string, unknown>) => void;
  pasteIntoFrontmostApp: () => void;

  // ── Settings ───────────────────────────────────────────
  getSetting: (id: string) => unknown;

  // ── UI builders ────────────────────────────────────────
  buildPreviewItemAction: (opts: Record<string, unknown>) => unknown;
  rankAction: (item: unknown, query: string) => boolean;

  // ── File helpers ───────────────────────────────────────
  fileToExtensionFile: (filePath: string) => Promise<unknown>;

  // ── Desktop helpers ────────────────────────────────────
  findFiles: (
    roots: string[],
    opts?: Record<string, unknown>,
  ) => Promise<unknown[]>;
  selectedFilePaths: () => Promise<string[]>;
  selectedExtensionFiles: () => Promise<unknown[]>;
  selectedText: () => Promise<string>;
  selectedFiles: () => Promise<string[]>;
  frontmostApp: () => Promise<{ name: string; path: string } | null>;
  readDesktopSelection: (opts?: Record<string, unknown>) => Promise<unknown>;

  // ── Constants ──────────────────────────────────────────
  CLIPBOARD_LIMIT: number;
  CLIPBOARD_POLL_INTERVAL_MS: number;
  CLIPBOARD_LAST_HOUR_MS: number;
  CLIPBOARD_LAST_DAY_MS: number;
};

// ───────────────────────────────────────────────────────────
// Factory
// ───────────────────────────────────────────────────────────

export function createClipboardHistory(deps: ClipboardHistoryDeps) {
  let watcherLastId = '';

  // ── helpers ─────────────────────────────────────────────

  function measureSync<T>(
    name: string,
    data: Record<string, unknown>,
    fn: () => T,
  ) {
    return deps.measureSync(name, data, fn);
  }

  // ── clipboard read / persist ────────────────────────────

  async function persistClipboardImage(
    png: Buffer,
    hash: string,
  ): Promise<string | null> {
    const imagePath = deps.pathJoin(deps.getImagesDir(), `${hash}.png`);
    try {
      await deps.ensureDir(deps.getImagesDir());
      await deps.writeFile(imagePath, png);
      return imagePath;
    } catch (error) {
      deps.logWarn('clipboard.image.persist.failed', error, {
        source: 'host',
        scope: 'clipboard',
      });
      return null;
    }
  }

  async function readClipboardItem(): Promise<any | null> {
    const filePath = clipboardFilePath(deps.clipboard);
    if (filePath && deps.isVideoPath(filePath)) {
      return {
        id: `video:${deps.hashValue(filePath)}`,
        type: 'video',
        filePath,
        videoUrl: deps.fileUrlForPath(filePath),
        thumbnailUrl: deps.thumbnailUrlForPath(filePath),
        createdAt: Date.now(),
      };
    }

    const text = deps.clipboard.readText().trim();
    if (text) {
      return {
        id: `text:${deps.hashValue(text)}`,
        type: 'text',
        text,
        createdAt: Date.now(),
      };
    }

    const image = deps.clipboard.readImage();
    if (image.isEmpty()) return null;

    const png = image.toPNG();
    const hash = deps.hashValue(png);
    const imagePath = await persistClipboardImage(png, hash);
    if (!imagePath) return null;
    return {
      id: `image:${hash}`,
      type: 'image',
      imagePath,
      imageDataUrl: deps.fileUrlForPath(imagePath),
      thumbnailUrl: deps.thumbnailUrlForPath(imagePath),
      createdAt: Date.now(),
    };
  }

  // ── history mutation ────────────────────────────────────

  function rememberClipboardItem(item: any) {
    if (!item) return;
    const history = deps.getHistory();
    const previousIds = new Set(history.map((entry) => entry.id));
    deps.setHistory(
      [item, ...history.filter((current) => current.id !== item.id)].slice(
        0,
        deps.CLIPBOARD_LIMIT,
      ),
    );
    deps.scheduleSaveState();
    deps.invalidateExtensionRootItems();
    deps.emitChanged();
    deps.sendToRenderer('clipboard:changed');
    const currentIds = new Set(deps.getHistory().map((entry) => entry.id));
    const removeItemIds = [...previousIds]
      .filter((id) => !currentIds.has(id))
      .map((id) => `clipboard:${id}`);
    deps.patchOpenView('clipboard-history', {
      mode: 'prepend',
      items: [clipboardHistoryItem(item)],
      removeItemIds,
    });
  }

  // ── history query ───────────────────────────────────────

  function clipboardHistorySnapshot(options: any = {}) {
    return measureSync(
      'clipboard.snapshot',
      {
        queryLength: String(options.query || '').length,
        clipboardCount: deps.getHistory().length,
        limit: options.limit,
      },
      () => {
        const { limit, query, types } = options;
        let entries = deps.getHistory();
        if (Array.isArray(types) && types.length)
          entries = entries.filter((entry) => types.includes(entry.type));
        if (query) {
          const needle = String(query).toLowerCase();
          entries = entries.filter((entry) =>
            `${entry.text || ''} ${entry.type || ''} ${entry.filePath || ''}`
              .toLowerCase()
              .includes(needle),
          );
        }
        const max = typeof limit === 'number' ? limit : deps.CLIPBOARD_LIMIT;
        return entries.slice(0, max).map((entry) => ({
          id: entry.id,
          type: entry.type,
          text: entry.text,
          imageDataUrl: entry.imageDataUrl,
          imagePath: entry.imagePath,
          videoUrl: entry.videoUrl,
          filePath: entry.filePath,
          thumbnailUrl: entry.thumbnailUrl,
          createdAt: entry.createdAt,
        }));
      },
    );
  }

  function clipboardHistoryRemovalEntries(action: any) {
    const range = action?.clipboardHistoryRange || 'item';
    const now = Date.now();
    const types = new Set(
      Array.isArray(action?.types) ? action.types.map(String) : [],
    );
    const typeMatches = (entry: any) => !types.size || types.has(entry.type);
    const history = deps.getHistory();
    if (range === 'item')
      return history.filter(
        (entry) =>
          entry.id === action?.clipboardHistoryItemId && typeMatches(entry),
      );
    if (range === 'ids') {
      const ids = new Set(
        Array.isArray(action?.clipboardHistoryItemIds)
          ? action.clipboardHistoryItemIds
          : [action?.clipboardHistoryItemId].filter(Boolean),
      );
      return history.filter((entry) => ids.has(entry.id) && typeMatches(entry));
    }
    if (range === 'last-hour')
      return history.filter(
        (entry) =>
          (entry.createdAt || 0) >= now - deps.CLIPBOARD_LAST_HOUR_MS &&
          typeMatches(entry),
      );
    if (range === 'last-day')
      return history.filter(
        (entry) =>
          (entry.createdAt || 0) >= now - deps.CLIPBOARD_LAST_DAY_MS &&
          typeMatches(entry),
      );
    if (range === 'older-than')
      return history.filter(
        (entry) =>
          (entry.createdAt || 0) <
            now - Math.max(0, Number(action?.olderThanMs || 0)) &&
          typeMatches(entry),
      );
    if (range === 'all') return history.filter(typeMatches);
    return [];
  }

  function clipboardHistoryGet(id: string) {
    return clipboardHistorySnapshot().find((entry) => entry.id === id) || null;
  }

  function removeClipboardHistoryByAction(action: any) {
    const removed = clipboardHistoryRemovalEntries(action);
    if (removed.length === 0) return 0;
    const removedIds = new Set(removed.map((entry) => entry.id));
    deps.setHistory(
      deps.getHistory().filter((entry) => !removedIds.has(entry.id)),
    );
    deps.scheduleSaveState();
    deps.invalidateExtensionRootItems();
    deps.emitChanged();
    deps.sendToRenderer('clipboard:changed');
    return removed.length;
  }

  function clipboardHistoryRemovedMessage(count: number) {
    return count === 1
      ? 'Removed 1 clipboard item'
      : `Removed ${count} clipboard items`;
  }

  function removeClipboardHistoryEntries(action: any) {
    const removed = removeClipboardHistoryByAction(action);
    if (removed === 0)
      return {
        toast: { message: 'No matching clipboard items to remove' },
      };
    return {
      view: clipboardHistoryView(),
      navigation: 'replace',
      toast: { message: clipboardHistoryRemovedMessage(removed) },
    };
  }

  // ── history view / UI builders ──────────────────────────

  const CLIPBOARD_ITEM_APPEARANCE = { foreground: 'blue' } as const;

  function clipboardPreviewAction(item: any) {
    return deps.buildPreviewItemAction({
      kind:
        item.type === 'image'
          ? 'image'
          : item.type === 'video'
            ? 'video'
            : item.filePath
              ? 'file'
              : 'clipboard',
      clipboardType: item.type,
      text: item.text,
      imageDataUrl: item.imageDataUrl,
      imagePath: item.imagePath,
      videoUrl: item.videoUrl,
      filePath: item.filePath,
      thumbnailUrl: item.thumbnailUrl,
    });
  }

  function clipboardCopyAction(item: any) {
    if (item.type === 'image')
      return {
        type: 'copyImage',
        title: 'Copy Image',
        imageDataUrl: item.imageDataUrl,
        imagePath: item.imagePath,
        dismissAfterRun: 'auto',
      };
    return {
      type: 'copyText',
      title: item.type === 'video' ? 'Copy Video Path' : 'Copy Text',
      text: item.filePath || item.text,
      dismissAfterRun: 'auto',
    };
  }

  function clipboardHistoryRemovalAction(
    range: string,
    title: string,
    message: string,
    itemId = '',
  ) {
    return {
      type: 'removeClipboardHistory',
      title,
      clipboardHistoryRange: range,
      clipboardHistoryItemId: itemId,
      style: 'destructive',
      requiresConfirmation: true,
      confirmLabel: title,
      confirmMessage: message,
    };
  }

  function clipboardHistoryRemovalActions(item: any = null) {
    if (deps.getHistory().length === 0) return [];
    const actions: any[] = [];
    if (item?.id)
      actions.push(
        clipboardHistoryRemovalAction(
          'item',
          'Remove Item',
          `Remove “${clipboardItemTitle(item)}” from clipboard history?`,
          item.id,
        ),
      );
    actions.push(
      clipboardHistoryRemovalAction(
        'last-hour',
        'Remove Entries from Last Hour',
        'Remove clipboard history entries copied in the last hour?',
      ),
      clipboardHistoryRemovalAction(
        'last-day',
        'Remove Entries from Last Day',
        'Remove clipboard history entries copied in the last day?',
      ),
      clipboardHistoryRemovalAction(
        'all',
        'Remove All Entries',
        'Remove all clipboard history entries?',
      ),
    );
    return actions;
  }

  function clipboardRootItem(item: any) {
    return {
      id: `clipboard:${item.id}`,
      title: clipboardItemTitle(item),
      subtitle: clipboardItemSubtitle(item),
      icon: 'clipboard',
      image: item.thumbnailUrl,
      score: 60,
      lastUsed: item.createdAt || 0,
      appearance: CLIPBOARD_ITEM_APPEARANCE,
      primaryAction: clipboardCopyAction(item),
      actionPanel: {
        sections: [
          {
            actions: [
              clipboardPreviewAction(item),
              clipboardCopyAction(item),
            ].filter(Boolean),
          },
        ],
      },
    };
  }

  function clipboardHistoryItem(item: any) {
    const isImage = item.type === 'image';
    const isVideo = item.type === 'video';
    const copyAction = clipboardCopyAction(item);
    const previewAction = clipboardPreviewAction(item);
    const pasteAction =
      isImage || isVideo
        ? null
        : {
            type: 'pasteText',
            title: 'Paste Text',
            text: item.text,
            dismissAfterRun: 'auto',
          };
    return {
      id: `clipboard:${item.id}`,
      title: clipboardItemTitle(item),
      subtitle: clipboardItemSubtitle(item),
      icon: 'clipboard',
      image: item.thumbnailUrl,
      keywords: [
        item.text || '',
        item.type || '',
        `clipboard ${item.type || ''}`,
        isImage ? 'image photo picture screenshot' : '',
        isVideo ? 'video movie recording' : '',
      ].filter(Boolean),
      appearance: CLIPBOARD_ITEM_APPEARANCE,
      primaryAction: copyAction,
      actionPanel: {
        sections: [
          {
            actions: [previewAction, copyAction, pasteAction].filter(Boolean),
          },
          {
            title: 'Manage History',
            actions: clipboardHistoryRemovalActions(item),
          },
        ],
      },
    };
  }

  function clipboardHistoryItems() {
    return deps
      .getHistory()
      .slice(0, deps.CLIPBOARD_LIMIT)
      .map(clipboardHistoryItem);
  }

  function clipboardHistoryView() {
    const actions = clipboardHistoryRemovalActions();
    return {
      type: 'list',
      id: 'clipboard-history',
      title: 'Clipboard History',
      presentation: 'root',
      searchBarPlaceholder: 'Search Clipboard History',
      emptyView: {
        title: 'No clipboard items found.',
        subtitle: 'Copy text or images and they will appear here.',
      },
      actions,
      actionPanel: actions.length
        ? { sections: [{ title: 'Manage History', actions }] }
        : undefined,
      items: clipboardHistoryItems(),
    };
  }

  // ── polling ─────────────────────────────────────────────

  async function pollClipboardChange() {
    const item = await readClipboardItem();
    if (!item || item.id === watcherLastId) return;
    const suppressed = deps.getSuppressedItemIds();
    const suppressUntil = suppressed.get(item.id) || 0;
    if (suppressUntil > Date.now()) {
      watcherLastId = item.id;
      return;
    }
    if (suppressUntil) suppressed.delete(item.id);
    watcherLastId = item.id;
    rememberClipboardItem(item);
  }

  // ── snapshot ────────────────────────────────────────────

  function clipboardSnapshot() {
    const image = deps.clipboard.readImage();
    return {
      text: deps.clipboard.readText(),
      html: deps.clipboard.readHTML(),
      rtf: deps.clipboard.readRTF(),
      bookmark: deps.clipboard.readBookmark(),
      image: image.isEmpty() ? null : image,
    };
  }

  function restoreClipboardSnapshot(
    snapshot: ReturnType<typeof clipboardSnapshot>,
  ) {
    if (!snapshot) return;
    const data: any = {};
    if (snapshot.text) data.text = snapshot.text;
    if (snapshot.html) data.html = snapshot.html;
    if (snapshot.rtf) data.rtf = snapshot.rtf;
    if (snapshot.bookmark?.title || snapshot.bookmark?.url)
      data.bookmark = snapshot.bookmark;
    if (snapshot.image && !snapshot.image.isEmpty())
      data.image = snapshot.image;
    if (Object.keys(data).length === 0) deps.clipboard.clear();
    else deps.clipboard.write(data);
  }

  function clipboardHistoryIdForText(text: string) {
    const value = String(text || '').trim();
    return value ? `text:${deps.hashValue(value)}` : '';
  }

  function suppressClipboardHistoryId(id: string, durationMs = 2_000) {
    if (id) deps.getSuppressedItemIds().set(id, Date.now() + durationMs);
  }

  // ── paste actions ───────────────────────────────────────

  function pasteTextAction(action: any) {
    const text = String(action.text || '');
    const restoreClipboard = Boolean(action.restoreClipboard);
    const concealed = Boolean(action.concealed || restoreClipboard);
    const snapshot = restoreClipboard ? clipboardSnapshot() : null;
    const suppressedId = clipboardHistoryIdForText(text);
    if (concealed) suppressClipboardHistoryId(suppressedId);
    if (action.plainText === false && action.html)
      deps.clipboard.write({ text, html: String(action.html) });
    else deps.clipboard.writeText(text);
    deps.pasteIntoFrontmostApp();
    if (restoreClipboard && snapshot) {
      const delay = Math.max(
        50,
        Math.min(5_000, Number(action.restoreDelayMs || 250)),
      );
      setTimeout(() => {
        suppressClipboardHistoryId(clipboardHistoryIdForText(snapshot.text));
        restoreClipboardSnapshot(snapshot);
      }, delay).unref?.();
    }
  }

  // ── desktop clipboard ───────────────────────────────────

  function clipboardFilePath(clipboard: Clipboard) {
    return clipboardFilePaths(clipboard)[0] || null;
  }

  async function clipboardFiles() {
    return Promise.all(
      clipboardFilePaths(deps.clipboard).map(deps.fileToExtensionFile),
    );
  }

  function clipboardImageDataUrl() {
    const image = deps.clipboard.readImage();
    return image.isEmpty() ? null : image.toDataURL();
  }

  function clipboardFormats(options: any = {}) {
    const formats = Array.isArray(options.formats)
      ? options.formats.map(String)
      : [];
    return formats.length ? new Set(formats) : null;
  }

  async function readDesktopClipboard(options: any = {}) {
    const formats = clipboardFormats(options);
    if (!formats || formats.has('files')) {
      const files = await clipboardFiles();
      if (files.length)
        return {
          type: 'files',
          files,
          paths: files.map((file: any) => file.path),
        };
    }
    if (!formats || formats.has('image')) {
      const image = clipboardImageDataUrl();
      if (image) return { type: 'image', imageDataUrl: image, image };
    }
    if (!formats || formats.has('html')) {
      const html = deps.clipboard.readHTML();
      if (html)
        return {
          type: 'html',
          html,
          text: deps.clipboard.readText(),
        };
    }
    if (!formats || formats.has('text')) {
      const text = deps.clipboard.readText();
      if (text)
        return {
          type: 'text',
          text,
          html: deps.clipboard.readHTML() || undefined,
        };
    }
    return { type: 'empty' };
  }

  function clipboardImageForContent(item: any) {
    const image = item?.image || item?.imageDataUrl || item?.path;
    if (!image) return null;
    return String(image).startsWith('data:')
      ? deps.nativeImage.createFromDataURL(String(image))
      : deps.nativeImage.createFromPath(deps.expandUserPath(String(image)));
  }

  function suppressClipboardHistoryForContent(item: any) {
    if (!item) return;
    if (typeof item === 'string')
      return suppressClipboardHistoryId(clipboardHistoryIdForText(item));
    const text = item.text || (item.type === 'html' ? item.html : '');
    if (text)
      suppressClipboardHistoryId(clipboardHistoryIdForText(String(text)));
    const image = clipboardImageForContent(item);
    if (image && !image.isEmpty())
      suppressClipboardHistoryId(`image:${deps.hashValue(image.toPNG())}`);
    const paths = Array.isArray(item.paths)
      ? item.paths
      : Array.isArray(item.files)
        ? item.files.map((file: any) => file.path || file).filter(Boolean)
        : [];
    for (const filePath of paths)
      if (deps.isVideoPath(String(filePath)))
        suppressClipboardHistoryId(
          `video:${deps.hashValue(deps.expandUserPath(String(filePath)))}`,
        );
  }

  function writeDesktopClipboardFiles(paths: any) {
    const resolvedPaths = (Array.isArray(paths) ? paths : [paths])
      .map((filePath: any) => deps.expandUserPath(String(filePath)))
      .filter(Boolean);
    const fileUrls = resolvedPaths
      .map((filePath: any) => deps.pathToFileURL(filePath).href)
      .join('\n');
    deps.clipboard.write({ text: resolvedPaths.join('\n') });
    if (fileUrls)
      deps.clipboard.writeBuffer(
        'public.file-url',
        Buffer.from(fileUrls, 'utf8'),
      );
  }

  function writeDesktopClipboard(item: any, options: any = {}) {
    const content =
      typeof item === 'string' ? { type: 'text', text: item } : item || {};
    if (content.concealed || options.concealed)
      suppressClipboardHistoryForContent(content);
    if (
      content.type === 'files' ||
      Array.isArray(content.paths) ||
      Array.isArray(content.files)
    )
      return writeDesktopClipboardFiles(content.paths || content.files);
    if (content.type === 'html' || content.html != null)
      return deps.clipboard.write({
        text: String(content.text || ''),
        html: String(content.html || ''),
      });
    if (content.type === 'text' || content.text != null)
      return content.html
        ? deps.clipboard.write({
            text: String(content.text || ''),
            html: String(content.html),
          })
        : deps.clipboard.writeText(String(content.text || ''));
    const image = clipboardImageForContent(content);
    if (content.type === 'image' || image)
      return deps.clipboard.writeImage(image || deps.nativeImage.createEmpty());
  }

  function pasteClipboardAction(action: any) {
    const restoreClipboard = Boolean(action.restoreClipboard);
    const snapshot = restoreClipboard ? clipboardSnapshot() : null;
    const content = action.content || action.clipboard || action;
    writeDesktopClipboard(content, {
      concealed: action.concealed || restoreClipboard,
    });
    deps.pasteIntoFrontmostApp();
    if (restoreClipboard && snapshot) {
      const delay = Math.max(
        50,
        Math.min(5_000, Number(action.restoreDelayMs || 250)),
      );
      setTimeout(() => restoreClipboardSnapshot(snapshot), delay).unref?.();
    }
  }

  // ── extension ───────────────────────────────────────────

  function createClipboardExtension() {
    function historyItem() {
      const latestClipboardTime = deps.getHistory()[0]?.createdAt || 0;
      return {
        id: 'clipboard-history',
        title: 'Clipboard History',
        subtitle: deps.getHistory().length
          ? `Show all ${deps.getHistory().length} copied items`
          : 'Show copied items',
        icon: 'clipboard',
        score: 14,
        lastUsed: latestClipboardTime ? latestClipboardTime - 1 : 0,
        primaryAction: {
          type: 'pushView',
          title: 'Clipboard History',
          view: clipboardHistoryView(),
        },
      };
    }

    return {
      id: 'nevermind.clipboard',
      title: 'Clipboard',
      permissions: ['clipboard.history'] as const,
      commands: [
        {
          id: 'clipboard-history',
          actionId: 'clipboard-history',
          title: 'Clipboard History',
          subtitle: 'Show copied items',
          icon: 'clipboard',
          score: 14,
          run: () => clipboardHistoryView(),
        },
      ],
      rootItems(ctx: any) {
        if (!deps.getSetting('showClipboardInRoot')) return [];
        return ctx.clipboard.history.list({ limit: 10 }).map(clipboardRootItem);
      },
      searchItems(ctx: any, query: string) {
        return ctx.clipboard.history
          .list()
          .map(clipboardRootItem)
          .filter((item: any) => deps.rankAction(item, query))
          .slice(0, 5);
      },
    };
  }

  // ── poller registration (caller passes jobRegistry register) ─

  async function startClipboardWatcher() {
    watcherLastId = (await readClipboardItem())?.id || '';
    return {
      id: 'clipboard.poll',
      title: 'Clipboard Poll',
      owner: 'host' as const,
      scope: 'clipboard',
      triggers: [
        {
          type: 'interval' as const,
          everyMs: deps.CLIPBOARD_POLL_INTERVAL_MS,
          delayMs: deps.CLIPBOARD_POLL_INTERVAL_MS,
        },
      ],
      timeoutMs: 2_000,
      run: () => pollClipboardChange(),
    };
  }

  // ── clipboard-only search helper ────────────────────────

  function clipboardOnlySearch(query: string): any[] {
    const q = query.trim();
    return deps
      .getHistory()
      .map(clipboardRootItem)
      .filter((item) => (q ? deps.rankAction(item, q) : true))
      .sort((a, b) =>
        q
          ? b.score - a.score || b.lastUsed - a.lastUsed
          : b.lastUsed - a.lastUsed,
      )
      .slice(0, deps.CLIPBOARD_LIMIT);
  }

  // ── public API ──────────────────────────────────────────

  return {
    // Phase 1 fix: async persist
    persistClipboardImage,
    // Core
    readClipboardItem,
    rememberClipboardItem,
    pollClipboardChange,
    startClipboardWatcher,
    // History CRUD
    clipboardHistorySnapshot,
    clipboardHistoryRemovalEntries,
    clipboardHistoryGet,
    removeClipboardHistoryByAction,
    clipboardHistoryRemovedMessage,
    removeClipboardHistoryEntries,
    clipboardHistoryView,
    clipboardHistoryIdForText,
    suppressClipboardHistoryId,
    // UI builders
    clipboardPreviewAction,
    clipboardCopyAction,
    clipboardHistoryRemovalAction,
    clipboardHistoryRemovalActions,
    clipboardRootItem,
    clipboardHistoryItem,
    clipboardHistoryItems,
    // Snapshot
    clipboardSnapshot,
    restoreClipboardSnapshot,
    // Desktop clipboard
    clipboardFiles,
    clipboardImageDataUrl,
    clipboardFormats,
    readDesktopClipboard,
    clipboardImageForContent,
    suppressClipboardHistoryForContent,
    writeDesktopClipboardFiles,
    writeDesktopClipboard,
    // Paste
    pasteTextAction,
    pasteClipboardAction,
    // Extension
    createClipboardExtension,
    // Search
    clipboardOnlySearch,
  };
}
