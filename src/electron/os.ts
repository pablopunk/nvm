import { app, shell, type BrowserWindow, type BrowserWindowConstructorOptions } from 'electron'
import fs from 'node:fs/promises'
import fsSync from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { execFile, spawn } from 'node:child_process'

type OsDependent<T> = Partial<Record<'darwin' | 'linux' | 'win32', T>> & { default?: T }

function noop() {}

function osDependent<T>(handlers: OsDependent<T>, fallback: T): T {
  return handlers[process.platform as 'darwin' | 'linux' | 'win32'] ?? handlers.default ?? fallback
}

function osFunction<TArgs extends unknown[], TResult>(handlers: OsDependent<(...args: TArgs) => TResult>, fallback: (...args: TArgs) => TResult = noop as (...args: TArgs) => TResult) {
  return osDependent(handlers, fallback)
}

export function osLabel() {
  return osDependent({ darwin: 'macOS', win32: 'Windows', linux: 'Linux' }, 'Linux')
}

const macOnlyCapabilities = new Set(['quick-look', 'selected-files', 'selected-text', 'frontmost-app', 'frontmost-paste', 'keyboard.type-text', 'applescript', 'app-icons', 'open-with', 'keyboard-settings', 'window-panel-policy', 'file-date-added'])

export function hasCapability(capability: string) {
  if (macOnlyCapabilities.has(capability)) return osDependent({ darwin: true }, false)
  if (capability === 'auto-updates') return osDependent({ darwin: true, linux: Boolean(process.env.APPIMAGE) }, false)
  if (capability === 'camera') return osDependent({ darwin: true, win32: true, linux: true }, false)
  return true
}

export function canRequestMediaPermission(permission: string) {
  if (permission === 'media') return hasCapability('camera')
  return true
}

export function settingsTitle() {
  return osDependent({ darwin: 'Open System Settings' }, 'Open Settings')
}

export function revealPathTitle() {
  return osDependent({ darwin: 'Show in Finder', win32: 'Show in File Explorer' }, 'Show in File Manager')
}

export function revealPathDescription() {
  return osDependent({ darwin: 'Show in Finder', win32: 'Show in File Explorer' }, 'Show in the system file manager')
}

export function quickLookTitle() {
  return osDependent({ darwin: 'Quick Look' }, 'Preview File')
}

export function quickLookDescription() {
  return osDependent({ darwin: 'Open Quick Look' }, 'Preview this file')
}

export function isReservedPaletteAccelerator(accelerator: string) {
  return osFunction({ darwin: () => accelerator === 'Command+Space' }, () => false)()
}

export function reservedPaletteShortcutName() {
  return osDependent({ darwin: 'Spotlight' }, 'the system')
}

export function keyboardSettingsSubtitle() {
  return osDependent({ darwin: 'System Settings → Keyboard → Keyboard Shortcuts', win32: 'Windows Settings' }, 'System keyboard shortcuts')
}

export function keyboardShortcutConflictContent(label: string) {
  return osFunction({
    darwin: () => `# ${label} is used by Spotlight\n\nmacOS has \`${label}\` bound to Spotlight, so Nevermind cannot toggle with it until you disable that binding.\n\nOpen **System Settings → Keyboard → Keyboard Shortcuts → Spotlight** and uncheck *Show Spotlight search*.`,
  }, () => `# ${label} is used by the system\n\nThe current desktop has \`${label}\` reserved, so Nevermind cannot use it until that binding is changed.`)()
}

export function paletteBrowserWindowOptions(): Partial<BrowserWindowConstructorOptions> {
  return osDependent<Partial<BrowserWindowConstructorOptions>>({ darwin: { type: 'panel' } }, {})
}

export function applyPaletteWindowPolicy(win: BrowserWindow | null) {
  return osFunction<[BrowserWindow | null], void>({
    darwin: (window) => {
      if (!window) return
      window.setAlwaysOnTop(true, 'screen-saver')
      window.setVisibleOnAllWorkspaces(true, { visibleOnFullScreen: true })
      window.setWindowButtonVisibility?.(false)
      window.setFullScreenable(false)
      window.setSkipTaskbar(true)
    },
  })(win)
}

export function prepareAppWindowPolicy() {
  return osFunction({
    darwin: () => {
      app.setActivationPolicy('accessory')
      app.dock?.hide()
    },
  })()
}

export function supportsAutoUpdates() {
  return hasCapability('auto-updates')
}

export function autoUpdatesUnavailableMessage() {
  return 'Automatic updates only run from packaged macOS builds or Linux AppImages'
}

export function appScanRoots() {
  return osFunction({
    darwin: () => ['/Applications', '/System/Applications', '/System/Library/CoreServices/Applications', path.join(os.homedir(), 'Applications')],
    win32: () => [process.env.ProgramData && path.join(process.env.ProgramData, 'Microsoft', 'Windows', 'Start Menu', 'Programs'), process.env.APPDATA && path.join(process.env.APPDATA, 'Microsoft', 'Windows', 'Start Menu', 'Programs')].filter(Boolean) as string[],
  }, () => ['/usr/share/applications', '/usr/local/share/applications', path.join(os.homedir(), '.local/share/applications')])()
}

export async function launchApp(item: any) {
  if (!item) return
  return osFunction<[any], any>({
    darwin: (appItem) => spawn('open', [appItem.path], { detached: true, stdio: 'ignore' }).unref(),
    win32: (appItem) => shell.openPath(appItem.path),
  }, (appItem) => {
    if (appItem.command) return spawn(appItem.command, { shell: true, detached: true, stdio: 'ignore' }).unref()
    return shell.openPath(appItem.path)
  })(item)
}

const macSystemApps = ['/System/Library/CoreServices/Finder.app']

async function scanMacApps() {
  const found: any[] = []
  for (const appPath of macSystemApps) {
    if (fsSync.existsSync(appPath)) found.push({ id: appPath, name: path.basename(appPath).replace(/\.app$/i, ''), path: appPath })
  }

  async function walk(dir: string, depth: number) {
    const entries = await fs.readdir(dir, { withFileTypes: true }).catch(() => [])
    await Promise.all(entries.map(async (entry) => {
      if (!entry.isDirectory() || entry.name.startsWith('.')) return
      const fullPath = path.join(dir, entry.name)
      if (entry.name.endsWith('.app')) return found.push({ id: fullPath, name: entry.name.replace(/\.app$/i, ''), path: fullPath })
      if (depth > 0) await walk(fullPath, depth - 1)
    }))
  }
  await Promise.all(appScanRoots().map((root) => walk(root, 2)))
  return found
}

async function scanWindowsApps() {
  const found: any[] = []
  async function walk(dir: string) {
    const entries = await fs.readdir(dir, { withFileTypes: true }).catch(() => [])
    await Promise.all(entries.map(async (entry) => {
      const fullPath = path.join(dir, entry.name)
      if (entry.isDirectory()) return walk(fullPath)
      if (entry.name.endsWith('.lnk')) found.push({ id: fullPath, name: entry.name.replace(/\.lnk$/i, ''), path: fullPath })
    }))
  }
  await Promise.all(appScanRoots().map(walk))
  return found
}

async function scanLinuxApps() {
  const found: any[] = []
  await Promise.all(appScanRoots().map(async (root) => {
    const entries = await fs.readdir(root, { withFileTypes: true }).catch(() => [])
    await Promise.all(entries.map(async (entry) => {
      if (!entry.isFile() || !entry.name.endsWith('.desktop')) return
      const fullPath = path.join(root, entry.name)
      const body = await fs.readFile(fullPath, 'utf8').catch(() => '')
      if (/^(NoDisplay|Hidden)=true$/im.test(body)) return
      const name = body.match(/^Name=(.+)$/m)?.[1]
      const exec = body.match(/^Exec=(.+)$/m)?.[1]
      if (!name || !exec) return
      found.push({ id: fullPath, name, path: fullPath, command: exec.replace(/\s*%[fFuUdDnNickvm]/g, '').trim() })
    }))
  }))
  return found
}

export async function scanApps() {
  return osFunction({ darwin: scanMacApps, win32: scanWindowsApps }, scanLinuxApps)()
}

export function watchApps(onChange: () => void) {
  const watchers: Array<{ close: () => unknown }> = []
  for (const root of appScanRoots()) {
    if (!fsSync.existsSync(root)) continue
    try {
      const watcher = fsSync.watch(root, { recursive: osDependent({ darwin: true, win32: true }, false) }, onChange)
      watcher.on('error', () => {})
      watchers.push(watcher)
    } catch {}
  }
  return watchers
}

function runAppleScript(script: string, timeout = 30_000) {
  return new Promise<{ stdout: string; stderr: string; exitCode: number }>((resolve) => {
    if (!hasCapability('applescript')) return resolve({ stdout: '', stderr: 'AppleScript is not available on this OS', exitCode: 1 })
    execFile('osascript', ['-e', script], { timeout }, (error, stdout, stderr) => resolve({ stdout, stderr: stderr || error?.message || '', exitCode: error ? 1 : 0 }))
  })
}

function parseMetadataDate(value: string) {
  const text = value.trim()
  if (!text || text === '(null)') return 0
  const match = text.match(/^(\d{4}-\d{2}-\d{2}) (\d{2}:\d{2}:\d{2}) ([+-]\d{2})(\d{2})$/)
  const timestamp = Date.parse(match ? `${match[1]}T${match[2]}${match[3]}:${match[4]}` : text)
  return Number.isFinite(timestamp) ? timestamp : 0
}

function chunkArray<T>(items: T[], size: number) {
  const chunks: T[][] = []
  for (let index = 0; index < items.length; index += size) chunks.push(items.slice(index, index + size))
  return chunks
}

export async function fileDateAddedMs(paths: string[]) {
  if (!hasCapability('file-date-added') || paths.length === 0) return new Map<string, number>()
  const dates = new Map<string, number>()
  await Promise.all(chunkArray(paths, 100).map(async (chunk) => {
    try {
      const output = await new Promise<string>((resolve, reject) => {
        execFile('mdls', ['-raw', '-name', 'kMDItemDateAdded', ...chunk], { timeout: 2_000, maxBuffer: 1024 * 1024 }, (error, stdout) => error ? reject(error) : resolve(stdout))
      })
      const values = output.split('\0').flatMap((part) => part.split('\n')).map((part) => part.trim()).filter(Boolean)
      chunk.forEach((filePath, index) => dates.set(filePath, parseMetadataDate(values[index] || '')))
    } catch {
      chunk.forEach((filePath) => dates.set(filePath, 0))
    }
  }))
  return dates
}

export function pasteIntoFrontmostApp() {
  return osFunction({ darwin: () => execFile('osascript', ['-e', 'tell application "System Events" to keystroke "v" using command down'], () => {}) })()
}

function appleScriptString(value: string) {
  return `"${String(value).replace(/\\/g, '\\\\').replace(/"/g, '\\"').replace(/\r?\n/g, '" & return & "')}"`
}

export function typeTextIntoFrontmostApp(text: string, options: any = {}) {
  return osFunction<[string, any], any>({
    darwin: (value, opts) => execFile('osascript', ['-e', `tell application "System Events" to keystroke ${appleScriptString(value)}`], { timeout: Math.max(5_000, Number(opts?.timeoutMs || 30_000)) }, () => {}),
  }, () => null)(text, options)
}

export async function selectedFilePaths() {
  return osFunction({
    darwin: async () => {
      const script = 'set AppleScript\'s text item delimiters to linefeed\ntell application "Finder"\ntry\nset selectedItems to selection as alias list\non error\nreturn ""\nend try\nset selectedPaths to {}\nrepeat with selectedItem in selectedItems\nset end of selectedPaths to POSIX path of (selectedItem as alias)\nend repeat\nreturn selectedPaths as text\nend tell'
      const result = await runAppleScript(script)
      if (result.exitCode !== 0) return []
      return result.stdout.split(/\r?\n/).map((item) => item.trim()).filter(Boolean)
    },
  }, async () => [])()
}

export async function selectedText() {
  return osFunction({
    darwin: async () => {
      const script = 'tell application "System Events"\nset frontProcess to first application process whose frontmost is true\ntry\nset selectedText to value of attribute "AXSelectedText" of focused UI element of frontProcess\nif selectedText is missing value then return ""\nreturn selectedText as text\non error\nreturn ""\nend try\nend tell'
      const result = await runAppleScript(script, 5_000)
      const text = result.stdout.trim()
      return text || null
    },
  }, async () => null)()
}

export async function frontmostApp() {
  return osFunction({
    darwin: async () => {
      const script = 'tell application "System Events"\nset frontProcess to first application process whose frontmost is true\nset appName to name of frontProcess\nset appBundle to bundle identifier of frontProcess\ntry\nset appPath to POSIX path of (file of frontProcess as alias)\non error\nset appPath to ""\nend try\nreturn appName & linefeed & appBundle & linefeed & appPath\nend tell'
      const result = await runAppleScript(script, 5_000)
      if (result.exitCode !== 0) return null
      const [name, bundleId, appPath] = result.stdout.split(/\r?\n/)
      return name ? { name, bundleId: bundleId || null, path: appPath || null } : null
    },
  }, async () => null)()
}

function detached(command: string, args: string[] = []) {
  spawn(command, args, { detached: true, stdio: 'ignore' }).unref()
}

function detachedShell(script: string) {
  detached('sh', ['-lc', script])
}

export async function executeSystemBuiltin(action: any, quit: () => void) {
  switch (action.builtin) {
    case 'lock-screen':
      return osFunction({
        darwin: () => detached('/System/Library/CoreServices/Menu Extras/User.menu/Contents/Resources/CGSession', ['-suspend']),
        win32: () => detached('rundll32.exe', ['user32.dll,LockWorkStation']),
      }, () => detachedShell('loginctl lock-session || xdg-screensaver lock || gnome-screensaver-command -l'))()
    case 'sleep':
      return osFunction({
        darwin: () => detached('pmset', ['sleepnow']),
        win32: () => detached('rundll32.exe', ['powrprof.dll,SetSuspendState', '0,1,0']),
      }, () => detached('systemctl', ['suspend']))()
    case 'restart':
      return osFunction({
        darwin: () => detached('osascript', ['-e', 'tell application "System Events" to restart']),
        win32: () => detached('shutdown', ['/r', '/t', '0']),
      }, () => detached('shutdown', ['-r', 'now']))()
    case 'settings':
      return osFunction({
        darwin: () => shell.openExternal('x-apple.systempreferences:'),
        win32: () => shell.openExternal('ms-settings:'),
      }, () => Promise.resolve(detachedShell('gnome-control-center || systemsettings || xfce4-settings-manager')))()
    case 'open-keyboard-settings':
      return osFunction({
        darwin: () => shell.openExternal('x-apple.systempreferences:com.apple.Keyboard-Settings.extension'),
        win32: () => shell.openExternal('ms-settings:typing'),
      }, () => Promise.resolve(detachedShell('gnome-control-center keyboard || systemsettings kcm_keys || xfce4-keyboard-settings')))()
    case 'open-path':
      return shell.openPath(action.targetPath)
    case 'quit':
      return quit()
  }
}
