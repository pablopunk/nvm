import assert from 'node:assert/strict'
import test from 'node:test'
import { createExtensionWindowManager, extensionWindowId, extensionWindowSize } from './extension-window-manager'

class FakeBrowserWindow {
  static instances: FakeBrowserWindow[] = []
  options: Record<string, unknown>
  title = ''
  visible = false
  destroyed = false
  bounds = { width: 0, height: 0 }
  sent: Array<{ channel: string; payload: unknown }> = []
  loadedUrl = ''
  loadedFile: { path: string; options?: unknown } | null = null
  handlers = new Map<string, (...args: any[]) => void>()
  webContents = {
    send: (channel: string, payload: unknown) => this.sent.push({ channel, payload }),
    on: (event: string, listener: (...args: any[]) => void) => this.handlers.set(`webContents:${event}`, listener),
    setWindowOpenHandler: (handler: (details: { url: string }) => { action: 'allow' | 'deny' }) => this.handlers.set('webContents:setWindowOpenHandler', handler),
  }

  constructor(options: Record<string, unknown>) {
    this.options = options
    this.title = String(options.title || '')
    this.bounds = { width: Number(options.width), height: Number(options.height) }
    FakeBrowserWindow.instances.push(this)
  }

  getBounds() { return this.bounds }
  setBounds(bounds: { x: number; y: number; width: number; height: number }) { this.bounds = { width: bounds.width, height: bounds.height } }
  setAlwaysOnTop() {}
  setVisibleOnAllWorkspaces() {}
  setTitle(title: string) { this.title = title }
  once(event: string, listener: (...args: any[]) => void) { this.handlers.set(`once:${event}`, listener) }
  on(event: string, listener: (...args: any[]) => void) { this.handlers.set(event, listener) }
  isDestroyed() { return this.destroyed }
  isVisible() { return this.visible }
  show() { this.visible = true }
  hide() { this.visible = false }
  focus() {}
  close() { this.destroyed = true; this.handlers.get('closed')?.() }
  loadURL(url: string) { this.loadedUrl = url }
  loadFile(filePath: string, options?: unknown) { this.loadedFile = { path: filePath, options } }
}

function createManager() {
  FakeBrowserWindow.instances = []
  const trustedChecks: Array<{ id: string; url: string }> = []
  const manager = createExtensionWindowManager({
    BrowserWindow: FakeBrowserWindow,
    preloadPath: '/preload.cjs',
    rendererIndexPath: '/index.html',
    rendererUrl: 'http://localhost:5173/',
    isDev: true,
    shouldUseDarkColors: () => true,
    getCursorScreenPoint: () => ({ x: 0, y: 0 }),
    getDisplayNearestPoint: () => ({ workArea: { x: 10, y: 20, width: 1000, height: 800 } }),
    normalizeView: (view) => ({ ...view, normalized: true }),
    hashValue: () => 'hash',
    installNavigationPolicy: (_win, isTrusted) => {
      trustedChecks.push({ id: 'installed', url: 'policy' })
      assert.equal(isTrusted('trusted-url'), true)
    },
    isTrustedPage: (url, id) => {
      trustedChecks.push({ id, url })
      return url === 'trusted-url'
    },
  })
  return { manager, trustedChecks }
}

test('extension window helpers clamp size and derive stable ids', () => {
  assert.deepEqual(extensionWindowSize({ width: 10, height: 10 }), { width: 320, height: 240 })
  assert.deepEqual(extensionWindowSize({ size: 'large' }), { width: 900, height: 680 })
  assert.deepEqual(extensionWindowSize({ width: 9999, height: 9999 }), { width: 1600, height: 1200 })
  assert.equal(extensionWindowId({ title: 'A' }, {}, () => 'abc'), 'window:abc')
  assert.equal(extensionWindowId({ id: 'view-id', title: 'A' }, {}, () => 'abc'), 'view-id')
  assert.equal(extensionWindowId({ id: 'view-id' }, { id: 'option-id' }, () => 'abc'), 'option-id')
})

test('creates extension windows with hardened renderer preferences and state', () => {
  const { manager, trustedChecks } = createManager()
  const record = manager.createOrUpdate({ title: 'Panel' }, { id: 'panel', titleBar: 'hidden' })
  const win = record.win as FakeBrowserWindow

  assert.equal(FakeBrowserWindow.instances.length, 1)
  assert.equal(win.loadedUrl, 'http://localhost:5173/?extensionWindowId=panel')
  assert.deepEqual(win.options.webPreferences, { preload: '/preload.cjs', contextIsolation: true, nodeIntegration: false, sandbox: false })
  assert.equal(win.options.backgroundColor, '#111111')
  assert.deepEqual(manager.getState('panel'), { id: 'panel', view: { title: 'Panel', normalized: true }, options: { id: 'panel', titleBar: 'hidden' } })
  assert.deepEqual(trustedChecks, [{ id: 'installed', url: 'policy' }, { id: 'panel', url: 'trusted-url' }])
})

test('updates existing extension windows in place and sends view payloads', () => {
  const { manager } = createManager()
  manager.createOrUpdate({ title: 'Old' }, { id: 'panel' })
  const record = manager.createOrUpdate({ title: 'New' }, { id: 'panel', title: 'Custom' })
  const win = record.win as FakeBrowserWindow

  assert.equal(FakeBrowserWindow.instances.length, 1)
  assert.equal(win.title, 'Custom')
  assert.equal(win.visible, true)
  assert.deepEqual(win.sent, [{ channel: 'extension-window:view', payload: { id: 'panel', view: { title: 'New', normalized: true }, options: { id: 'panel', title: 'Custom' } } }])
})

test('executes toggle and cleanup actions against tracked windows', () => {
  const { manager } = createManager()

  assert.deepEqual(manager.executeWindowAction({ type: 'toggleWindow', id: 'panel', view: { title: 'Panel' } }), { toast: { message: 'Opened window' } })
  const win = FakeBrowserWindow.instances[0]
  win.show()
  assert.deepEqual(manager.executeWindowAction({ type: 'toggleWindow', id: 'panel' }), { toast: { message: 'Toggled window' } })
  assert.equal(win.visible, false)
  assert.deepEqual(manager.executeWindowAction({ type: 'showWindow', id: 'panel' }), { toast: { message: 'Shown window' } })
  assert.equal(win.visible, true)
  manager.closeAll()
  assert.equal(win.destroyed, true)
  assert.equal(manager.getState('panel'), null)
})
