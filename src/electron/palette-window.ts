import {
  app,
  BrowserWindow,
  type BrowserWindowConstructorOptions,
  globalShortcut,
  screen,
  session,
} from 'electron';
import {
  markDebugPerformance,
  measureDebugPerformanceSync,
} from './debug-performance';
import * as logger from './logger';
import {
  applyPaletteWindowPolicy as applyOsPaletteWindowPolicy,
  canRequestMediaPermission,
  paletteBrowserWindowOptions,
} from './os';
import {
  installExternalNavigationPolicy,
  isTrustedAppPage,
} from './window-navigation-policy';
import {
  isNvmTestMode,
  recordPackagedStartupReady,
  recordTestWindowEvent,
} from './test-mode';

export type PaletteMode = 'default' | 'ai-chat' | 'stacked' | 'preview';

type PaletteWindowOptions = {
  isDev: boolean;
  preloadPath: string;
  rendererUrl?: string;
  rendererIndexPath: string;
  getPaletteHotkey: () => string;
};

const WINDOW_BLUR_MARGIN = 96;
const DEFAULT_PALETTE_SIZE = { width: 720, height: 480 };
const AI_CHAT_PALETTE_SIZE = { width: 760, height: 560 };
const STACKED_PALETTE_SIZE = { width: 760, height: 720 };
const PREVIEW_PALETTE_SIZE = { width: 1080, height: 760 };
const DEFAULT_WINDOW_SIZE = addWindowBlurMargin(DEFAULT_PALETTE_SIZE);
const AI_CHAT_WINDOW_SIZE = addWindowBlurMargin(AI_CHAT_PALETTE_SIZE);
const STACKED_WINDOW_SIZE = addWindowBlurMargin(STACKED_PALETTE_SIZE);
const PREVIEW_WINDOW_SIZE = addWindowBlurMargin(PREVIEW_PALETTE_SIZE);

function addWindowBlurMargin(size: { width: number; height: number }) {
  return {
    width: size.width + WINDOW_BLUR_MARGIN * 2,
    height: size.height + WINDOW_BLUR_MARGIN * 2,
  };
}

export function installPermissionHandlers(
  isDev: boolean,
  rendererUrl = process.env.ELECTRON_RENDERER_URL || '',
  rendererIndexPath?: string,
) {
  const allowedPermissions = [
    'media',
    'display-capture',
    'clipboard-read',
    'clipboard-sanitized-write',
  ];
  session.defaultSession.setPermissionCheckHandler(
    (webContents, permission) => {
      const url = webContents?.getURL() || '';
      return (
        isTrustedAppPage(url, isDev, rendererUrl, rendererIndexPath) &&
        allowedPermissions.includes(permission) &&
        canRequestMediaPermission(permission)
      );
    },
  );
  session.defaultSession.setPermissionRequestHandler(
    (webContents, permission, callback) => {
      const url = webContents.getURL();
      if (
        isTrustedAppPage(url, isDev, rendererUrl, rendererIndexPath) &&
        allowedPermissions.includes(permission) &&
        canRequestMediaPermission(permission)
      )
        return callback(true);
      callback(false);
    },
  );
}

export function createPaletteWindowController(options: PaletteWindowOptions) {
  let win: BrowserWindow | null = null;
  let ignorePaletteBlurUntil = 0;
  let pendingShowOnReady = false;
  let currentPaletteMode: PaletteMode = 'default';

  function debugLog(message: string, data?: unknown) {
    logger.debug(message, data, { source: 'host', scope: 'palette-window' });
  }

  function dismissAfterFocusLoss(reason: string) {
    const remainingBlurGrace = ignorePaletteBlurUntil - Date.now();
    if (remainingBlurGrace > 0) {
      setTimeout(() => dismissAfterFocusLoss(reason), remainingBlurGrace);
      return;
    }
    if (!win?.isVisible() || win.isFocused()) return;
    debugLog('dismissAfterFocusLoss', { reason });
    hidePalette();
  }

  function createWindow() {
    win = new BrowserWindow({
      width: DEFAULT_WINDOW_SIZE.width,
      height: DEFAULT_WINDOW_SIZE.height,
      show: false,
      frame: false,
      transparent: true,
      resizable: false,
      movable: true,
      closable: false,
      minimizable: false,
      maximizable: false,
      ...paletteBrowserWindowOptions(),
      alwaysOnTop: true,
      skipTaskbar: true,
      focusable: true,
      fullscreenable: false,
      hasShadow: false,
      title: 'Nevermind',
      backgroundColor: '#00000000',
      webPreferences: {
        preload: options.preloadPath,
        contextIsolation: true,
        nodeIntegration: false,
        sandbox: false,
      },
    } satisfies BrowserWindowConstructorOptions);

    applyPaletteWindowPolicy();

    win.on('blur', () => dismissAfterFocusLoss('window-blur'));
    app.on('browser-window-blur', (_event, blurredWindow) => {
      if (blurredWindow === win) dismissAfterFocusLoss('browser-window-blur');
    });
    (
      app as typeof app & {
        on(event: 'deactivate', listener: () => void): Electron.App;
      }
    ).on('deactivate', () => dismissAfterFocusLoss('app-deactivate'));
    win.on('close', (event) => {
      if (!(app as typeof app & { isQuiting?: boolean }).isQuiting) {
        event.preventDefault();
        hidePalette();
      }
    });

    installExternalNavigationPolicy(win, (url) =>
      isTrustedAppPage(
        url,
        options.isDev,
        options.rendererUrl,
        options.rendererIndexPath,
      ),
    );
    win.webContents.on(
      'did-fail-load',
      (_event, errorCode, errorDescription, validatedURL) => {
        debugLog('renderer.didFailLoad', {
          errorCode,
          errorDescription,
          validatedURL,
        });
      },
    );
    win.webContents.on('render-process-gone', (_event, details) => {
      debugLog('renderer.gone', details);
    });
    win.webContents.on('console-message', (event) => {
      const { level, message, lineNumber, sourceId } = event;
      debugLog('renderer.console', {
        level,
        message,
        line: lineNumber,
        sourceId,
      });
    });
    win.webContents.once('did-finish-load', () => {
      debugLog('renderer.didFinishLoad', { url: win?.webContents.getURL() });
      if (options.isDev || pendingShowOnReady) {
        pendingShowOnReady = false;
        showPalette();
      }
    });
    win.once('ready-to-show', recordPackagedStartupReady);

    if (options.isDev && options.rendererUrl) win.loadURL(options.rendererUrl);
    else win.loadFile(options.rendererIndexPath);

    return win;
  }

  function setPaletteSizeForMode(mode: PaletteMode = 'default') {
    measureDebugPerformanceSync(
      'palette-window.set-mode',
      { mode, currentPaletteMode },
      () => {
        if (!win || mode === currentPaletteMode) return;
        currentPaletteMode = mode;
        const size =
          mode === 'preview'
            ? PREVIEW_WINDOW_SIZE
            : mode === 'stacked'
              ? STACKED_WINDOW_SIZE
              : mode === 'ai-chat'
                ? AI_CHAT_WINDOW_SIZE
                : DEFAULT_WINDOW_SIZE;
        win.setSize(size.width, size.height, false);
        if (win.isVisible()) centerWindow();
      },
    );
  }

  function centerWindow() {
    measureDebugPerformanceSync('palette-window.center', undefined, () => {
      if (!win) return;
      const cursor = screen.getCursorScreenPoint();
      const display = screen.getDisplayNearestPoint(cursor);
      const { width, height } = win.getBounds();
      const { x, y, width: sw, height: sh } = display.workArea;
      win.setBounds({
        x: Math.round(x + (sw - width) / 2),
        y: Math.round(y + Math.min(sh * 0.18, 180)),
        width,
        height,
      });
    });
  }

  function applyPaletteWindowPolicy() {
    applyOsPaletteWindowPolicy(win);
  }

  function showPalette(
    showOptions: { deferReveal?: boolean; skipShownEvent?: boolean } = {},
  ) {
    measureDebugPerformanceSync(
      'palette-window.show',
      { options: showOptions },
      () => {
        if (!win) return;
        markDebugPerformance('palette-window.show.start', {
          visible: win.isVisible(),
          bounds: win.getBounds(),
        });
        ignorePaletteBlurUntil = Date.now() + 500;
        applyPaletteWindowPolicy();
        debugLog('showPalette', {
          options: showOptions,
          visible: win.isVisible(),
          bounds: win.getBounds(),
        });
        centerWindow();
        if (showOptions.deferReveal) {
          win.setOpacity(0);
          setTimeout(() => win?.setOpacity(1), 250);
        } else {
          win.setOpacity(1);
        }
        if (showOptions.skipShownEvent)
          win.webContents.send('palette:shortcut-show');
        else win.webContents.send('palette:shown');
        win.show();
        if (isNvmTestMode) recordTestWindowEvent('shown');
        win.moveTop();
        win.focus();
        win.webContents.focus();
        debugLog('showPalette.after', {
          visible: win.isVisible(),
          focused: win.isFocused(),
          bounds: win.getBounds(),
          opacity: win.getOpacity(),
        });
        markDebugPerformance('palette-window.show.after', {
          visible: win.isVisible(),
          focused: win.isFocused(),
          bounds: win.getBounds(),
          opacity: win.getOpacity(),
        });
        setTimeout(
          () =>
            debugLog('showPalette.later', {
              visible: win?.isVisible(),
              focused: win?.isFocused(),
              bounds: win?.getBounds(),
              opacity: win?.getOpacity(),
            }),
          300,
        );
      },
    );
  }

  function revealPalette() {
    win?.setOpacity(1);
  }

  function hidePalette() {
    measureDebugPerformanceSync('palette-window.hide', undefined, () => {
      if (!win) return;
      debugLog('hidePalette', {
        visible: win.isVisible(),
        focused: win.isFocused(),
        bounds: win.getBounds(),
        opacity: win.getOpacity(),
      });
      if (isNvmTestMode) recordTestWindowEvent('hidden');
      win.webContents.send('palette:hidden');
      win.hide();
      if (isNvmTestMode) setTimeout(() => showPalette(), 100).unref?.();
    });
  }

  function showPaletteWhenReady() {
    if (!win) {
      pendingShowOnReady = true;
      return;
    }
    if (win.webContents.isLoading()) pendingShowOnReady = true;
    else showPalette();
  }

  function togglePalette() {
    if (win?.isVisible()) hidePalette();
    else showPalette();
  }

  function registerHotkey() {
    const hotkey = options.getPaletteHotkey();
    const ok = globalShortcut.register(hotkey, togglePalette);
    debugLog('registerHotkey', {
      accelerator: hotkey,
      ok,
      isRegistered: globalShortcut.isRegistered(hotkey),
    });
    if (ok)
      logger.info(
        'globalShortcut.registered',
        { hotkey },
        { source: 'host', scope: 'palette-window' },
      );
    else {
      logger.warn(
        'globalShortcut.register.failed',
        { hotkey },
        { source: 'host', scope: 'palette-window' },
      );
      showPaletteWhenReady();
    }

    win?.webContents.on('before-input-event', (_event, input) => {
      if (
        !(input.meta || input.control) ||
        !input.alt ||
        input.key.toLowerCase() !== 'i'
      )
        return;
      if (win?.webContents.isDevToolsOpened()) win.webContents.closeDevTools();
      else win?.webContents.openDevTools({ mode: 'detach' });
    });
  }

  return {
    get win() {
      return win;
    },
    createWindow,
    debugLog,
    setPaletteSizeForMode,
    centerWindow,
    applyPaletteWindowPolicy,
    showPalette,
    revealPalette,
    hidePalette,
    showPaletteWhenReady,
    togglePalette,
    registerHotkey,
  };
}
