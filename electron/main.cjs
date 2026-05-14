const { app, BrowserWindow, globalShortcut, ipcMain, shell, screen, clipboard, nativeImage, nativeTheme } = require('electron')
const fs = require('node:fs/promises')
const path = require('node:path')
const os = require('node:os')
const crypto = require('node:crypto')
const { spawn } = require('node:child_process')

const isDev = Boolean(process.env.ELECTRON_RENDERER_URL)
const HOTKEY = 'Alt+Space'
const CLIPBOARD_LIMIT = 100
const FILE_RESULT_LIMIT = 6
const CLIPBOARD_POLL_INTERVAL_MS = 1000

let win
let appIndex = []
let fileIndex = []
let clipboardHistory = []
let statePath
let iconCacheDir
let saveTimer
let userState = {
  recents: {},
  aliases: {},
  shortcuts: {},
  shortcutActions: {},
  overrides: {},
  clipboardHistory: [],
}
const appIconCache = new Map()

const BUILT_IN_ACTIONS = [
  {
    id: 'builtin:lock-screen',
    kind: 'builtin',
    builtin: 'lock-screen',
    title: 'Lock Screen',
    subtitle: 'Secure this computer',
    icon: 'lock',
    score: 22,
  },
  {
    id: 'builtin:sleep',
    kind: 'builtin',
    builtin: 'sleep',
    title: 'Sleep',
    subtitle: 'Put this computer to sleep',
    icon: 'moon',
    score: 21,
  },
  {
    id: 'builtin:restart',
    kind: 'builtin',
    builtin: 'restart',
    title: 'Restart Computer',
    subtitle: 'Restart this computer',
    icon: 'restart',
    score: 20,
  },
  {
    id: 'builtin:settings',
    kind: 'builtin',
    builtin: 'settings',
    title: process.platform === 'darwin' ? 'Open System Settings' : 'Open Settings',
    subtitle: 'Open system preferences',
    icon: 'settings',
    score: 19,
  },
  {
    id: 'builtin:downloads',
    kind: 'builtin',
    builtin: 'open-path',
    targetPath: path.join(os.homedir(), 'Downloads'),
    title: 'Open Downloads',
    subtitle: '~/Downloads',
    icon: 'folder',
    score: 18,
  },
  {
    id: 'builtin:documents',
    kind: 'builtin',
    builtin: 'open-path',
    targetPath: path.join(os.homedir(), 'Documents'),
    title: 'Open Documents',
    subtitle: '~/Documents',
    icon: 'folder',
    score: 17,
  },
  {
    id: 'builtin:desktop',
    kind: 'builtin',
    builtin: 'open-path',
    targetPath: path.join(os.homedir(), 'Desktop'),
    title: 'Open Desktop',
    subtitle: '~/Desktop',
    icon: 'folder',
    score: 16,
  },
  {
    id: 'builtin:quit',
    kind: 'builtin',
    builtin: 'quit',
    title: 'Quit Nevermind',
    subtitle: 'Close the app',
    icon: 'power',
    score: 15,
  },
]

function createWindow() {
  win = new BrowserWindow({
    width: 760,
    height: 520,
    show: false,
    frame: false,
    transparent: true,
    resizable: false,
    movable: true,
    alwaysOnTop: true,
    skipTaskbar: true,
    hasShadow: false,
    title: 'Nevermind',
    backgroundColor: '#00000000',
    webPreferences: {
      preload: path.join(__dirname, 'preload.cjs'),
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: false,
    },
  })

  win.on('blur', () => hidePalette())
  win.on('close', (event) => {
    if (!app.isQuiting) {
      event.preventDefault()
      hidePalette()
    }
  })

  win.webContents.once('did-finish-load', () => {
    if (isDev) showPalette()
  })

  if (isDev) {
    win.loadURL(process.env.ELECTRON_RENDERER_URL)
  } else {
    win.loadFile(path.join(__dirname, '..', 'dist', 'index.html'))
  }
}

function centerWindow() {
  if (!win) return
  const cursor = screen.getCursorScreenPoint()
  const display = screen.getDisplayNearestPoint(cursor)
  const { width, height } = win.getBounds()
  const { x, y, width: sw, height: sh } = display.workArea
  win.setBounds({
    x: Math.round(x + (sw - width) / 2),
    y: Math.round(y + Math.min(sh * 0.18, 180)),
    width,
    height,
  })
}

function showPalette() {
  if (!win) return
  centerWindow()
  win.show()
  win.focus()
  win.webContents.focus()
  win.webContents.send('palette:shown')
}

function hidePalette() {
  if (!win) return
  win.webContents.send('palette:hidden')
  win.hide()
}

function togglePalette() {
  if (win?.isVisible()) hidePalette()
  else showPalette()
}

function registerHotkey() {
  const ok = globalShortcut.register(HOTKEY, togglePalette)
  if (ok) console.log(`Registered global shortcut: ${HOTKEY}`)
  else console.warn(`Could not register global shortcut: ${HOTKEY}`)
}

function normalize(value) {
  return String(value || '').toLowerCase().trim()
}

function hashValue(value) {
  return crypto.createHash('sha1').update(String(value)).digest('hex')
}

function score(value, query) {
  const v = normalize(value)
  const q = normalize(query)
  if (!q) return 0
  if (v === q) return 100
  if (v.startsWith(q)) return 80
  if (v.includes(q)) return 50
  let pos = 0
  for (const ch of q) {
    pos = v.indexOf(ch, pos)
    if (pos === -1) return 0
    pos += 1
  }
  return 20
}

function actionAliases(actionId) {
  const value = userState.aliases[actionId]
  if (!value) return []
  return Array.isArray(value) ? value : [value]
}

function actionSearchScore(action, query) {
  if (!query.trim()) return action.score || 0
  return Math.max(
    score(action.title, query),
    score(action.subtitle, query),
    ...actionAliases(action.id).map((alias) => score(alias, query)),
  )
}

function recentBoost(actionId) {
  const recent = userState.recents[actionId]
  if (!recent) return 0
  const ageHours = Math.max(0, (Date.now() - recent.lastUsed) / 36e5)
  const recency = Math.max(0, 120 - ageHours * 3)
  const frequency = Math.min(30, (recent.count || 1) * 3)
  return recency + frequency
}

function defaultActionIdFor(action) {
  if (action.defaultActionId) return action.defaultActionId
  if (action.kind === 'builtin') return action.id
  if (action.kind === 'calculate') return 'default:calculator'
  return null
}

function withDefaultOverride(action) {
  const defaultActionId = defaultActionIdFor(action)
  if (!defaultActionId) return action
  const override = userState.overrides[defaultActionId]
  return {
    ...action,
    defaultActionId,
    isOverridden: Boolean(override),
    overrideSummary: override?.instruction,
  }
}

function rankAction(action, query) {
  const base = actionSearchScore(action, query)
  if (query.trim() && base <= 0) return null
  return {
    ...action,
    aliases: actionAliases(action.id),
    score: base + recentBoost(action.id),
    lastUsed: userState.recents[action.id]?.lastUsed || 0,
  }
}

function recordRecent(action) {
  if (!action?.id) return
  const current = userState.recents[action.id] || { count: 0, lastUsed: 0, title: action.title }
  userState.recents[action.id] = {
    count: current.count + 1,
    lastUsed: Date.now(),
    title: action.title,
    kind: action.kind,
  }
  scheduleSaveState()
}

function isLikelyUrl(input) {
  const value = input.trim()
  if (!value || value.includes(' ')) return false
  if (/^https?:\/\//i.test(value)) return true
  return /^[\w-]+(\.[\w-]+)+([/:?#].*)?$/i.test(value)
}

function getUrlFromQuery(query) {
  const trimmed = query.trim()
  const opened = trimmed.match(/^open\s+(.+)$/i)?.[1]?.trim() || trimmed
  if (!isLikelyUrl(opened)) return null
  return /^https?:\/\//i.test(opened) ? opened : `https://${opened}`
}

function calculate(query) {
  const expression = query.trim().replace(/^=?\s*/, '').replace(/^calc(?:ulate)?\s+/i, '')
  if (!expression || !/[+\-*/%^()]/.test(expression)) return null
  if (!/^[\d\s.+\-*/%^(),]+$/.test(expression)) return null

  try {
    const jsExpression = expression.replace(/%/g, '/100')
    const result = Function(`"use strict"; return (${jsExpression})`)()
    if (typeof result !== 'number' || !Number.isFinite(result)) return null
    return Number.isInteger(result) ? String(result) : String(Number(result.toPrecision(12)))
  } catch {
    return null
  }
}

async function getAppIconDataUrl(appPath) {
  if (process.platform !== 'darwin' || !appPath || !appPath.endsWith('.app')) return null
  if (appIconCache.has(appPath)) return appIconCache.get(appPath)

  const promise = (async () => {
    try {
      const cachePath = path.join(iconCacheDir, `${hashValue(appPath)}.png`)
      const cached = await fs.readFile(cachePath).catch(() => null)
      if (cached) return `data:image/png;base64,${cached.toString('base64')}`

      const { fileIconToBuffer } = await import('file-icon')
      const png = Buffer.from(await fileIconToBuffer(appPath, { size: 64 }))
      await fs.mkdir(iconCacheDir, { recursive: true })
      await fs.writeFile(cachePath, png).catch(() => {})
      return `data:image/png;base64,${png.toString('base64')}`
    } catch (error) {
      console.warn(`Failed to load app icon for ${appPath}`, error)
      return null
    }
  })()

  appIconCache.set(appPath, promise)
  const result = await promise
  appIconCache.set(appPath, result)
  return result
}

function clipboardActionFromItem(item) {
  return {
    id: `clipboard:${item.id}`,
    kind: 'clipboard',
    title: clipboardItemTitle(item),
    subtitle: clipboardItemSubtitle(item),
    clipboardType: item.type,
    text: item.text,
    imageDataUrl: item.imageDataUrl,
    thumbnailUrl: item.thumbnailUrl,
    icon: 'clipboard',
    score: 3,
    lastUsed: item.createdAt || 0,
  }
}

function searchActions(query, options = {}) {
  const q = query.trim()

  if (options.clipboardOnly) {
    return clipboardHistory.map(clipboardActionFromItem).slice(0, CLIPBOARD_LIMIT)
  }

  const results = []
  const url = getUrlFromQuery(q)
  const mathResult = q ? calculate(q) : null

  if (url) {
    results.push({
      id: `open-url:${url}`,
      kind: 'open-url',
      title: `Open ${url.replace(/^https?:\/\//, '')}`,
      subtitle: 'Open website',
      url,
      icon: 'globe',
      score: 100,
    })
  }

  if (mathResult !== null) {
    results.push({
      id: `calculate:${q}`,
      kind: 'calculate',
      title: `${q} = ${mathResult}`,
      subtitle: 'Copy result to clipboard',
      query: q,
      result: mathResult,
      icon: 'calculator',
      score: 105,
    })
  }

  for (const item of BUILT_IN_ACTIONS) {
    const ranked = rankAction(withDefaultOverride(item), q)
    if (ranked) results.push(ranked)
  }

  const latestClipboardTime = clipboardHistory[0]?.createdAt || 0
  const clipboardHistoryAction = rankAction({
    id: 'clipboard-history',
    kind: 'clipboard-history',
    title: 'Clipboard History',
    subtitle: clipboardHistory.length ? `Show all ${clipboardHistory.length} copied items` : 'Show copied items',
    icon: 'clipboard',
    score: 14,
    lastUsed: latestClipboardTime ? latestClipboardTime - 1 : 0,
  }, q)
  if (clipboardHistoryAction) {
    clipboardHistoryAction.lastUsed = Math.max(clipboardHistoryAction.lastUsed || 0, latestClipboardTime ? latestClipboardTime - 1 : 0)
    results.push(clipboardHistoryAction)
  }

  for (const item of appIndex) {
    const action = {
      id: `app:${item.id}`,
      kind: 'app',
      title: item.name,
      subtitle: 'Launch application',
      app: item,
      icon: 'app',
      score: 5,
    }
    const ranked = rankAction(action, q)
    if (ranked) results.push(ranked)
  }

  let fileMatches = 0
  for (const item of fileIndex) {
    if (fileMatches >= FILE_RESULT_LIMIT) break
    const action = {
      id: `file:${item.path}`,
      kind: 'file',
      title: item.name,
      subtitle: item.displayPath,
      filePath: item.path,
      icon: 'folder',
      score: 4,
    }
    const ranked = rankAction(action, q)
    if (ranked) {
      results.push(ranked)
      fileMatches += 1
    }
  }

  for (const item of clipboardHistory) {
    const ranked = rankAction(clipboardActionFromItem(item), q)
    if (ranked) {
      ranked.lastUsed = Math.max(ranked.lastUsed || 0, item.createdAt || 0)
      results.push(ranked)
    }
  }

  if (q && !url && mathResult === null) {
    const hasLocalResults = results.some(
      (r) => r.kind === 'builtin' || r.kind === 'app' || r.kind === 'file' || r.kind === 'clipboard',
    )
    if (!hasLocalResults) {
      results.push({
        id: `ai:${q}`,
        kind: 'ai-placeholder',
        title: `Press Tab to automate "${q}"`,
        subtitle: 'Automate with AI',
        query: q,
        icon: 'bolt',
        score: 90,
      })
    }
  }

  if (q) {
    results.push({
      id: `web-search:${q}`,
      kind: 'web-search',
      title: `Search the web for "${q}"`,
      subtitle: 'Search instead',
      query: q,
      icon: 'search',
      score: 10 + recentBoost(`web-search:${q}`),
    })
  }

  return results
    .sort((a, b) => {
      if (!q && a.lastUsed !== b.lastUsed) return b.lastUsed - a.lastUsed
      return b.score - a.score || b.lastUsed - a.lastUsed || a.title.localeCompare(b.title)
    })
    .slice(0, 10)
}

async function executeAction(action, options = {}) {
  if (!action) return
  recordRecent(action)

  switch (action.kind) {
    case 'open-url':
      await shell.openExternal(action.url)
      break
    case 'web-search':
      await shell.openExternal(`https://www.google.com/search?q=${encodeURIComponent(action.query)}`)
      break
    case 'app':
      await launchApp(action.app)
      break
    case 'clipboard':
      if (action.clipboardType === 'image' && action.imageDataUrl) {
        clipboard.writeImage(nativeImage.createFromDataURL(action.imageDataUrl))
      } else {
        clipboard.writeText(action.text || '')
      }
      break
    case 'file':
      await shell.openPath(action.filePath)
      break
    case 'calculate':
      clipboard.writeText(action.result)
      break
    case 'builtin':
      await executeBuiltin(action)
      break
    case 'ai-placeholder':
      // TODO: open the AI script creation flow.
      break
  }

  if (!options.keepPaletteOpen) hidePalette()
}

async function executeBuiltin(action) {
  switch (action.builtin) {
    case 'lock-screen':
      if (process.platform === 'darwin') {
        spawn('/System/Library/CoreServices/Menu Extras/User.menu/Contents/Resources/CGSession', ['-suspend'], { detached: true, stdio: 'ignore' }).unref()
      } else if (process.platform === 'win32') {
        spawn('rundll32.exe', ['user32.dll,LockWorkStation'], { detached: true, stdio: 'ignore' }).unref()
      } else {
        spawn('sh', ['-lc', 'loginctl lock-session || xdg-screensaver lock || gnome-screensaver-command -l'], { detached: true, stdio: 'ignore' }).unref()
      }
      break
    case 'sleep':
      if (process.platform === 'darwin') spawn('pmset', ['sleepnow'], { detached: true, stdio: 'ignore' }).unref()
      else if (process.platform === 'win32') spawn('rundll32.exe', ['powrprof.dll,SetSuspendState', '0,1,0'], { detached: true, stdio: 'ignore' }).unref()
      else spawn('systemctl', ['suspend'], { detached: true, stdio: 'ignore' }).unref()
      break
    case 'restart':
      if (process.platform === 'darwin') spawn('osascript', ['-e', 'tell application "System Events" to restart'], { detached: true, stdio: 'ignore' }).unref()
      else spawn('shutdown', [process.platform === 'win32' ? '/r' : '-r', process.platform === 'win32' ? '/t' : 'now', process.platform === 'win32' ? '0' : ''], { detached: true, stdio: 'ignore' }).unref()
      break
    case 'settings':
      if (process.platform === 'darwin') await shell.openExternal('x-apple.systempreferences:')
      else if (process.platform === 'win32') await shell.openExternal('ms-settings:')
      else spawn('sh', ['-lc', 'gnome-control-center || systemsettings || xfce4-settings-manager'], { detached: true, stdio: 'ignore' }).unref()
      break
    case 'open-path':
      await shell.openPath(action.targetPath)
      break
    case 'quit':
      app.isQuiting = true
      app.quit()
      break
  }
}

async function launchApp(item) {
  if (!item) return
  if (process.platform === 'darwin') {
    spawn('open', [item.path], { detached: true, stdio: 'ignore' }).unref()
    return
  }
  if (process.platform === 'win32') {
    await shell.openPath(item.path)
    return
  }
  if (item.command) {
    spawn(item.command, { shell: true, detached: true, stdio: 'ignore' }).unref()
    return
  }
  await shell.openPath(item.path)
}

async function scanMacApps() {
  const roots = ['/Applications', '/System/Applications', path.join(os.homedir(), 'Applications')]
  const found = []

  async function walk(dir, depth) {
    let entries = []
    try {
      entries = await fs.readdir(dir, { withFileTypes: true })
    } catch {
      return
    }

    await Promise.all(entries.map(async (entry) => {
      if (!entry.isDirectory() || entry.name.startsWith('.')) return
      const fullPath = path.join(dir, entry.name)
      if (entry.name.endsWith('.app')) {
        found.push({
          id: fullPath,
          name: entry.name.replace(/\.app$/i, ''),
          path: fullPath,
        })
        return
      }
      if (depth > 0) await walk(fullPath, depth - 1)
    }))
  }

  await Promise.all(roots.map((root) => walk(root, 2)))
  return found
}

function displayUserPath(filePath) {
  const home = os.homedir()
  return filePath.startsWith(home) ? `~${filePath.slice(home.length)}` : filePath
}

async function scanFiles() {
  const roots = ['Desktop', 'Documents', 'Downloads'].map((name) => path.join(os.homedir(), name))
  const ignored = new Set(['node_modules', '.git', 'Library', 'Applications'])
  const found = []

  async function walk(dir, depth) {
    let entries = []
    try {
      entries = await fs.readdir(dir, { withFileTypes: true })
    } catch {
      return
    }

    await Promise.all(entries.map(async (entry) => {
      if (entry.name.startsWith('.') || ignored.has(entry.name)) return
      const fullPath = path.join(dir, entry.name)
      if (entry.isFile()) {
        found.push({
          id: fullPath,
          name: entry.name,
          path: fullPath,
          displayPath: displayUserPath(fullPath),
        })
        return
      }
      if (entry.isDirectory() && depth > 0) await walk(fullPath, depth - 1)
    }))
  }

  await Promise.all(roots.map((root) => walk(root, 2)))
  return found.slice(0, 5000)
}

async function scanWindowsApps() {
  const roots = [
    process.env.ProgramData && path.join(process.env.ProgramData, 'Microsoft', 'Windows', 'Start Menu', 'Programs'),
    process.env.APPDATA && path.join(process.env.APPDATA, 'Microsoft', 'Windows', 'Start Menu', 'Programs'),
  ].filter(Boolean)
  const found = []

  async function walk(dir) {
    let entries = []
    try {
      entries = await fs.readdir(dir, { withFileTypes: true })
    } catch {
      return
    }
    await Promise.all(entries.map(async (entry) => {
      const fullPath = path.join(dir, entry.name)
      if (entry.isDirectory()) return walk(fullPath)
      if (entry.name.endsWith('.lnk')) {
        found.push({ id: fullPath, name: entry.name.replace(/\.lnk$/i, ''), path: fullPath })
      }
    }))
  }

  await Promise.all(roots.map(walk))
  return found
}

async function scanLinuxApps() {
  const roots = ['/usr/share/applications', '/usr/local/share/applications', path.join(os.homedir(), '.local/share/applications')]
  const found = []

  await Promise.all(roots.map(async (root) => {
    let entries = []
    try {
      entries = await fs.readdir(root, { withFileTypes: true })
    } catch {
      return
    }

    await Promise.all(entries.map(async (entry) => {
      if (!entry.isFile() || !entry.name.endsWith('.desktop')) return
      const fullPath = path.join(root, entry.name)
      const body = await fs.readFile(fullPath, 'utf8').catch(() => '')
      if (/^(NoDisplay|Hidden)=true$/im.test(body)) return
      const name = body.match(/^Name=(.+)$/m)?.[1]
      const exec = body.match(/^Exec=(.+)$/m)?.[1]
      if (!name || !exec) return
      const command = exec.replace(/\s*%[fFuUdDnNickvm]/g, '').trim()
      found.push({ id: fullPath, name, path: fullPath, command })
    }))
  }))

  return found
}

async function indexApplications() {
  try {
    const apps = process.platform === 'darwin'
      ? await scanMacApps()
      : process.platform === 'win32'
        ? await scanWindowsApps()
        : await scanLinuxApps()

    const deduped = new Map()
    for (const item of apps) deduped.set(normalize(item.name), item)
    appIndex = Array.from(deduped.values()).sort((a, b) => a.name.localeCompare(b.name))
    win?.webContents.send('apps:indexed', appIndex.length)
  } catch (error) {
    console.error('Failed to index applications', error)
  }
}

async function indexFiles() {
  try {
    fileIndex = await scanFiles()
  } catch (error) {
    console.error('Failed to index files', error)
  }
}

async function loadUserState() {
  statePath = path.join(app.getPath('userData'), 'state.json')
  iconCacheDir = path.join(app.getPath('userData'), 'icon-cache')

  try {
    const loaded = JSON.parse(await fs.readFile(statePath, 'utf8'))
    userState = {
      recents: loaded.recents || {},
      aliases: loaded.aliases || {},
      shortcuts: loaded.shortcuts || {},
      shortcutActions: loaded.shortcutActions || {},
      overrides: loaded.overrides || {},
      clipboardHistory: loaded.clipboardHistory || [],
    }
  } catch {
    // First run.
  }

  clipboardHistory = normalizeClipboardHistory(userState.clipboardHistory)
}

function normalizeClipboardHistory(items) {
  return (Array.isArray(items) ? items : [])
    .map((item) => {
      if (item?.type === 'image' && item.imageDataUrl) {
        return {
          id: item.id || `image:${hashValue(item.imageDataUrl)}`,
          type: 'image',
          imageDataUrl: item.imageDataUrl,
          thumbnailUrl: item.thumbnailUrl || item.imageDataUrl,
          createdAt: item.createdAt || Date.now(),
        }
      }
      if (item?.text) {
        const text = String(item.text).trim()
        if (!text) return null
        return {
          id: item.id?.startsWith('text:') ? item.id : `text:${hashValue(text)}`,
          type: 'text',
          text,
          createdAt: item.createdAt || Date.now(),
        }
      }
      return null
    })
    .filter(Boolean)
    .slice(0, CLIPBOARD_LIMIT)
}

function clipboardItemTitle(item) {
  if (item.type === 'image') return 'Clipboard image'
  return item.text.length > 72 ? `${item.text.slice(0, 72)}…` : item.text
}

function clipboardItemSubtitle(item) {
  const when = new Date(item.createdAt || Date.now()).toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  })
  return item.type === 'image' ? `Image copied ${when}` : `Copied ${when}`
}

function readClipboardItem() {
  const text = clipboard.readText().trim()
  if (text) {
    return {
      id: `text:${hashValue(text)}`,
      type: 'text',
      text,
      createdAt: Date.now(),
    }
  }

  const image = clipboard.readImage()
  if (image.isEmpty()) return null

  const png = image.toPNG()
  const thumbnail = image.resize({ width: 64, height: 64 })
  return {
    id: `image:${hashValue(png)}`,
    type: 'image',
    imageDataUrl: `data:image/png;base64,${png.toString('base64')}`,
    thumbnailUrl: thumbnail.toDataURL(),
    createdAt: Date.now(),
  }
}

function rememberClipboardItem(item) {
  if (!item) return
  clipboardHistory = [
    item,
    ...clipboardHistory.filter((current) => current.id !== item.id),
  ].slice(0, CLIPBOARD_LIMIT)
  scheduleSaveState()
  win?.webContents.send('clipboard:changed')
}

function scheduleSaveState() {
  if (saveTimer) clearTimeout(saveTimer)
  saveTimer = setTimeout(saveUserState, 200)
  saveTimer.unref?.()
}

async function saveUserState() {
  userState.clipboardHistory = clipboardHistory
  await fs.mkdir(path.dirname(statePath), { recursive: true })
  await fs.writeFile(statePath, JSON.stringify(userState, null, 2)).catch((error) => {
    console.error('Failed to save state', error)
  })
}

function startClipboardWatcher() {
  let lastId = readClipboardItem()?.id || ''
  setInterval(() => {
    const item = readClipboardItem()
    if (!item || item.id === lastId) return
    lastId = item.id
    rememberClipboardItem(item)
  }, CLIPBOARD_POLL_INTERVAL_MS).unref?.()
}

function normalizeAccelerator(value) {
  return String(value || '')
    .split('+')
    .map((part) => part.trim())
    .filter(Boolean)
    .map((part) => {
      const normalized = part.toLowerCase()
      if (['cmd', 'command', '⌘'].includes(normalized)) return 'Command'
      if (['ctrl', 'control', '^'].includes(normalized)) return 'Control'
      if (['option', 'opt', 'alt', '⌥'].includes(normalized)) return 'Alt'
      if (['shift', '⇧'].includes(normalized)) return 'Shift'
      if (['enter', 'return', '↵'].includes(normalized)) return 'Enter'
      if (['esc', 'escape'].includes(normalized)) return 'Escape'
      if (normalized === 'space') return 'Space'
      return part.length === 1 ? part.toUpperCase() : part
    })
    .join('+')
}

function unregisterShortcutForAction(actionId) {
  const current = userState.shortcuts[actionId]
  if (current) globalShortcut.unregister(current)
}

function registerActionShortcut(actionId, accelerator, action) {
  globalShortcut.unregister(accelerator)
  const ok = globalShortcut.register(accelerator, () => executeAction(action, { keepPaletteOpen: true }))
  if (!ok) return false
  userState.shortcuts[actionId] = accelerator
  userState.shortcutActions[actionId] = action
  return true
}

function registerActionShortcuts() {
  for (const [actionId, accelerator] of Object.entries(userState.shortcuts)) {
    const action = userState.shortcutActions[actionId]
    if (!action) continue
    const ok = globalShortcut.register(accelerator, () => executeAction(action, { keepPaletteOpen: true }))
    if (!ok) console.warn(`Could not register action shortcut ${accelerator} for ${actionId}`)
  }
}

async function setAlias(action, alias) {
  if (!action?.id || !alias.trim()) return { ok: false, message: 'Missing alias' }
  const aliases = new Set(actionAliases(action.id))
  aliases.add(alias.trim())
  userState.aliases[action.id] = Array.from(aliases)
  scheduleSaveState()
  return { ok: true, message: `Alias set: ${alias.trim()}` }
}

async function setShortcut(action, shortcut) {
  if (!action?.id || !shortcut.trim()) return { ok: false, message: 'Missing shortcut' }
  const accelerator = normalizeAccelerator(shortcut)
  unregisterShortcutForAction(action.id)
  const ok = registerActionShortcut(action.id, accelerator, action)
  if (!ok) return { ok: false, message: `Could not register ${accelerator}` }
  scheduleSaveState()
  return { ok: true, message: `Shortcut set: ${accelerator}` }
}

async function setOverride(action, instruction) {
  const defaultActionId = defaultActionIdFor(action)
  if (!defaultActionId) return { ok: false, message: 'This action cannot be overridden yet' }
  if (!instruction.trim()) return { ok: false, message: 'Missing override instructions' }
  userState.overrides[defaultActionId] = {
    instruction: instruction.trim(),
    updatedAt: Date.now(),
    originalTitle: action.title,
  }
  scheduleSaveState()
  return { ok: true, message: 'Override saved' }
}

async function clearOverride(action) {
  const defaultActionId = defaultActionIdFor(action)
  if (!defaultActionId) return { ok: false, message: 'This action has no original to restore' }
  delete userState.overrides[defaultActionId]
  scheduleSaveState()
  return { ok: true, message: 'Original restored' }
}

app.whenReady().then(async () => {
  nativeTheme.themeSource = 'dark'
  if (process.platform === 'darwin') app.dock.hide()

  await loadUserState()
  createWindow()
  registerHotkey()
  registerActionShortcuts()
  startClipboardWatcher()
  setTimeout(indexApplications, 100)
  setTimeout(indexFiles, 200)

  ipcMain.handle('actions:search', (_event, query, options) => searchActions(query, options))
  ipcMain.handle('actions:execute', (_event, action) => executeAction(action))
  ipcMain.handle('actions:set-alias', (_event, action, alias) => setAlias(action, alias))
  ipcMain.handle('actions:set-shortcut', (_event, action, shortcut) => setShortcut(action, shortcut))
  ipcMain.handle('actions:set-override', (_event, action, instruction) => setOverride(action, instruction))
  ipcMain.handle('actions:clear-override', (_event, action) => clearOverride(action))
  ipcMain.handle('apps:icon', (_event, appPath) => getAppIconDataUrl(appPath))
  ipcMain.handle('palette:hide', () => hidePalette())
})

app.on('activate', () => showPalette())
app.on('will-quit', () => {
  app.isQuiting = true
  globalShortcut.unregisterAll()
})

const gotLock = app.requestSingleInstanceLock()
if (!gotLock) app.quit()
else app.on('second-instance', () => showPalette())
