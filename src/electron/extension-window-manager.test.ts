import assert from 'node:assert/strict';
import test from 'node:test';
import {
  createExtensionWindowManager,
  EXTENSION_WINDOW_OPTION_DEFAULTS,
  extensionWindowId,
  extensionWindowSize,
} from './extension-window-manager';

class FakeBrowserWindow {
  static instances: FakeBrowserWindow[] = [];
  options: Record<string, unknown>;
  title = '';
  visible = false;
  destroyed = false;
  bounds = { x: 0, y: 0, width: 0, height: 0 };
  alwaysOnTop: Array<{ flag: boolean; level?: string }> = [];
  visibleOnAllWorkspaces: Array<{
    flag: boolean;
    options?: { visibleOnFullScreen?: boolean };
  }> = [];
  sent: Array<{ channel: string; payload: unknown }> = [];
  loadedUrl = '';
  loadedFile: { path: string; options?: unknown } | null = null;
  handlers = new Map<string, (...args: any[]) => void>();
  showCount = 0;
  hideCount = 0;
  focusCount = 0;
  webContents = {
    send: (channel: string, payload: unknown) =>
      this.sent.push({ channel, payload }),
    on: (event: string, listener: (...args: any[]) => void) =>
      this.handlers.set(`webContents:${event}`, listener),
    setWindowOpenHandler: (
      handler: (details: { url: string }) => { action: 'allow' | 'deny' },
    ) => this.handlers.set('webContents:setWindowOpenHandler', handler),
  };

  constructor(options: Record<string, unknown>) {
    this.options = options;
    this.title = String(options.title || '');
    this.bounds = {
      x: 0,
      y: 0,
      width: Number(options.width),
      height: Number(options.height),
    };
    FakeBrowserWindow.instances.push(this);
  }

  getBounds() {
    return this.bounds;
  }
  setBounds(bounds: { x: number; y: number; width: number; height: number }) {
    this.bounds = bounds;
  }
  setAlwaysOnTop(flag: boolean, level?: string) {
    this.alwaysOnTop.push({ flag, level });
  }
  setVisibleOnAllWorkspaces(
    flag: boolean,
    options?: { visibleOnFullScreen?: boolean },
  ) {
    this.visibleOnAllWorkspaces.push({ flag, options });
  }
  setTitle(title: string) {
    this.title = title;
  }
  once(event: string, listener: (...args: any[]) => void) {
    this.handlers.set(`once:${event}`, listener);
  }
  on(event: string, listener: (...args: any[]) => void) {
    this.handlers.set(event, listener);
  }
  isDestroyed() {
    return this.destroyed;
  }
  isVisible() {
    return this.visible;
  }
  show() {
    this.showCount += 1;
    this.visible = true;
  }
  hide() {
    this.hideCount += 1;
    this.visible = false;
  }
  focus() {
    this.focusCount += 1;
  }
  close() {
    this.destroyed = true;
    this.handlers.get('closed')?.();
  }
  loadURL(url: string) {
    this.loadedUrl = url;
  }
  loadFile(filePath: string, options?: unknown) {
    this.loadedFile = { path: filePath, options };
  }
}

function createManager(unsupportedCapabilities: string[] = []) {
  FakeBrowserWindow.instances = [];
  const trustedChecks: Array<{ id: string; url: string }> = [];
  const diagnostics: Array<{
    message: string;
    data?: Record<string, unknown>;
  }> = [];
  const manager = createExtensionWindowManager({
    BrowserWindow: FakeBrowserWindow,
    preloadPath: '/preload.cjs',
    rendererIndexPath: '/index.html',
    rendererUrl: 'http://localhost:5173/',
    isDev: true,
    shouldUseDarkColors: () => true,
    getCursorScreenPoint: () => ({ x: 0, y: 0 }),
    getDisplayNearestPoint: () => ({
      workArea: { x: 10, y: 20, width: 1000, height: 800 },
    }),
    normalizeView: (view) => ({ ...view, normalized: true }),
    hashValue: () => 'hash',
    hasCapability: (capability) =>
      !unsupportedCapabilities.includes(capability),
    installNavigationPolicy: (_win, isTrusted) => {
      trustedChecks.push({ id: 'installed', url: 'policy' });
      assert.equal(isTrusted('trusted-url'), true);
    },
    isTrustedPage: (url, id) => {
      trustedChecks.push({ id, url });
      return url === 'trusted-url';
    },
    debug: (message, data) => diagnostics.push({ message, data }),
  });
  return { manager, trustedChecks, diagnostics };
}

test('extension window helpers clamp size and derive stable ids', () => {
  assert.deepEqual(EXTENSION_WINDOW_OPTION_DEFAULTS, {
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
  });
  assert.deepEqual(extensionWindowSize(), { width: 560, height: 420 });
  assert.deepEqual(extensionWindowSize({ width: 10, height: 10 }), {
    width: 320,
    height: 240,
  });
  assert.deepEqual(extensionWindowSize({ size: 'large' }), {
    width: 900,
    height: 680,
  });
  assert.deepEqual(extensionWindowSize({ width: 9999, height: 9999 }), {
    width: 1600,
    height: 1200,
  });
  assert.equal(
    extensionWindowId({ title: 'A' }, {}, () => 'abc'),
    'window:abc',
  );
  assert.equal(
    extensionWindowId({ id: 'view-id', title: 'A' }, {}, () => 'abc'),
    'view-id',
  );
  assert.equal(
    extensionWindowId({ id: 'view-id' }, { id: 'option-id' }, () => 'abc'),
    'option-id',
  );
});

test('creates extension windows with hardened renderer preferences and state', () => {
  const { manager, trustedChecks } = createManager();
  const record = manager.createOrUpdate(
    { title: 'Panel' },
    { id: 'panel', titleBar: 'hidden' },
  );
  const win = record.win as FakeBrowserWindow;

  assert.equal(FakeBrowserWindow.instances.length, 1);
  assert.equal(win.loadedUrl, 'http://localhost:5173/?extensionWindowId=panel');
  assert.deepEqual(win.options.webPreferences, {
    preload: '/preload.cjs',
    contextIsolation: true,
    nodeIntegration: false,
    sandbox: true,
  });
  assert.equal(win.options.backgroundColor, '#111111');
  assert.deepEqual(manager.getState('panel'), {
    id: 'panel',
    view: { title: 'Panel', normalized: true },
    options: { id: 'panel', titleBar: 'hidden' },
  });
  assert.deepEqual(trustedChecks, [
    { id: 'installed', url: 'policy' },
    { id: 'panel', url: 'trusted-url' },
  ]);
  assert.deepEqual(win.alwaysOnTop, [{ flag: true, level: 'floating' }]);
  assert.deepEqual(win.visibleOnAllWorkspaces, [
    { flag: false, options: { visibleOnFullScreen: true } },
  ]);
  assert.equal(win.handlers.has('blur'), false);
  win.handlers.get('once:ready-to-show')?.();
  assert.equal(win.visible, true);
  assert.deepEqual(win.bounds, { x: 230, y: 210, width: 560, height: 420 });
});

test('updates existing extension windows in place and sends clone-safe view payloads', () => {
  const { manager } = createManager();
  manager.createOrUpdate(
    { title: 'Old' },
    {
      id: 'panel',
      width: 400,
      height: 300,
      visibleOnAllSpaces: true,
      onClick: () => 'ignored',
    },
  );
  const record = manager.createOrUpdate(
    { title: 'New' },
    {
      id: 'panel',
      title: 'Custom',
      width: 640,
      height: 360,
      alwaysOnTop: false,
      visibleOnAllSpaces: false,
      onClick: () => 'ignored',
    },
  );
  const win = record.win as FakeBrowserWindow;

  assert.equal(FakeBrowserWindow.instances.length, 1);
  assert.equal(win.title, 'Custom');
  assert.equal(win.visible, true);
  assert.deepEqual(win.bounds, { x: 0, y: 0, width: 640, height: 360 });
  assert.deepEqual(win.alwaysOnTop.at(-1), { flag: false, level: 'normal' });
  assert.deepEqual(win.visibleOnAllWorkspaces.at(-1), {
    flag: false,
    options: { visibleOnFullScreen: true },
  });
  assert.deepEqual(win.sent, [
    {
      channel: 'extension-window:view',
      payload: {
        id: 'panel',
        view: { title: 'New', normalized: true },
        options: {
          id: 'panel',
          width: 640,
          height: 360,
          visibleOnAllSpaces: false,
          title: 'Custom',
          alwaysOnTop: false,
        },
      },
    },
  ]);
  assert.doesNotThrow(() => structuredClone(win.sent[0].payload));
});

test('executes toggle and cleanup actions against tracked windows', () => {
  const { manager } = createManager();

  assert.deepEqual(
    manager.executeWindowAction({
      type: 'toggleWindow',
      id: 'panel',
      view: { title: 'Panel' },
    }),
    { toast: { message: 'Opened window' } },
  );
  const win = FakeBrowserWindow.instances[0];
  win.show();
  assert.deepEqual(
    manager.executeWindowAction({ type: 'toggleWindow', id: 'panel' }),
    { toast: { message: 'Toggled window' } },
  );
  assert.equal(win.visible, false);
  assert.deepEqual(
    manager.executeWindowAction({ type: 'showWindow', id: 'panel' }),
    { toast: { message: 'Shown window' } },
  );
  assert.equal(win.visible, true);
  manager.closeAll();
  assert.equal(win.destroyed, true);
  assert.equal(manager.getState('panel'), null);
});

test('shows, focuses, hides, and closes existing windows', () => {
  const { manager } = createManager();
  const record = manager.createOrUpdate(
    { id: 'panel', title: 'Panel' },
    { hideOnBlur: true },
  );
  const win = record.win as FakeBrowserWindow;

  assert.equal(win.handlers.has('blur'), true);
  assert.deepEqual(
    manager.executeWindowAction({ type: 'showWindow', id: 'panel' }),
    { toast: { message: 'Shown window' } },
  );
  assert.equal(win.visible, true);
  assert.equal(win.focusCount, 1);
  assert.deepEqual(
    manager.executeWindowAction({ type: 'hideWindow', id: 'panel' }),
    { toast: { message: 'Hidden window' } },
  );
  assert.equal(win.visible, false);
  assert.deepEqual(
    manager.executeWindowAction({ type: 'closeWindow', id: 'panel' }),
    { toast: { message: 'Closed window' } },
  );
  assert.equal(win.destroyed, true);
  assert.equal(manager.getState('panel'), null);
});

test('returns the frozen missing-window error for every id-only control action', () => {
  const { manager } = createManager();
  for (const type of [
    'showWindow',
    'hideWindow',
    'toggleWindow',
    'closeWindow',
  ])
    assert.deepEqual(manager.executeWindowAction({ type, id: 'missing' }), {
      toast: { message: 'Window is not open', tone: 'error' },
    });
});

test('toggles an existing window exactly once while updating its view and options', () => {
  const { manager } = createManager();
  const record = manager.createOrUpdate(
    { id: 'panel', title: 'Old' },
    { width: 400 },
  );
  const win = record.win as FakeBrowserWindow;
  win.handlers.get('once:ready-to-show')?.();
  assert.equal(win.visible, true);

  const showsBeforeHideToggle = win.showCount;
  assert.deepEqual(
    manager.executeWindowAction({
      type: 'toggleWindow',
      view: { id: 'panel', title: 'New' },
      windowOptions: { width: 640 },
    }),
    { toast: { message: 'Toggled window' } },
  );
  assert.equal(win.visible, false);
  assert.equal(win.showCount, showsBeforeHideToggle);
  assert.equal(win.hideCount, 1);

  const hidesBeforeShowToggle = win.hideCount;
  manager.executeWindowAction({
    type: 'toggleWindow',
    view: { id: 'panel', title: 'Newest' },
    windowOptions: { height: 480 },
  });
  assert.equal(win.visible, true);
  assert.equal(win.hideCount, hidesBeforeShowToggle);
  assert.equal(win.showCount, showsBeforeHideToggle + 1);
  assert.equal(win.focusCount, 1);
  assert.equal(win.title, 'Newest');
  assert.deepEqual(win.bounds, { x: 310, y: 210, width: 640, height: 480 });
});

test('keeps persistent windows without restore keys live and reports session-only persistence', () => {
  const { manager, diagnostics } = createManager();
  const result = manager.executeWindowAction({
    type: 'createWindow',
    view: { id: 'panel', title: 'Panel' },
    windowOptions: { persistent: true },
  });

  assert.deepEqual(result, {
    toast: { message: 'Opened window' },
    persistence: 'session-only',
    diagnostics: [{ reason: 'missing-restore-key' }],
  });
  assert.notEqual(manager.getState('panel'), null);
  assert.deepEqual(
    manager.executeWindowAction({ type: 'hideWindow', id: 'panel' }),
    {
      toast: { message: 'Hidden window' },
      persistence: 'session-only',
      diagnostics: [{ reason: 'missing-restore-key' }],
    },
  );
  assert.deepEqual(
    manager.executeWindowAction({ type: 'showWindow', id: 'panel' }),
    {
      toast: { message: 'Shown window' },
      persistence: 'session-only',
      diagnostics: [{ reason: 'missing-restore-key' }],
    },
  );
  assert.deepEqual(
    manager.executeWindowAction({ type: 'toggleWindow', id: 'panel' }),
    {
      toast: { message: 'Toggled window' },
      persistence: 'session-only',
      diagnostics: [{ reason: 'missing-restore-key' }],
    },
  );
  assert.deepEqual(diagnostics, [
    {
      message: 'extensionWindow.persistenceDegraded',
      data: {
        id: 'panel',
        persistence: 'session-only',
        reason: 'missing-restore-key',
      },
    },
  ]);
  assert.deepEqual(
    manager.executeWindowAction({ type: 'closeWindow', id: 'panel' }),
    {
      toast: { message: 'Closed window' },
      persistence: 'session-only',
      diagnostics: [{ reason: 'missing-restore-key' }],
    },
  );
  assert.equal(manager.getState('panel'), null);
});

test('opens in degraded mode and skips unsupported native window methods', () => {
  const unsupported = [
    'windows.always-on-top',
    'windows.all-spaces',
    'windows.frame-restore',
    'windows.display-recovery',
  ];
  const { manager, diagnostics } = createManager(unsupported);
  const result = manager.executeWindowAction({
    type: 'createWindow',
    view: { id: 'panel', title: 'Panel' },
    windowOptions: {
      visibleOnAllSpaces: true,
      remembersFrame: true,
    },
  });
  const win = FakeBrowserWindow.instances[0];

  assert.deepEqual(win.alwaysOnTop, []);
  assert.deepEqual(win.visibleOnAllWorkspaces, []);
  assert.deepEqual(result, {
    toast: { message: 'Opened window' },
    degradedCapabilities: unsupported,
    diagnostics: unsupported.map((capability) => ({
      reason: 'unsupported-capability',
      capability,
    })),
  });
  assert.deepEqual(
    diagnostics.map(({ data }) => data?.capability),
    unsupported,
  );
});
