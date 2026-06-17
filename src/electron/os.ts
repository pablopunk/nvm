import { execFile, spawn } from 'node:child_process';
import crypto from 'node:crypto';
import fsSync from 'node:fs';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import {
  app,
  type BrowserWindow,
  type BrowserWindowConstructorOptions,
  shell,
} from 'electron';

type OsDependent<T> = Partial<Record<'darwin' | 'linux' | 'win32', T>> & {
  default?: T;
};

function noop() {}

function osDependent<T>(handlers: OsDependent<T>, fallback: T): T {
  return (
    handlers[process.platform as 'darwin' | 'linux' | 'win32'] ??
    handlers.default ??
    fallback
  );
}

function osFunction<TArgs extends unknown[], TResult>(
  handlers: OsDependent<(...args: TArgs) => TResult>,
  fallback: (...args: TArgs) => TResult = noop as (...args: TArgs) => TResult,
) {
  return osDependent(handlers, fallback);
}

export function osLabel() {
  return osDependent(
    { darwin: 'macOS', win32: 'Windows', linux: 'Linux' },
    'Linux',
  );
}

const macOnlyCapabilities = new Set([
  'quick-look',
  'selected-files',
  'selected-text',
  'frontmost-app',
  'frontmost-paste',
  'keyboard.type-text',
  'applescript',
  'app-icons',
  'open-with',
  'keyboard-settings',
  'window-panel-policy',
  'file-date-added',
]);

export function hasCapability(capability: string) {
  if (capability === 'ocr')
    return osDependent({ darwin: fsSync.existsSync('/usr/bin/swift') }, false);
  if (capability === 'screen-capture')
    return osDependent(
      { darwin: fsSync.existsSync('/usr/sbin/screencapture') },
      false,
    );
  if (macOnlyCapabilities.has(capability))
    return osDependent({ darwin: true }, false);
  if (capability === 'auto-updates')
    return osDependent(
      { darwin: true, linux: Boolean(process.env.APPIMAGE) },
      false,
    );
  if (capability === 'camera')
    return osDependent({ darwin: true, win32: true, linux: true }, false);
  if (capability === 'launch-at-login')
    return osDependent({ darwin: true, win32: true }, false);
  return true;
}

export function canRequestMediaPermission(permission: string) {
  if (permission === 'media') return hasCapability('camera');
  return true;
}

export function settingsTitle() {
  return osDependent({ darwin: 'Open System Settings' }, 'Open Settings');
}

export function revealPathTitle() {
  return osDependent(
    { darwin: 'Show in Finder', win32: 'Show in File Explorer' },
    'Show in File Manager',
  );
}

export function revealPathDescription() {
  return osDependent(
    { darwin: 'Show in Finder', win32: 'Show in File Explorer' },
    'Show in the system file manager',
  );
}

export function quickLookTitle() {
  return osDependent({ darwin: 'Quick Look' }, 'Preview File');
}

export function quickLookDescription() {
  return osDependent({ darwin: 'Open Quick Look' }, 'Preview this file');
}

export function isReservedPaletteAccelerator(accelerator: string) {
  return osFunction(
    { darwin: () => accelerator === 'Command+Space' },
    () => false,
  )();
}

export function reservedPaletteShortcutName() {
  return osDependent({ darwin: 'Spotlight' }, 'the system');
}

export function keyboardSettingsSubtitle() {
  return osDependent(
    {
      darwin: 'System Settings → Keyboard → Keyboard Shortcuts',
      win32: 'Windows Settings',
    },
    'System keyboard shortcuts',
  );
}

export function keyboardShortcutConflictContent(label: string) {
  return osFunction(
    {
      darwin: () =>
        `# ${label} is used by Spotlight\n\nmacOS has \`${label}\` bound to Spotlight, so Nevermind cannot toggle with it until you disable that binding.\n\nOpen **System Settings → Keyboard → Keyboard Shortcuts → Spotlight** and uncheck *Show Spotlight search*.`,
    },
    () =>
      `# ${label} is used by the system\n\nThe current desktop has \`${label}\` reserved, so Nevermind cannot use it until that binding is changed.`,
  )();
}

export function paletteBrowserWindowOptions(): Partial<BrowserWindowConstructorOptions> {
  return osDependent<Partial<BrowserWindowConstructorOptions>>(
    { darwin: { type: 'panel' } },
    {},
  );
}

export function applyPaletteWindowPolicy(win: BrowserWindow | null) {
  return osFunction<[BrowserWindow | null], void>({
    darwin: (window) => {
      if (!window) return;
      window.setAlwaysOnTop(true, 'screen-saver');
      window.setVisibleOnAllWorkspaces(true, { visibleOnFullScreen: true });
      window.setWindowButtonVisibility?.(false);
      window.setFullScreenable(false);
      window.setSkipTaskbar(true);
    },
  })(win);
}

export function prepareAppWindowPolicy() {
  return osFunction({
    darwin: () => {
      app.setActivationPolicy('accessory');
      app.dock?.hide();
    },
  })();
}

export function getLaunchAtLoginEnabled() {
  if (!hasCapability('launch-at-login') || !app.isPackaged) return false;
  return app.getLoginItemSettings().openAtLogin;
}

export function setLaunchAtLoginEnabled(enabled: boolean) {
  if (!hasCapability('launch-at-login'))
    return {
      ok: false,
      message: `Start at login is not available on ${osLabel()}`,
    };
  if (!app.isPackaged)
    return {
      ok: false,
      message: 'Start at login is only available in packaged builds',
    };
  try {
    app.setLoginItemSettings(
      process.platform === 'darwin'
        ? { openAtLogin: enabled, openAsHidden: true }
        : { openAtLogin: enabled },
    );
    const current = app.getLoginItemSettings().openAtLogin;
    if (current !== enabled)
      return {
        ok: false,
        message: `Could not ${enabled ? 'enable' : 'disable'} start at login`,
      };
    return {
      ok: true,
      message: enabled
        ? 'Nevermind will start at login'
        : 'Nevermind will not start at login',
    };
  } catch (error) {
    return {
      ok: false,
      message:
        error instanceof Error
          ? error.message
          : `Could not ${enabled ? 'enable' : 'disable'} start at login`,
    };
  }
}

export function supportsAutoUpdates() {
  return hasCapability('auto-updates');
}

export function autoUpdatesUnavailableMessage() {
  return 'Automatic updates only run from packaged macOS builds or Linux AppImages';
}

export function appScanRoots() {
  return osFunction(
    {
      darwin: () => [
        '/Applications',
        '/System/Applications',
        '/System/Library/CoreServices/Applications',
        path.join(os.homedir(), 'Applications'),
      ],
      win32: () =>
        [
          process.env.ProgramData &&
            path.join(
              process.env.ProgramData,
              'Microsoft',
              'Windows',
              'Start Menu',
              'Programs',
            ),
          process.env.APPDATA &&
            path.join(
              process.env.APPDATA,
              'Microsoft',
              'Windows',
              'Start Menu',
              'Programs',
            ),
        ].filter(Boolean) as string[],
    },
    () => [
      '/usr/share/applications',
      '/usr/local/share/applications',
      path.join(os.homedir(), '.local/share/applications'),
    ],
  )();
}

export async function launchApp(item: any) {
  if (!item) return;
  return osFunction<[any], any>(
    {
      darwin: (appItem) =>
        spawn('open', [appItem.path], {
          detached: true,
          stdio: 'ignore',
        }).unref(),
      win32: (appItem) => shell.openPath(appItem.path),
    },
    (appItem) => {
      if (appItem.command)
        return spawn(appItem.command, {
          shell: true,
          detached: true,
          stdio: 'ignore',
        }).unref();
      return shell.openPath(appItem.path);
    },
  )(item);
}

const macSystemApps = ['/System/Library/CoreServices/Finder.app'];

async function scanMacApps() {
  const found: any[] = [];
  for (const appPath of macSystemApps) {
    if (fsSync.existsSync(appPath))
      found.push({
        id: appPath,
        name: path.basename(appPath).replace(/\.app$/i, ''),
        path: appPath,
      });
  }

  async function walk(dir: string, depth: number) {
    const entries = await fs
      .readdir(dir, { withFileTypes: true })
      .catch(() => []);
    await Promise.all(
      entries.map(async (entry) => {
        if (!entry.isDirectory() || entry.name.startsWith('.')) return;
        const fullPath = path.join(dir, entry.name);
        if (entry.name.endsWith('.app'))
          return found.push({
            id: fullPath,
            name: entry.name.replace(/\.app$/i, ''),
            path: fullPath,
          });
        if (depth > 0) await walk(fullPath, depth - 1);
      }),
    );
  }
  await Promise.all(appScanRoots().map((root) => walk(root, 2)));
  return found;
}

async function scanWindowsApps() {
  const found: any[] = [];
  async function walk(dir: string) {
    const entries = await fs
      .readdir(dir, { withFileTypes: true })
      .catch(() => []);
    await Promise.all(
      entries.map(async (entry) => {
        const fullPath = path.join(dir, entry.name);
        if (entry.isDirectory()) return walk(fullPath);
        if (entry.name.endsWith('.lnk'))
          found.push({
            id: fullPath,
            name: entry.name.replace(/\.lnk$/i, ''),
            path: fullPath,
          });
      }),
    );
  }
  await Promise.all(appScanRoots().map(walk));
  return found;
}

async function scanLinuxApps() {
  const found: any[] = [];
  await Promise.all(
    appScanRoots().map(async (root) => {
      const entries = await fs
        .readdir(root, { withFileTypes: true })
        .catch(() => []);
      await Promise.all(
        entries.map(async (entry) => {
          if (!entry.isFile() || !entry.name.endsWith('.desktop')) return;
          const fullPath = path.join(root, entry.name);
          const body = await fs.readFile(fullPath, 'utf8').catch(() => '');
          if (/^(NoDisplay|Hidden)=true$/im.test(body)) return;
          const name = body.match(/^Name=(.+)$/m)?.[1];
          const exec = body.match(/^Exec=(.+)$/m)?.[1];
          const wmClass = body.match(/^StartupWMClass=(.+)$/m)?.[1];
          if (!name || !exec) return;
          found.push({
            id: fullPath,
            name,
            path: fullPath,
            command: exec.replace(/\s*%[fFuUdDnNickvm]/g, '').trim(),
            wmClass,
          });
        }),
      );
    }),
  );
  return found;
}

export async function scanApps() {
  return osFunction(
    { darwin: scanMacApps, win32: scanWindowsApps },
    scanLinuxApps,
  )();
}

type RunningAppCandidate = {
  id?: string;
  name?: string;
  path?: string;
  command?: string;
  wmClass?: string;
};

function normalizedRunningPath(value: unknown) {
  const text = String(value || '').trim();
  return process.platform === 'darwin' || process.platform === 'win32'
    ? text.toLowerCase()
    : text;
}

async function macProcessExecutablePaths() {
  return new Promise<string[]>((resolve) => {
    execFile(
      'ps',
      ['-axo', 'comm='],
      { timeout: 1_000, maxBuffer: 1024 * 1024 },
      (error, stdout) => {
        if (error) return resolve([]);
        resolve(
          stdout
            .split(/\r?\n/)
            .map((item) => item.trim())
            .filter(Boolean),
        );
      },
    );
  });
}

function macExecutableBelongsToApp(executablePath: string, appPath: string) {
  const normalizedExecutable = normalizedRunningPath(executablePath);
  const normalizedAppPath = normalizedRunningPath(appPath).replace(/\/+$/, '');
  return normalizedExecutable.startsWith(`${normalizedAppPath}/contents/`);
}

async function runningMacAppPaths(apps: RunningAppCandidate[] = []) {
  const candidates = Array.from(
    new Set(
      apps
        .map((item) => item.path || item.id)
        .filter((item): item is string => Boolean(item?.endsWith('.app'))),
    ),
  );
  if (!candidates.length) return new Set<string>();
  const executables = await macProcessExecutablePaths();
  const running = new Set<string>();
  for (const appPath of candidates) {
    if (
      executables.some((executablePath) =>
        macExecutableBelongsToApp(executablePath, appPath),
      )
    )
      running.add(normalizedRunningPath(appPath));
  }
  return running;
}

function shellWords(command: string) {
  return (
    command
      .match(/"[^"]+"|'[^']+'|\S+/g)
      ?.map((token) => token.replace(/^['"]|['"]$/g, '')) || []
  );
}

function executableNameForLinuxCommand(command: unknown) {
  const words = shellWords(String(command || '').trim());
  while (
    words[0] &&
    (/^[A-Za-z_][A-Za-z0-9_]*=/.test(words[0]) ||
      path.basename(words[0]) === 'env')
  )
    words.shift();
  return words[0] ? path.basename(words[0]).toLowerCase() : '';
}

async function linuxProcessNames() {
  const names = new Set<string>();
  const entries = await fs
    .readdir('/proc', { withFileTypes: true })
    .catch(() => []);
  await Promise.all(
    entries.map(async (entry) => {
      if (!entry.isDirectory() || !/^\d+$/.test(entry.name)) return;
      const base = path.join('/proc', entry.name);
      const [comm, cmdline] = await Promise.all([
        fs.readFile(path.join(base, 'comm'), 'utf8').catch(() => ''),
        fs.readFile(path.join(base, 'cmdline'), 'utf8').catch(() => ''),
      ]);
      const commName = comm.trim();
      if (commName) names.add(commName.toLowerCase());
      const commandName = path.basename(cmdline.split('\0')[0] || '').trim();
      if (commandName) names.add(commandName.toLowerCase());
    }),
  );
  return names;
}

async function runningLinuxAppPaths(apps: RunningAppCandidate[] = []) {
  const processNames = await linuxProcessNames();
  const running = new Set<string>();
  for (const item of apps) {
    const candidates = [
      executableNameForLinuxCommand(item.command),
      String(item.wmClass || '').toLowerCase(),
    ].filter(Boolean);
    if (candidates.some((candidate) => processNames.has(candidate)))
      running.add(normalizedRunningPath(item.path || item.id));
  }
  return running;
}

export async function runningAppPaths(apps: RunningAppCandidate[] = []) {
  return osFunction(
    {
      darwin: () => runningMacAppPaths(apps),
      linux: () => runningLinuxAppPaths(apps),
    },
    async () => new Set<string>(),
  )();
}

export function watchApps(onChange: () => void) {
  const watchers: Array<{ close: () => unknown }> = [];
  for (const root of appScanRoots()) {
    if (!fsSync.existsSync(root)) continue;
    try {
      const watcher = fsSync.watch(
        root,
        { recursive: osDependent({ darwin: true, win32: true }, false) },
        onChange,
      );
      watcher.on('error', () => {});
      watchers.push(watcher);
    } catch {}
  }
  return watchers;
}

function runAppleScript(script: string, timeout = 30_000) {
  return new Promise<{ stdout: string; stderr: string; exitCode: number }>(
    (resolve) => {
      if (!hasCapability('applescript'))
        return resolve({
          stdout: '',
          stderr: 'AppleScript is not available on this OS',
          exitCode: 1,
        });
      execFile(
        'osascript',
        ['-e', script],
        { timeout },
        (error, stdout, stderr) =>
          resolve({
            stdout,
            stderr: stderr || error?.message || '',
            exitCode: error ? 1 : 0,
          }),
      );
    },
  );
}

function parseMetadataDate(value: string) {
  const text = value.trim();
  if (!text || text === '(null)') return 0;
  const match = text.match(
    /^(\d{4}-\d{2}-\d{2}) (\d{2}:\d{2}:\d{2}) ([+-]\d{2})(\d{2})$/,
  );
  const timestamp = Date.parse(
    match ? `${match[1]}T${match[2]}${match[3]}:${match[4]}` : text,
  );
  return Number.isFinite(timestamp) ? timestamp : 0;
}

function chunkArray<T>(items: T[], size: number) {
  const chunks: T[][] = [];
  for (let index = 0; index < items.length; index += size)
    chunks.push(items.slice(index, index + size));
  return chunks;
}

export async function fileDateAddedMs(paths: string[]) {
  if (!hasCapability('file-date-added') || paths.length === 0)
    return new Map<string, number>();
  const dates = new Map<string, number>();
  const chunks = chunkArray(paths, 100);
  let nextChunkIndex = 0;

  async function readNextChunk() {
    const chunk = chunks[nextChunkIndex++];
    if (!chunk) return;
    try {
      const output = await new Promise<string>((resolve, reject) => {
        execFile(
          'mdls',
          ['-raw', '-name', 'kMDItemDateAdded', ...chunk],
          { timeout: 2_000, maxBuffer: 1024 * 1024 },
          (error, stdout) => (error ? reject(error) : resolve(stdout)),
        );
      });
      const values = output
        .split('\0')
        .flatMap((part) => part.split('\n'))
        .map((part) => part.trim())
        .filter(Boolean);
      chunk.forEach((filePath, index) =>
        dates.set(filePath, parseMetadataDate(values[index] || '')),
      );
    } catch {
      chunk.forEach((filePath) => dates.set(filePath, 0));
    }
    await readNextChunk();
  }

  await Promise.all(
    Array.from({ length: Math.min(4, chunks.length) }, readNextChunk),
  );
  return dates;
}

export function pasteIntoFrontmostApp() {
  return osFunction({
    darwin: () =>
      execFile(
        'osascript',
        [
          '-e',
          'tell application "System Events" to keystroke "v" using command down',
        ],
        () => {},
      ),
  })();
}

function appleScriptString(value: string) {
  return `"${String(value).replace(/\\/g, '\\\\').replace(/"/g, '\\"').replace(/\r?\n/g, '" & return & "')}"`;
}

export function typeTextIntoFrontmostApp(text: string, options: any = {}) {
  return osFunction<[string, any], Promise<{ ok: boolean; error?: string }>>(
    {
      darwin: (value, opts) =>
        new Promise((resolve) => {
          const delayMs = Math.max(0, Number(opts?.delayMs || 0));
          const delaySeconds = delayMs / 1000;
          const script =
            delayMs > 0
              ? `tell application "System Events"\n${Array.from(String(value))
                  .map(
                    (char) =>
                      `keystroke ${appleScriptString(char)}\ndelay ${delaySeconds}`,
                  )
                  .join('\n')}\nend tell`
              : `tell application "System Events" to keystroke ${appleScriptString(value)}`;
          execFile(
            'osascript',
            ['-e', script],
            { timeout: Math.max(5_000, Number(opts?.timeoutMs || 30_000)) },
            (error, _stdout, stderr) => {
              resolve(
                error
                  ? {
                      ok: false,
                      error: String(
                        stderr || error.message || 'Unable to type text',
                      ),
                    }
                  : { ok: true },
              );
            },
          );
        }),
    },
    async () => ({
      ok: false,
      error: `${osLabel()} does not support keyboard text injection`,
    }),
  )(text, options);
}

export async function selectedFilePaths() {
  return osFunction(
    {
      darwin: async () => {
        const script =
          'set AppleScript\'s text item delimiters to linefeed\ntell application "Finder"\ntry\nset selectedItems to selection as alias list\non error\nreturn ""\nend try\nset selectedPaths to {}\nrepeat with selectedItem in selectedItems\nset end of selectedPaths to POSIX path of (selectedItem as alias)\nend repeat\nreturn selectedPaths as text\nend tell';
        const result = await runAppleScript(script);
        if (result.exitCode !== 0) return [];
        return result.stdout
          .split(/\r?\n/)
          .map((item) => item.trim())
          .filter(Boolean);
      },
    },
    async () => [],
  )();
}

export async function selectedText() {
  return osFunction(
    {
      darwin: async () => {
        const script =
          'tell application "System Events"\nset frontProcess to first application process whose frontmost is true\ntry\nset selectedText to value of attribute "AXSelectedText" of focused UI element of frontProcess\nif selectedText is missing value then return ""\nreturn selectedText as text\non error\nreturn ""\nend try\nend tell';
        const result = await runAppleScript(script, 5_000);
        const text = result.stdout.trim();
        return text || null;
      },
    },
    async () => null,
  )();
}

export async function frontmostApp() {
  return osFunction(
    {
      darwin: async () => {
        const script =
          'tell application "System Events"\nset frontProcess to first application process whose frontmost is true\nset appName to name of frontProcess\nset appBundle to bundle identifier of frontProcess\ntry\nset appPath to POSIX path of (file of frontProcess as alias)\non error\nset appPath to ""\nend try\nreturn appName & linefeed & appBundle & linefeed & appPath\nend tell';
        const result = await runAppleScript(script, 5_000);
        if (result.exitCode !== 0) return null;
        const [name, bundleId, appPath] = result.stdout.split(/\r?\n/);
        return name
          ? { name, bundleId: bundleId || null, path: appPath || null }
          : null;
      },
    },
    async () => null,
  )();
}

function detached(command: string, args: string[] = []) {
  spawn(command, args, { detached: true, stdio: 'ignore' }).unref();
}

function detachedShell(script: string) {
  detached('sh', ['-lc', script]);
}

const OCR_SWIFT_HELPER = `
import Foundation
import Vision

struct OcrBox: Codable {
  let x: Double
  let y: Double
  let width: Double
  let height: Double
}

struct OcrBlock: Codable {
  let text: String
  let confidence: Float
  let boundingBox: OcrBox
}

struct OcrResult: Codable {
  let text: String
  let confidence: Float?
  let language: String?
  let blocks: [OcrBlock]
}

func fail(_ message: String, _ code: Int32 = 1) -> Never {
  FileHandle.standardError.write(Data(message.utf8))
  exit(code)
}

let arguments = CommandLine.arguments
if arguments.count < 2 { fail("Missing image path") }
let imagePath = arguments[1]
let languages = Array(arguments.dropFirst(2)).filter { !$0.isEmpty }
let imageUrl = URL(fileURLWithPath: imagePath)

let request = VNRecognizeTextRequest()
request.recognitionLevel = .accurate
request.usesLanguageCorrection = true
if !languages.isEmpty { request.recognitionLanguages = languages }

let handler = VNImageRequestHandler(url: imageUrl, options: [:])
do {
  try handler.perform([request])
  let observations = request.results ?? []
  let blocks: [OcrBlock] = observations.compactMap { observation in
    guard let candidate = observation.topCandidates(1).first else { return nil }
    let box = observation.boundingBox
    return OcrBlock(
      text: candidate.string,
      confidence: candidate.confidence,
      boundingBox: OcrBox(x: box.origin.x, y: box.origin.y, width: box.size.width, height: box.size.height)
    )
  }
  let text = blocks.map { $0.text }.joined(separator: "\\n")
  let confidence = blocks.isEmpty ? nil : blocks.map { $0.confidence }.reduce(0, +) / Float(blocks.count)
  let result = OcrResult(text: text, confidence: confidence, language: languages.first, blocks: blocks)
  let data = try JSONEncoder().encode(result)
  FileHandle.standardOutput.write(data)
} catch {
  fail(error.localizedDescription)
}
`;

async function writeOcrSwiftHelper() {
  const helperPath = path.join(os.tmpdir(), 'nevermind-ocr-helper.swift');
  await fs.writeFile(helperPath, OCR_SWIFT_HELPER, 'utf8');
  return helperPath;
}

function execFileJson(command: string, args: string[], options: any = {}) {
  return new Promise<any>((resolve, reject) => {
    execFile(
      command,
      args,
      {
        timeout: options.timeout || 30_000,
        maxBuffer: options.maxBuffer || 2 * 1024 * 1024,
      },
      (error, stdout, stderr) => {
        if (error)
          return reject(
            new Error(
              String(stderr || error.message || 'Command failed').trim(),
            ),
          );
        try {
          resolve(JSON.parse(stdout || '{}'));
        } catch {
          reject(new Error('OCR returned invalid JSON'));
        }
      },
    );
  });
}

async function recognizeTextInImageMac(filePath: string, options: any = {}) {
  const helperPath = await writeOcrSwiftHelper();
  const languages = Array.isArray(options.languages)
    ? options.languages.map(String)
    : options.language
      ? [String(options.language)]
      : [];
  return execFileJson('/usr/bin/swift', [helperPath, filePath, ...languages], {
    timeout: Math.max(5_000, Number(options.timeoutMs || 30_000)),
  });
}

export async function recognizeTextInImage(
  filePath: string,
  options: any = {},
) {
  return osFunction<[string, any], Promise<any>>(
    {
      darwin: recognizeTextInImageMac,
    },
    async () => {
      throw new Error(`OCR is not available on ${osLabel()}`);
    },
  )(filePath, options);
}

function screenshotPath() {
  return path.join(os.tmpdir(), `nevermind-ocr-${crypto.randomUUID()}.png`);
}

async function captureScreenImageMac(options: any = {}) {
  const outputPath = screenshotPath();
  const rect = options.region || options.rect;
  const args = ['-x'];
  if (rect && Number(rect.width) > 0 && Number(rect.height) > 0)
    args.push(
      '-R',
      [rect.x || 0, rect.y || 0, rect.width, rect.height]
        .map((value) => Math.round(Number(value)))
        .join(','),
    );
  args.push(outputPath);
  await new Promise<void>((resolve, reject) => {
    execFile(
      '/usr/sbin/screencapture',
      args,
      { timeout: Math.max(5_000, Number(options.timeoutMs || 15_000)) },
      (error, _stdout, stderr) =>
        error
          ? reject(
              new Error(
                String(
                  stderr || error.message || 'Screen capture failed',
                ).trim(),
              ),
            )
          : resolve(),
    );
  });
  return outputPath;
}

export async function captureScreenImage(options: any = {}) {
  return osFunction<[any], Promise<string>>(
    {
      darwin: captureScreenImageMac,
    },
    async () => {
      throw new Error(`Screen capture is not available on ${osLabel()}`);
    },
  )(options);
}

export async function executeSystemBuiltin(action: any, quit: () => void) {
  switch (action.builtin) {
    case 'lock-screen':
      return osFunction(
        {
          darwin: () =>
            detached(
              '/System/Library/CoreServices/Menu Extras/User.menu/Contents/Resources/CGSession',
              ['-suspend'],
            ),
          win32: () => detached('rundll32.exe', ['user32.dll,LockWorkStation']),
        },
        () =>
          detachedShell(
            'loginctl lock-session || xdg-screensaver lock || gnome-screensaver-command -l',
          ),
      )();
    case 'sleep':
      return osFunction(
        {
          darwin: () => detached('pmset', ['sleepnow']),
          win32: () =>
            detached('rundll32.exe', ['powrprof.dll,SetSuspendState', '0,1,0']),
        },
        () => detached('systemctl', ['suspend']),
      )();
    case 'restart':
      return osFunction(
        {
          darwin: () =>
            detached('osascript', [
              '-e',
              'tell application "System Events" to restart',
            ]),
          win32: () => detached('shutdown', ['/r', '/t', '0']),
        },
        () => detached('shutdown', ['-r', 'now']),
      )();
    case 'settings':
      return osFunction(
        {
          darwin: () => shell.openExternal('x-apple.systempreferences:'),
          win32: () => shell.openExternal('ms-settings:'),
        },
        () =>
          Promise.resolve(
            detachedShell(
              'gnome-control-center || systemsettings || xfce4-settings-manager',
            ),
          ),
      )();
    case 'open-keyboard-settings':
      return osFunction(
        {
          darwin: () =>
            shell.openExternal(
              'x-apple.systempreferences:com.apple.Keyboard-Settings.extension',
            ),
          win32: () => shell.openExternal('ms-settings:typing'),
        },
        () =>
          Promise.resolve(
            detachedShell(
              'gnome-control-center keyboard || systemsettings kcm_keys || xfce4-keyboard-settings',
            ),
          ),
      )();
    case 'open-path':
      return shell.openPath(action.targetPath);
    case 'quit':
      return quit();
  }
}
