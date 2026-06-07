type ExtensionWindowLike = {
  webContents: {
    send(channel: string, payload: unknown): void
    on(event: string, listener: (...args: any[]) => void): void
    setWindowOpenHandler(handler: (details: { url: string }) => { action: 'allow' | 'deny' }): void
  }
  getBounds(): { width: number; height: number }
  setBounds(bounds: { x: number; y: number; width: number; height: number }): void
  setAlwaysOnTop(flag: boolean, level?: string): void
  setVisibleOnAllWorkspaces(flag: boolean, options?: { visibleOnFullScreen?: boolean }): void
  setTitle(title: string): void
  once(event: string, listener: (...args: any[]) => void): void
  on(event: string, listener: (...args: any[]) => void): void
  isDestroyed(): boolean
  isVisible(): boolean
  show(): void
  hide(): void
  focus(): void
  close(): void
  loadURL(url: string): unknown
  loadFile(filePath: string, options?: unknown): unknown
}

type BrowserWindowConstructor = new (options: Record<string, unknown>) => ExtensionWindowLike

type ExtensionWindowRecord = { id: string; win: ExtensionWindowLike; view: any; options: any }

type ExtensionWindowManagerDeps = {
  BrowserWindow: BrowserWindowConstructor
  preloadPath: string
  rendererIndexPath: string
  rendererUrl?: string
  isDev: boolean
  shouldUseDarkColors: () => boolean
  getCursorScreenPoint: () => { x: number; y: number }
  getDisplayNearestPoint: (point: { x: number; y: number }) => { workArea: { x: number; y: number; width: number; height: number } }
  normalizeView: (view: any) => any
  hashValue: (value: string) => string
  installNavigationPolicy: (win: ExtensionWindowLike, isTrusted: (url: string) => boolean) => void
  isTrustedPage: (url: string, id: string) => boolean
  debug?: (message: string, data?: Record<string, unknown>) => void
}

export function extensionWindowSize(options: any = {}) {
  const large = options.size === 'large'
  return {
    width: Math.max(320, Math.min(1600, Number(options.width || (large ? 900 : 560)))),
    height: Math.max(240, Math.min(1200, Number(options.height || (large ? 680 : 420)))),
  }
}

export function extensionWindowId(view: any, options: any = {}, hash = defaultHashValue) {
  return String(options.id || view?.id || `window:${hash(`${view?.title || 'Extension Window'}:${JSON.stringify(view || {})}`)}`)
}

function defaultHashValue(value: string) {
  let hash = 0
  for (let index = 0; index < value.length; index += 1) hash = Math.imul(31, hash) + value.charCodeAt(index) | 0
  return String(Math.abs(hash))
}

export function createExtensionWindowManager(deps: ExtensionWindowManagerDeps) {
  const records = new Map<string, ExtensionWindowRecord>()

  function load(win: ExtensionWindowLike, id: string) {
    if (deps.isDev && deps.rendererUrl) return win.loadURL(`${deps.rendererUrl}?extensionWindowId=${encodeURIComponent(id)}`)
    return win.loadFile(deps.rendererIndexPath, { query: { extensionWindowId: id } })
  }

  function center(win: ExtensionWindowLike) {
    const cursor = deps.getCursorScreenPoint()
    const display = deps.getDisplayNearestPoint(cursor)
    const bounds = win.getBounds()
    const { x, y, width, height } = display.workArea
    win.setBounds({ x: Math.round(x + (width - bounds.width) / 2), y: Math.round(y + (height - bounds.height) / 2), width: bounds.width, height: bounds.height })
  }

  function applyOptions(win: ExtensionWindowLike, options: any = {}) {
    const alwaysOnTop = options.alwaysOnTop !== false
    win.setAlwaysOnTop(alwaysOnTop, alwaysOnTop ? 'floating' : 'normal')
    if (options.visibleOnAllSpaces) win.setVisibleOnAllWorkspaces(true, { visibleOnFullScreen: true })
  }

  function createOrUpdate(view: any, options: any = {}) {
    const normalizedView = deps.normalizeView(view)
    structuredClone(normalizedView)
    const id = extensionWindowId(normalizedView, options, deps.hashValue)
    const existing = records.get(id)
    if (existing && !existing.win.isDestroyed()) {
      existing.view = normalizedView
      existing.options = { ...existing.options, ...options, id }
      existing.win.setTitle(String(options.title || normalizedView.title || 'Nevermind'))
      existing.win.webContents.send('extension-window:view', { id, view: normalizedView, options: existing.options })
      existing.win.show()
      existing.win.focus()
      return existing
    }

    const size = extensionWindowSize(options)
    const hiddenTitleBar = options.titleBar === 'hidden'
    const win = new deps.BrowserWindow({
      width: size.width,
      height: size.height,
      minWidth: 320,
      minHeight: 240,
      show: false,
      frame: true,
      ...(hiddenTitleBar ? { titleBarStyle: 'hiddenInset', trafficLightPosition: { x: 12, y: 12 } } : {}),
      title: String(options.title || normalizedView.title || 'Nevermind'),
      backgroundColor: deps.shouldUseDarkColors() ? '#111111' : '#f7f7f7',
      webPreferences: { preload: deps.preloadPath, contextIsolation: true, nodeIntegration: false, sandbox: true },
    })
    const record = { id, win, view: normalizedView, options: { ...options, id } }
    records.set(id, record)
    applyOptions(win, options)
    win.once('ready-to-show', () => { center(win); win.show() })
    if (options.hideOnBlur) win.on('blur', () => win.hide())
    win.on('closed', () => { if (records.get(id)?.win === win) records.delete(id) })
    deps.installNavigationPolicy(win, (url) => deps.isTrustedPage(url, id))
    win.webContents.on('did-fail-load', (_event, errorCode, errorDescription, validatedURL) => deps.debug?.('extensionWindow.didFailLoad', { id, errorCode, errorDescription, validatedURL }))
    load(win, id)
    return record
  }

  function executeWindowAction(action: any) {
    const id = String(action.windowId || action.id || '')
    if (action.type === 'createWindow') {
      createOrUpdate(action.view, action.windowOptions || {})
      return { toast: { message: 'Opened window' } }
    }
    const record = records.get(id)
    if (!record) {
      if (action.type === 'toggleWindow' && action.view) {
        createOrUpdate(action.view, { ...(action.windowOptions || {}), id })
        return { toast: { message: 'Opened window' } }
      }
      return { toast: { message: 'Window is not open', tone: 'error' } }
    }
    if (action.type === 'showWindow') { record.win.show(); record.win.focus(); return { toast: { message: 'Shown window' } } }
    if (action.type === 'hideWindow') { record.win.hide(); return { toast: { message: 'Hidden window' } } }
    if (action.type === 'toggleWindow') {
      if (action.view || action.windowOptions) createOrUpdate(action.view || record.view, { ...(record.options || {}), ...(action.windowOptions || {}), id })
      if (record.win.isVisible()) record.win.hide(); else { record.win.show(); record.win.focus() }
      return { toast: { message: 'Toggled window' } }
    }
    if (action.type === 'closeWindow') { record.win.close(); return { toast: { message: 'Closed window' } } }
    return null
  }

  function getState(id: string) {
    const record = records.get(String(id || ''))
    return record ? { id: record.id, view: record.view, options: record.options } : null
  }

  function closeAll() {
    for (const record of records.values()) record.win.close()
    records.clear()
  }

  return { records, createOrUpdate, executeWindowAction, getState, closeAll }
}
