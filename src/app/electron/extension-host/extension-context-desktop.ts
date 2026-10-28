// biome-ignore-all lint: Host-side extension payload boundary keeps dynamic desktop API shapes by design.
import { execFile } from 'node:child_process';
import fs from 'node:fs/promises';
import { shell } from 'electron';
import {
  fileToExtensionFile,
  thumbnailUrlForPreviewablePath,
} from '../files/extension-file';
import { defaultFileIndexRoots } from '../files/file-index';
import {
  displayUserPath,
  expandUserPath,
  fileUrlForPath,
} from '../files/file-utils';
import { findFiles } from '../files/find-files';
import { ocrImage, ocrScreen } from '../files/ocr';
import {
  type createOpenWithApps,
  quickLookPath,
} from '../files/quick-look-and-open-with';
import type { createElectronClipboardApi } from '../os/electron-clipboard';
import {
  frontmostApp,
  frontmostAppFocusTarget,
  hasCapability,
  keyboardSettingsSubtitle,
  osLabel,
  quickLookTitle,
  replaceSelectedText,
  revealPathTitle,
  selectedTextAccess,
  settingsTitle,
  typeTextIntoFrontmostApp,
} from '../os/os';
import { limitedOutput } from '../platform/limited-output';
import { openExternalUrl } from '../platform/url-utils';
import { trustedContextCapabilities } from './extension-context-capabilities';
import {
  createDesktopWindowManagement,
  type DesktopWindowManagementDeps,
} from './extension-desktop-window-management';

export type ContextDesktopDeps = {
  clipboardApi: Pick<
    ReturnType<typeof createElectronClipboardApi>,
    'readText' | 'readHTML'
  >;
  writeDesktopClipboard: (item: any, options?: any) => unknown;
  clipboardImageDataUrl: () => Promise<unknown>;
  clipboardFiles: () => Promise<unknown>;
  readDesktopClipboard: (options?: any) => Promise<unknown>;
  selectedText: () => Promise<string>;
  readDesktopSelection: () => Promise<unknown>;
  selectedFiles: () => Promise<unknown>;
  appIndexService: { get: () => any[] };
  appIconCache: { get: (appPath: string) => unknown };
  openWithApps: ReturnType<typeof createOpenWithApps>;
  fileIndexSnapshot: (options?: any) => unknown;
  reindexFiles: (options?: any) => unknown;
  runInBackground: (task: () => unknown, traceId?: string) => unknown;
  runShellCommand: (command: any, args?: any, options?: any) => unknown;
  runShellScript: (script: any, options?: any) => unknown;
  shellSpawnEnv: (extraEnv?: Record<string, string>) => Promise<any>;
  windowManagement?: DesktopWindowManagementDeps;
};

function createDesktopKeyboard() {
  return {
    typeText: (text, options: any = {}) =>
      typeTextIntoFrontmostApp(String(text || ''), options),
  };
}

function createDesktopClipboard(deps: ContextDesktopDeps) {
  const { canUseClipboard } = trustedContextCapabilities;
  const {
    clipboardApi,
    writeDesktopClipboard,
    clipboardImageDataUrl,
    clipboardFiles,
    readDesktopClipboard,
  } = deps;
  return canUseClipboard
    ? {
        readText: () => clipboardApi.readText(),
        writeText: (text, options: any = {}) =>
          writeDesktopClipboard({ type: 'text', text }, options),
        readHtml: () => clipboardApi.readHTML(),
        writeHtml: (html, text = '', options: any = {}) =>
          writeDesktopClipboard({ type: 'html', html, text }, options),
        readImage: clipboardImageDataUrl,
        writeImage: (image, options: any = {}) =>
          writeDesktopClipboard({ type: 'image', image }, options),
        readFiles: clipboardFiles,
        writeFiles: (paths, options: any = {}) =>
          writeDesktopClipboard({ type: 'files', paths }, options),
        read: readDesktopClipboard,
        write: writeDesktopClipboard,
      }
    : undefined;
}

function createDesktopSelection(deps: ContextDesktopDeps) {
  const { selectedText, selectedFiles, readDesktopSelection } = deps;
  return {
    access: selectedTextAccess,
    text: selectedText,
    replaceText: async (text) => {
      const target = await frontmostAppFocusTarget();
      return target ? replaceSelectedText(target, String(text || '')) : false;
    },
    files: selectedFiles,
    read: readDesktopSelection,
  };
}

function createDesktopApps(deps: ContextDesktopDeps) {
  const { canUseDesktopApps } = trustedContextCapabilities;
  const { runInBackground, appIndexService, appIconCache } = deps;
  return canUseDesktopApps
    ? {
        frontmost: frontmostApp,
        launch: (appPath) =>
          runInBackground(() => shell.openPath(expandUserPath(appPath))),
        list: () =>
          appIndexService.get().map((entry) => ({
            id: entry.id,
            name: entry.name,
            path: entry.path,
            firstSeenAt: entry.firstSeenAt,
          })),
        search: (query) => {
          const needle = String(query || '').toLowerCase();
          return appIndexService
            .get()
            .filter(
              (entry) =>
                !needle ||
                String(entry.name || '')
                  .toLowerCase()
                  .includes(needle),
            )
            .map((entry) => ({
              id: entry.id,
              name: entry.name,
              path: entry.path,
              firstSeenAt: entry.firstSeenAt,
            }));
        },
        icon: (appPath) => appIconCache.get(appPath),
      }
    : undefined;
}

function createDesktopFiles(deps: ContextDesktopDeps) {
  const { canUseDesktopFiles } = trustedContextCapabilities;
  const { openWithApps, runInBackground, fileIndexSnapshot, reindexFiles } =
    deps;
  return canUseDesktopFiles
    ? {
        find: findFiles,
        findImages: (roots, options) =>
          findFiles(roots, { ...options, kind: 'image' }),
        findVideos: (roots, options) =>
          findFiles(roots, { ...options, kind: 'video' }),
        findMedia: (roots, options) =>
          findFiles(roots, { ...options, kind: 'media' }),
        openWithApps,
        open: (filePath) =>
          runInBackground(() => shell.openPath(expandUserPath(filePath))),
        reveal: (filePath) =>
          runInBackground(() =>
            shell.showItemInFolder(expandUserPath(filePath)),
          ),
        preview: quickLookPath,
        readText: (filePath) => fs.readFile(expandUserPath(filePath), 'utf8'),
        displayPath: (filePath) => displayUserPath(expandUserPath(filePath)),
        toFileUrl: (filePath) => fileUrlForPath(expandUserPath(filePath)),
        thumbnail: (filePath) => thumbnailUrlForPreviewablePath(filePath),
        metadata: (filePath) =>
          fileToExtensionFile(filePath, { includeDimensions: true }),
        indexedRoots: () => defaultFileIndexRoots().map(displayUserPath),
        indexSnapshot: (options: any = {}) => fileIndexSnapshot(options),
        reindex: (options: any = {}) => reindexFiles(options),
        recent: (options: any = {}) => fileIndexSnapshot(options),
        searchIndex: (query, options: any = {}) =>
          fileIndexSnapshot({ ...options, query }),
      }
    : undefined;
}

function createDesktopShell(deps: ContextDesktopDeps) {
  const { canUseSystem } = trustedContextCapabilities;
  const { runShellCommand, runShellScript, shellSpawnEnv } = deps;
  return canUseSystem
    ? {
        openExternal: async (url) => {
          if (!(await openExternalUrl(url)))
            throw new Error('Unsafe external URL');
        },
        exec: runShellCommand,
        script: runShellScript,
        appleScript: (script, options: any = {}) =>
          new Promise((resolve) => {
            if (!hasCapability('applescript'))
              return resolve({
                stdout: '',
                stderr: 'AppleScript is not available on this OS',
                exitCode: 1,
              });
            execFile(
              'osascript',
              ['-e', String(script)],
              { timeout: Number(options.timeout || 30_000) },
              (error, stdout, stderr) =>
                resolve({
                  stdout: limitedOutput(stdout, options.outputLimit),
                  stderr: limitedOutput(
                    stderr || error?.message || '',
                    options.outputLimit,
                  ),
                  exitCode: error ? 1 : 0,
                }),
            );
          }),
        which: async (command) => {
          const env = await shellSpawnEnv();
          return new Promise((resolve) => {
            execFile(
              '/usr/bin/which',
              [String(command)],
              { env },
              (error, stdout, stderr) =>
                resolve({
                  stdout: stdout.trim(),
                  stderr: stderr || error?.message || '',
                  exitCode: error ? 1 : 0,
                }),
            );
          });
        },
      }
    : undefined;
}

export function createContextDesktop(deps: ContextDesktopDeps) {
  return {
    keyboard: createDesktopKeyboard(),
    clipboard: createDesktopClipboard(deps),
    selection: createDesktopSelection(deps),
    apps: createDesktopApps(deps),
    files: createDesktopFiles(deps),
    shell: createDesktopShell(deps),
    windowManagement: createDesktopWindowManagement(deps.windowManagement),
  };
}

export function createContextSystem() {
  return {
    os: osLabel(),
    capabilities: { has: (id) => hasCapability(String(id)) },
    labels: {
      revealInFileManager: revealPathTitle(),
      previewFile: quickLookTitle(),
      openSystemSettings: settingsTitle(),
      keyboardSettings: keyboardSettingsSubtitle(),
    },
  };
}

export type ContextClipboardHistoryDeps = {
  clipboardHistorySnapshot: (options?: any) => unknown;
  clipboardHistoryGet: (id: string) => unknown;
  removeClipboardHistoryByAction: (action: any) => unknown;
};

export function createContextClipboardHistory(
  deps: ContextClipboardHistoryDeps,
) {
  const { canUseClipboard } = trustedContextCapabilities;
  const {
    clipboardHistorySnapshot,
    clipboardHistoryGet,
    removeClipboardHistoryByAction,
  } = deps;
  return {
    history: canUseClipboard
      ? {
          list: (options: any = {}) => clipboardHistorySnapshot(options),
          search: (query, options: any = {}) =>
            clipboardHistorySnapshot({ ...options, query }),
          get: (id) => clipboardHistoryGet(String(id || '')),
          remove: async (idOrIds) => ({
            removed: removeClipboardHistoryByAction({
              clipboardHistoryRange: 'ids',
              clipboardHistoryItemIds: Array.isArray(idOrIds)
                ? idOrIds
                : [idOrIds],
            }),
          }),
          clear: async (options: any = {}) => ({
            removed: removeClipboardHistoryByAction({
              clipboardHistoryRange: options.olderThanMs ? 'older-than' : 'all',
              olderThanMs: options.olderThanMs,
              types: options.types,
            }),
          }),
        }
      : undefined,
  };
}

export function createContextOcr() {
  const { canUseOcr } = trustedContextCapabilities;
  return canUseOcr
    ? {
        image: ocrImage,
        screen: (options: any = {}) => ocrScreen(options),
        region: (rect, options: any = {}) =>
          ocrScreen({ ...options, region: rect }),
      }
    : undefined;
}
