// biome-ignore-all lint: This legacy structural Electron boundary retains established dynamic extension payload and imperative lifecycle conventions.
import type { ExtensionWindowCapability } from './extension-window-capabilities';

type ExtensionWindowLike = {
  webContents: {
    send(channel: string, payload: unknown): void;
    on(event: string, listener: (...args: any[]) => void): void;
    setWindowOpenHandler(
      handler: (details: { url: string }) => { action: 'allow' | 'deny' },
    ): void;
  };
  getBounds(): { x?: number; y?: number; width: number; height: number };
  setBounds(bounds: {
    x: number;
    y: number;
    width: number;
    height: number;
  }): void;
  setAlwaysOnTop(flag: boolean, level?: string): void;
  setVisibleOnAllWorkspaces(
    flag: boolean,
    options?: { visibleOnFullScreen?: boolean },
  ): void;
  setTitle(title: string): void;
  once(event: string, listener: (...args: any[]) => void): void;
  on(event: string, listener: (...args: any[]) => void): void;
  isDestroyed(): boolean;
  isVisible(): boolean;
  show(): void;
  hide(): void;
  focus(): void;
  close(): void;
  loadURL(url: string): unknown;
  loadFile(filePath: string, options?: unknown): unknown;
};

type BrowserWindowConstructor = new (
  options: Record<string, unknown>,
) => ExtensionWindowLike;

type ExtensionWindowRecord = {
  id: string;
  win: ExtensionWindowLike;
  view: any;
  options: any;
  compatibility: ExtensionWindowCompatibility;
  ownerExtensionId?: string;
  restoredFrame?: boolean;
};

export type PersistedExtensionWindowFrame = {
  x: number;
  y: number;
  width: number;
  height: number;
};

export type PersistedExtensionWindow = {
  restoreKey: string;
  ownerExtensionId?: string;
  options?: Record<string, unknown>;
};

export type PersistedExtensionWindowState = {
  frames?: Record<string, PersistedExtensionWindowFrame>;
  windows?: PersistedExtensionWindow[];
};

type ExtensionWindowCompatibility = {
  persistence?: 'session-only';
  degradedCapabilities?: ExtensionWindowCapability[];
  diagnostics?: Array<
    | { reason: 'missing-restore-key' }
    | {
        reason: 'unsupported-capability';
        capability: ExtensionWindowCapability;
      }
  >;
};

type CloneSafeRecord = { [key: string]: CloneSafeValue };
type CloneSafeValue =
  | null
  | boolean
  | number
  | string
  | CloneSafeValue[]
  | CloneSafeRecord;

type ExtensionWindowManagerDeps = {
  BrowserWindow: BrowserWindowConstructor;
  preloadPath: string;
  rendererIndexPath: string;
  rendererUrl?: string;
  isDev: boolean;
  shouldUseDarkColors: () => boolean;
  getCursorScreenPoint: () => { x: number; y: number };
  getDisplayNearestPoint: (point: { x: number; y: number }) => {
    workArea: { x: number; y: number; width: number; height: number };
  };
  normalizeView: (view: any) => any;
  hashValue: (value: string) => string;
  installNavigationPolicy: (
    win: ExtensionWindowLike,
    isTrusted: (url: string) => boolean,
  ) => void;
  isTrustedPage: (url: string, id: string) => boolean;
  hasCapability: (capability: ExtensionWindowCapability) => boolean;
  persistence?: {
    read?(): PersistedExtensionWindowState;
    write(state: PersistedExtensionWindowState): void;
  };
  debug?: (message: string, data?: Record<string, unknown>) => void;
};

export const EXTENSION_WINDOW_OPTION_DEFAULTS = Object.freeze({
  titleBar: 'default',
  chrome: 'default',
  size: 'default',
  width: 560,
  height: 420,
  alwaysOnTop: true,
  visibleOnAllSpaces: false,
  hideOnBlur: false,
  persistent: false,
  remembersFrame: false,
} as const);

const WINDOW_OPTION_KEYS = new Set([
  'id',
  'restoreKey',
  'title',
  'titleBar',
  'chrome',
  'width',
  'height',
  'size',
  'alwaysOnTop',
  'visibleOnAllSpaces',
  'hideOnBlur',
  'persistent',
  'remembersFrame',
]);
const STABLE_KEY = /^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/;

function invalidWindowInput(message: string): never {
  throw new Error(`Invalid extension window input: ${message}`);
}

function finiteDimension(value: unknown, fallback: number, name: string) {
  if (value === undefined) return fallback;
  if (typeof value !== 'number' || !Number.isFinite(value))
    invalidWindowInput(`${name} must be a finite number`);
  return Math.round(value);
}

/** Validate only the bounded window-options structure, never ExtensionView itself. */
export function normalizeExtensionWindowOptions(value: unknown) {
  if (!value || typeof value !== 'object' || Array.isArray(value))
    invalidWindowInput('options must be an object');
  const options = value as Record<string, unknown>;
  for (const key of Object.keys(options)) {
    if (!WINDOW_OPTION_KEYS.has(key))
      invalidWindowInput(`unknown option ${key}`);
  }
  for (const key of ['id', 'restoreKey'] as const) {
    if (options[key] !== undefined) {
      if (typeof options[key] !== 'string' || !STABLE_KEY.test(options[key]))
        invalidWindowInput(`${key} must be a bounded stable key`);
    }
  }
  if (options.title !== undefined && typeof options.title !== 'string')
    invalidWindowInput('title must be a string');
  if (
    options.titleBar !== undefined &&
    options.titleBar !== 'default' &&
    options.titleBar !== 'hidden'
  )
    invalidWindowInput('titleBar must be default or hidden');
  if (
    options.chrome !== undefined &&
    options.chrome !== 'default' &&
    options.chrome !== 'none'
  )
    invalidWindowInput('chrome must be default or none');
  if (
    options.size !== undefined &&
    options.size !== 'default' &&
    options.size !== 'large'
  )
    invalidWindowInput('size must be default or large');
  for (const key of [
    'alwaysOnTop',
    'visibleOnAllSpaces',
    'hideOnBlur',
    'persistent',
    'remembersFrame',
  ]) {
    if (options[key] !== undefined && typeof options[key] !== 'boolean')
      invalidWindowInput(`${key} must be a boolean`);
  }
  finiteDimension(
    options.width,
    EXTENSION_WINDOW_OPTION_DEFAULTS.width,
    'width',
  );
  finiteDimension(
    options.height,
    EXTENSION_WINDOW_OPTION_DEFAULTS.height,
    'height',
  );
  return { ...options };
}

export function extensionWindowSize(options: any = {}) {
  const large = options.size === 'large';
  return {
    width: Math.max(
      320,
      Math.min(
        1600,
        finiteDimension(options.width, large ? 900 : 560, 'width'),
      ),
    ),
    height: Math.max(
      240,
      Math.min(
        1200,
        finiteDimension(options.height, large ? 680 : 420, 'height'),
      ),
    ),
  };
}

export function extensionWindowId(
  view: any,
  options: any = {},
  hash = defaultHashValue,
) {
  return String(
    options.id ||
      view?.id ||
      `window:${hash(`${view?.title || 'Extension Window'}:${JSON.stringify(view || {})}`)}`,
  );
}

function defaultHashValue(value: string) {
  let hash = 0;
  for (let index = 0; index < value.length; index += 1)
    hash = (Math.imul(31, hash) + value.charCodeAt(index)) | 0;
  return String(Math.abs(hash));
}

function cloneSafeWindowOptions(value: unknown): CloneSafeValue | undefined {
  if (value === null) return null;
  if (
    typeof value === 'boolean' ||
    typeof value === 'number' ||
    typeof value === 'string'
  )
    return value;
  if (Array.isArray(value)) {
    const result: CloneSafeValue[] = [];
    for (const item of value) {
      const safeItem = cloneSafeWindowOptions(item);
      if (safeItem !== undefined) result.push(safeItem);
    }
    return result;
  }
  if (!value || typeof value !== 'object') return undefined;
  const result: CloneSafeRecord = {};
  for (const [key, item] of Object.entries(value)) {
    const safeItem = cloneSafeWindowOptions(item);
    if (safeItem !== undefined) result[key] = safeItem;
  }
  return result;
}

function cloneSafeWindowOptionRecord(value: unknown): CloneSafeRecord {
  const safeValue = cloneSafeWindowOptions(value);
  return safeValue && typeof safeValue === 'object' && !Array.isArray(safeValue)
    ? safeValue
    : {};
}

function extensionWindowViewPayload(id: string, view: any, options: any) {
  const payload = { id, view, options: cloneSafeWindowOptionRecord(options) };
  structuredClone(payload);
  return payload;
}

export function createExtensionWindowManager(deps: ExtensionWindowManagerDeps) {
  const records = new Map<string, ExtensionWindowRecord>();
  let persistedState: PersistedExtensionWindowState =
    deps.persistence?.read?.() || {};
  let persistedStateHydrated = Boolean(deps.persistence?.read);
  let quitting = false;

  /**
   * Async startup hydration path. The manager is constructed before persisted
   * state is readable without blocking I/O, so the host hydrates it once.
   */
  function hydratePersistedState(state: PersistedExtensionWindowState) {
    if (persistedStateHydrated) return;
    persistedStateHydrated = true;
    persistedState = state && typeof state === 'object' ? state : {};
  }

  function writePersistedState() {
    deps.persistence?.write(persistedState);
  }

  function frameKeyFor(id: string, options: any) {
    return String(options?.restoreKey || id);
  }

  function savedFrameFor(id: string, options: any) {
    const frame = persistedState.frames?.[frameKeyFor(id, options)];
    if (
      !frame ||
      !Number.isFinite(frame.x) ||
      !Number.isFinite(frame.y) ||
      !Number.isFinite(frame.width) ||
      !Number.isFinite(frame.height)
    )
      return null;
    return frame;
  }

  function clampedFrame(frame: PersistedExtensionWindowFrame) {
    const size = extensionWindowSize(frame);
    if (!deps.hasCapability('windows.display-recovery'))
      return { x: Math.round(frame.x), y: Math.round(frame.y), ...size };
    const centerPoint = {
      x: frame.x + frame.width / 2,
      y: frame.y + frame.height / 2,
    };
    const { workArea } = deps.getDisplayNearestPoint(centerPoint);
    return {
      x: Math.min(
        Math.max(Math.round(frame.x), workArea.x),
        Math.max(workArea.x, workArea.x + workArea.width - size.width),
      ),
      y: Math.min(
        Math.max(Math.round(frame.y), workArea.y),
        Math.max(workArea.y, workArea.y + workArea.height - size.height),
      ),
      ...size,
    };
  }

  function rememberFrame(id: string, options: any, bounds: any) {
    if (!deps.persistence || !options?.remembersFrame) return;
    if (
      !bounds ||
      !Number.isFinite(bounds.width) ||
      !Number.isFinite(bounds.height)
    )
      return;
    persistedState = {
      ...persistedState,
      frames: {
        ...(persistedState.frames || {}),
        [frameKeyFor(id, options)]: {
          x: Math.round(Number(bounds.x) || 0),
          y: Math.round(Number(bounds.y) || 0),
          width: Math.round(bounds.width),
          height: Math.round(bounds.height),
        },
      },
    };
    writePersistedState();
  }

  function persistWindowRecord(record: ExtensionWindowRecord) {
    if (!deps.persistence) return;
    const options = record.options || {};
    if (
      !options.persistent ||
      !options.restoreKey ||
      record.compatibility.persistence === 'session-only'
    )
      return;
    const restoreKey = String(options.restoreKey);
    const entry: PersistedExtensionWindow = {
      restoreKey,
      ...(record.ownerExtensionId
        ? { ownerExtensionId: record.ownerExtensionId }
        : {}),
      options: cloneSafeWindowOptionRecord(options),
    };
    const windows = (persistedState.windows || []).filter(
      (saved) => saved.restoreKey !== restoreKey,
    );
    persistedState = { ...persistedState, windows: [...windows, entry] };
    writePersistedState();
  }

  function forgetPersistentWindow(restoreKey: string) {
    if (!deps.persistence) return;
    const windows = (persistedState.windows || []).filter(
      (saved) => saved.restoreKey !== restoreKey,
    );
    if (windows.length === (persistedState.windows || []).length) return;
    persistedState = { ...persistedState, windows };
    writePersistedState();
  }

  function persistentWindowRecords() {
    return [...(persistedState.windows || [])].map((entry) => ({
      ...entry,
      options: entry.options ? { ...entry.options } : undefined,
    }));
  }

  function compatibilityForOptions(
    id: string,
    options: Record<string, unknown>,
  ): ExtensionWindowCompatibility {
    const requestedCapabilities: ExtensionWindowCapability[] = [];
    if (options.alwaysOnTop !== false)
      requestedCapabilities.push('windows.always-on-top');
    if (options.visibleOnAllSpaces)
      requestedCapabilities.push('windows.all-spaces');
    if (options.remembersFrame || (options.persistent && options.restoreKey)) {
      requestedCapabilities.push(
        'windows.frame-restore',
        'windows.display-recovery',
      );
    }
    const degradedCapabilities = requestedCapabilities.filter(
      (capability) => !deps.hasCapability(capability),
    );
    const diagnostics: ExtensionWindowCompatibility['diagnostics'] =
      degradedCapabilities.map((capability) => ({
        reason: 'unsupported-capability',
        capability,
      }));
    const missingRestoreKey = Boolean(
      options.persistent && !options.restoreKey,
    );
    if (missingRestoreKey)
      diagnostics.unshift({ reason: 'missing-restore-key' });
    for (const diagnostic of diagnostics) {
      if (diagnostic.reason === 'missing-restore-key')
        deps.debug?.('extensionWindow.persistenceDegraded', {
          id,
          persistence: 'session-only',
          reason: diagnostic.reason,
        });
      else
        deps.debug?.('extensionWindow.capabilityDegraded', {
          id,
          capability: diagnostic.capability,
          reason: diagnostic.reason,
        });
    }
    return {
      ...(missingRestoreKey ? { persistence: 'session-only' as const } : {}),
      ...(degradedCapabilities.length > 0 ? { degradedCapabilities } : {}),
      ...(diagnostics.length > 0 ? { diagnostics } : {}),
    };
  }

  function load(win: ExtensionWindowLike, id: string) {
    if (deps.isDev && deps.rendererUrl)
      return win.loadURL(
        `${deps.rendererUrl}?extensionWindowId=${encodeURIComponent(id)}`,
      );
    return win.loadFile(deps.rendererIndexPath, {
      query: { extensionWindowId: id },
    });
  }

  function center(win: ExtensionWindowLike) {
    const cursor = deps.getCursorScreenPoint();
    const display = deps.getDisplayNearestPoint(cursor);
    const bounds = win.getBounds();
    const { x, y, width, height } = display.workArea;
    win.setBounds({
      x: Math.round(x + (width - bounds.width) / 2),
      y: Math.round(y + (height - bounds.height) / 2),
      width: bounds.width,
      height: bounds.height,
    });
  }

  function applyOptions(win: ExtensionWindowLike, options: any = {}) {
    const size = extensionWindowSize(options);
    const bounds = win.getBounds();
    if (bounds.width !== size.width || bounds.height !== size.height) {
      win.setBounds({
        x: bounds.x || 0,
        y: bounds.y || 0,
        width: size.width,
        height: size.height,
      });
    }
    const alwaysOnTop = options.alwaysOnTop !== false;
    if (deps.hasCapability('windows.always-on-top'))
      win.setAlwaysOnTop(alwaysOnTop, alwaysOnTop ? 'floating' : 'normal');
    if (deps.hasCapability('windows.all-spaces'))
      win.setVisibleOnAllWorkspaces(Boolean(options.visibleOnAllSpaces), {
        visibleOnFullScreen: true,
      });
  }

  function createOrUpdate(
    view: any,
    options: any = {},
    visibility: 'show' | 'preserve' = 'show',
    ownerExtensionId?: string,
  ) {
    const normalizedView = deps.normalizeView(view);
    structuredClone(normalizedView);
    const safeOptions = normalizeExtensionWindowOptions(options);
    const id = extensionWindowId(normalizedView, safeOptions, deps.hashValue);
    const existing = records.get(id);
    if (existing && !existing.win.isDestroyed()) {
      existing.view = normalizedView;
      existing.options = { ...existing.options, ...safeOptions, id };
      existing.ownerExtensionId = ownerExtensionId || existing.ownerExtensionId;
      existing.compatibility = compatibilityForOptions(id, existing.options);
      applyOptions(existing.win, existing.options);
      existing.win.setTitle(
        String(existing.options.title || normalizedView.title || 'Nevermind'),
      );
      existing.win.webContents.send(
        'extension-window:view',
        extensionWindowViewPayload(id, normalizedView, existing.options),
      );
      if (visibility === 'show') {
        existing.win.show();
        existing.win.focus();
      }
      persistWindowRecord(existing);
      return existing;
    }

    const canRestoreFrame =
      Boolean(safeOptions.remembersFrame) &&
      deps.hasCapability('windows.frame-restore');
    const savedFrame = canRestoreFrame ? savedFrameFor(id, safeOptions) : null;
    const restoredFrame = savedFrame ? clampedFrame(savedFrame) : null;
    const size = restoredFrame || extensionWindowSize(safeOptions);
    const hiddenTitleBar = safeOptions.titleBar === 'hidden';
    const win = new deps.BrowserWindow({
      width: size.width,
      height: size.height,
      minWidth: 320,
      minHeight: 240,
      show: false,
      frame: true,
      ...(hiddenTitleBar
        ? {
            titleBarStyle: 'hiddenInset',
            trafficLightPosition: { x: 12, y: 12 },
          }
        : {}),
      title: String(
        (safeOptions as any).title || normalizedView.title || 'Nevermind',
      ),
      backgroundColor: deps.shouldUseDarkColors() ? '#111111' : '#f7f7f7',
      webPreferences: {
        preload: deps.preloadPath,
        contextIsolation: true,
        nodeIntegration: false,
        sandbox: true,
      },
    });
    const record: ExtensionWindowRecord = {
      id,
      win,
      view: normalizedView,
      options: { ...safeOptions, id },
      compatibility: compatibilityForOptions(id, safeOptions),
      ownerExtensionId,
      restoredFrame: Boolean(restoredFrame),
    };
    records.set(id, record);
    structuredClone(
      extensionWindowViewPayload(id, normalizedView, record.options),
    );
    applyOptions(win, record.options);
    if (restoredFrame) win.setBounds(restoredFrame);
    win.once('ready-to-show', () => {
      if (!record.restoredFrame) center(win);
      win.show();
    });
    if ((safeOptions as any).hideOnBlur) win.on('blur', () => win.hide());
    if (safeOptions.remembersFrame) {
      const trackFrame = () =>
        rememberFrame(id, record.options, win.getBounds());
      win.on('resize', trackFrame);
      win.on('move', trackFrame);
    }
    win.on('closed', () => {
      if (records.get(id)?.win === win) records.delete(id);
      if (!quitting && record.options?.restoreKey)
        forgetPersistentWindow(String(record.options.restoreKey));
    });
    deps.installNavigationPolicy(win, (url) => deps.isTrustedPage(url, id));
    win.webContents.on(
      'did-fail-load',
      (_event, errorCode, errorDescription, validatedURL) =>
        deps.debug?.('extensionWindow.didFailLoad', {
          id,
          errorCode,
          errorDescription,
          validatedURL,
        }),
    );
    load(win, id);
    persistWindowRecord(record);
    return record;
  }

  function executeWindowAction(action: any) {
    const id = String(
      action.windowId ||
        action.id ||
        (action.type === 'toggleWindow' && action.view
          ? extensionWindowId(
              action.view,
              action.windowOptions || {},
              deps.hashValue,
            )
          : ''),
    );
    if (action.type === 'createWindow') {
      const record = createOrUpdate(
        action.view,
        action.windowOptions || {},
        'show',
        action.ownerExtensionId,
      );
      return {
        toast: { message: 'Opened window' },
        ...record.compatibility,
      };
    }
    const record = records.get(id);
    if (!record) {
      if (action.type === 'toggleWindow' && action.view) {
        const created = createOrUpdate(
          action.view,
          {
            ...(action.windowOptions || {}),
            id,
          },
          'show',
          action.ownerExtensionId,
        );
        return {
          toast: { message: 'Opened window' },
          ...created.compatibility,
        };
      }
      return { toast: { message: 'Window is not open', tone: 'error' } };
    }
    if (action.type === 'showWindow') {
      record.win.show();
      record.win.focus();
      return {
        toast: { message: 'Shown window' },
        ...record.compatibility,
      };
    }
    if (action.type === 'hideWindow') {
      record.win.hide();
      return {
        toast: { message: 'Hidden window' },
        ...record.compatibility,
      };
    }
    if (action.type === 'toggleWindow') {
      if (action.view || action.windowOptions)
        createOrUpdate(
          action.view || record.view,
          {
            ...(record.options || {}),
            ...(action.windowOptions || {}),
            id,
          },
          'preserve',
          action.ownerExtensionId,
        );
      if (record.win.isVisible()) record.win.hide();
      else {
        record.win.show();
        record.win.focus();
      }
      return {
        toast: { message: 'Toggled window' },
        ...record.compatibility,
      };
    }
    if (action.type === 'closeWindow') {
      record.win.close();
      return {
        toast: { message: 'Closed window' },
        ...record.compatibility,
      };
    }
    return null;
  }

  function getState(id: string) {
    const record = records.get(String(id || ''));
    return record
      ? { id: record.id, view: record.view, options: record.options }
      : null;
  }

  /**
   * Independent windows never authorize state reads by a renderer-supplied id.
   * Electron gives us the exact sender object, which must belong to one live
   * record. Palette and sibling renderers therefore cannot inspect each other.
   */
  function getStateForSender(sender: unknown) {
    for (const record of records.values()) {
      if (!record.win.isDestroyed() && record.win.webContents === sender) {
        return { id: record.id, view: record.view, options: record.options };
      }
    }
    return null;
  }

  function closeForSender(sender: unknown) {
    for (const record of records.values()) {
      if (!record.win.isDestroyed() && record.win.webContents === sender) {
        record.win.close();
        return true;
      }
    }
    return false;
  }

  function closeAll() {
    quitting = true;
    for (const record of records.values()) record.win.close();
    records.clear();
  }

  return {
    records,
    createOrUpdate,
    executeWindowAction,
    getState,
    getStateForSender,
    closeForSender,
    closeAll,
    persistentWindowRecords,
    forgetPersistentWindow,
    hydratePersistedState,
  };
}
