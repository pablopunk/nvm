const { app, BrowserWindow, globalShortcut, ipcMain, shell, screen, clipboard, nativeTheme } = require('electron')
const fs = require('node:fs/promises')
const path = require('node:path')
const os = require('node:os')
const { spawn } = require('node:child_process')

const isDev = Boolean(process.env.ELECTRON_RENDERER_URL)
const HOTKEY = 'Alt+Space'

let win
let appIndex = []
let clipboardHistory = []
const appIconCache = new Map()

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

async function getAppIconDataUrl(appPath) {
  if (process.platform !== 'darwin' || !appPath || !appPath.endsWith('.app')) return null
  if (appIconCache.has(appPath)) return appIconCache.get(appPath)

  const promise = (async () => {
    try {
      const { fileIconToBuffer } = await import('file-icon')
      const png = await fileIconToBuffer(appPath, { size: 64 })
      return `data:image/png;base64,${Buffer.from(png).toString('base64')}`
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

function searchActions(query) {
  const q = query.trim()
  const results = []
  const url = getUrlFromQuery(q)

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

  if (q && !url) {
    results.push({
      id: `ai:${q}`,
      kind: 'ai-placeholder',
      title: `Press Tab to automate “${q}”`,
      subtitle: 'Automate with AI',
      query: q,
      icon: 'sparkles',
      score: 90,
    })
  }

  for (const item of appIndex) {
    const appScore = score(item.name, q)
    if (!q || appScore > 0) {
      results.push({
        id: `app:${item.id}`,
        kind: 'app',
        title: item.name,
        subtitle: 'Launch application',
        app: item,
        icon: 'app',
        score: appScore || 5,
      })
    }
  }

  for (const item of clipboardHistory) {
    const clipScore = score(item.text, q)
    if (!q || clipScore > 0) {
      results.push({
        id: `clipboard:${item.id}`,
        kind: 'clipboard',
        title: item.text.length > 72 ? `${item.text.slice(0, 72)}…` : item.text,
        subtitle: 'Copy from clipboard history',
        text: item.text,
        icon: 'clipboard',
        score: clipScore || 3,
      })
    }
  }

  if (q) {
    results.push({
      id: `web-search:${q}`,
      kind: 'web-search',
      title: `Search the web for “${q}”`,
      subtitle: 'Search instead',
      query: q,
      icon: 'search',
      score: 10,
    })
  }

  return results
    .sort((a, b) => b.score - a.score || a.title.localeCompare(b.title))
    .slice(0, 10)
}

async function executeAction(action) {
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
      clipboard.writeText(action.text)
      break
    case 'ai-placeholder':
      // TODO: open the AI script creation flow.
      break
  }
  hidePalette()
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

function startClipboardWatcher() {
  let last = ''
  setInterval(() => {
    const text = clipboard.readText().trim()
    if (!text || text === last) return
    last = text
    clipboardHistory = [
      { id: `${Date.now()}`, text },
      ...clipboardHistory.filter((item) => item.text !== text),
    ].slice(0, 50)
  }, 1200).unref?.()
}

app.whenReady().then(() => {
  nativeTheme.themeSource = 'dark'
  if (process.platform === 'darwin') app.dock.hide()

  createWindow()
  registerHotkey()
  startClipboardWatcher()
  setTimeout(indexApplications, 100)

  ipcMain.handle('actions:search', (_event, query) => searchActions(query))
  ipcMain.handle('actions:execute', (_event, action) => executeAction(action))
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
