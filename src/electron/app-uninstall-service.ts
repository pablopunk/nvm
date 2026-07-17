import { execFile } from 'node:child_process';
import { randomUUID } from 'node:crypto';
import { constants } from 'node:fs';
import fs from 'node:fs/promises';
import path from 'node:path';
import { promisify } from 'node:util';

const execFileAsync = promisify(execFile);
const PLUTIL = '/usr/bin/plutil';
const MAX_BUNDLE_ID_BYTES = 255;

type Stat = {
  dev: number;
  ino: number;
  uid: number;
  isDirectory: () => boolean;
  isSymbolicLink: () => boolean;
};

type Identity = Pick<Stat, 'dev' | 'ino' | 'uid'> & {
  type: 'directory' | 'file';
};

export type AppUninstallCandidate = {
  id: string;
  path: string;
  kind: 'app' | 'associated';
  slot: string;
};

type SnapshotCandidate = AppUninstallCandidate & {
  identity: Identity;
  root: string;
};

export type AppUninstallSnapshot = {
  appPath: string;
  appBundleId: string;
  appIdentity: Identity;
  candidates: SnapshotCandidate[];
};

export type AppUninstallNote = { code: string; message: string };

export type DiscoveryResult =
  | { status: 'unavailable'; reasonCode: string; message: string }
  | {
      status: 'ready';
      snapshot: AppUninstallSnapshot;
      candidates: AppUninstallCandidate[];
      notes: AppUninstallNote[];
    };

export type TrashResult = {
  status: 'complete' | 'partial' | 'failed';
  moved: string[];
  untouched: Array<{ path: string; code: string; message: string }>;
  notes: AppUninstallNote[];
};

export type AppUninstallDependencies = {
  platform: string;
  homeDirectory: string;
  currentUid: number;
  lstat: (value: string) => Promise<Stat>;
  realpath: (value: string) => Promise<string>;
  access: (value: string, mode: number) => Promise<void>;
  readBundleId: (appPath: string) => Promise<unknown>;
  trashItem: (value: string) => Promise<void>;
  nevermindAppPath?: string | null;
  nevermindBundleId?: string | null;
  runningAppPaths: (appPath: string) => Promise<Set<string>>;
  randomId?: () => string;
};

type CandidateSpec = {
  slot: string;
  path: string;
  root: string;
  kind: 'app' | 'associated';
};

function isMissing(error: unknown) {
  return Boolean(
    error &&
      typeof error === 'object' &&
      (error as { code?: string }).code === 'ENOENT',
  );
}

function safeMessage(error: unknown) {
  return error instanceof Error
    ? error.message
    : String(error || 'unavailable');
}

function statType(stat: Stat): Identity['type'] | null {
  if (stat.isSymbolicLink()) return null;
  return stat.isDirectory() ? 'directory' : 'file';
}

function identity(stat: Stat): Identity | null {
  const type = statType(stat);
  return type ? { type, dev: stat.dev, ino: stat.ino, uid: stat.uid } : null;
}

function sameIdentity(left: Identity, right: Identity) {
  return (
    left.type === right.type && left.dev === right.dev && left.ino === right.ino
  );
}

function isWithin(root: string, target: string) {
  const relative = path.relative(root, target);
  return (
    relative === '' ||
    (!path.isAbsolute(relative) &&
      relative !== '..' &&
      !relative.startsWith(`..${path.sep}`))
  );
}

function normalizeBundleId(value: string) {
  return value.toLowerCase();
}

export function validateBundleId(value: unknown): string | null {
  if (
    typeof value !== 'string' ||
    Buffer.byteLength(value, 'utf8') > MAX_BUNDLE_ID_BYTES
  )
    return null;
  if (!value || /[\x00-\x20\x7f/\\]/.test(value)) return null;
  const components = value.split('.');
  if (
    !components.length ||
    components.some((part) => !/^[A-Za-z0-9-]+$/.test(part))
  )
    return null;
  return value;
}

async function productionPlistReader(appPath: string): Promise<unknown> {
  const result = await execFileAsync(
    PLUTIL,
    [
      '-extract',
      'CFBundleIdentifier',
      'raw',
      '-o',
      '-',
      path.join(appPath, 'Contents', 'Info.plist'),
    ],
    { shell: false, timeout: 5_000, maxBuffer: 4_096 },
  );
  return String(result.stdout).trim();
}

export function createProductionAppUninstallService(
  input: Omit<
    Partial<AppUninstallDependencies>,
    'lstat' | 'realpath' | 'access' | 'readBundleId' | 'randomId'
  > &
    Pick<AppUninstallDependencies, 'trashItem' | 'runningAppPaths'>,
) {
  return createAppUninstallService({
    platform: input.platform || process.platform,
    homeDirectory: input.homeDirectory || process.env.HOME || '',
    currentUid: input.currentUid ?? process.getuid?.() ?? -1,
    lstat: (value) => fs.lstat(value) as Promise<Stat>,
    realpath: (value) => fs.realpath(value),
    access: (value, mode) => fs.access(value, mode),
    readBundleId: productionPlistReader,
    trashItem: input.trashItem,
    nevermindAppPath: input.nevermindAppPath,
    nevermindBundleId: input.nevermindBundleId,
    runningAppPaths: input.runningAppPaths,
  });
}

export function createAppUninstallService(deps: AppUninstallDependencies) {
  const randomId = deps.randomId || randomUUID;

  async function inspectPath(
    value: string,
  ): Promise<
    | { canonical: string; identity: Identity }
    | { code: string; message: string }
  > {
    const parsed = path.parse(path.resolve(value));
    let current = parsed.root;
    const components = path
      .relative(parsed.root, path.resolve(value))
      .split(path.sep)
      .filter(Boolean);
    for (const component of components) {
      current = path.join(current, component);
      let stat: Stat;
      try {
        stat = await deps.lstat(current);
      } catch (error) {
        return {
          code: isMissing(error) ? 'missing' : 'inaccessible',
          message: safeMessage(error),
        };
      }
      if (stat.isSymbolicLink())
        return {
          code: 'symlink',
          message: `Refusing symbolic-link path component: ${current}`,
        };
    }
    try {
      const stat = await deps.lstat(value);
      const itemIdentity = identity(stat);
      if (!itemIdentity)
        return { code: 'symlink', message: `Refusing symbolic link: ${value}` };
      return { canonical: await deps.realpath(value), identity: itemIdentity };
    } catch (error) {
      return {
        code: isMissing(error) ? 'missing' : 'inaccessible',
        message: safeMessage(error),
      };
    }
  }

  async function inspectAllowed(
    spec: CandidateSpec,
  ): Promise<
    | { canonical: string; identity: Identity }
    | { code: string; message: string }
  > {
    const [root, target] = await Promise.all([
      inspectPath(spec.root),
      inspectPath(spec.path),
    ]);
    if (!('canonical' in root)) return root;
    if (!('canonical' in target)) return target;
    if (!isWithin(root.canonical, target.canonical))
      return {
        code: 'outside-allowlist',
        message: `Refusing path outside ${spec.root}`,
      };
    return target;
  }

  function appSpecs(appPath: string): CandidateSpec[] {
    const homeApps = path.join(deps.homeDirectory, 'Applications');
    return [
      { slot: 'app', path: appPath, root: '/Applications', kind: 'app' },
      { slot: 'app', path: appPath, root: homeApps, kind: 'app' },
    ];
  }

  function associatedSpecs(bundleId: string): CandidateSpec[] {
    const library = path.join(deps.homeDirectory, 'Library');
    return [
      [
        'application-support',
        path.join(library, 'Application Support', bundleId),
        path.join(library, 'Application Support'),
      ],
      [
        'caches',
        path.join(library, 'Caches', bundleId),
        path.join(library, 'Caches'),
      ],
      [
        'preferences',
        path.join(library, 'Preferences', `${bundleId}.plist`),
        path.join(library, 'Preferences'),
      ],
      [
        'saved-state',
        path.join(library, 'Saved Application State', `${bundleId}.savedState`),
        path.join(library, 'Saved Application State'),
      ],
      [
        'containers',
        path.join(library, 'Containers', bundleId),
        path.join(library, 'Containers'),
      ],
      [
        'application-scripts',
        path.join(library, 'Application Scripts', bundleId),
        path.join(library, 'Application Scripts'),
      ],
      [
        'http-storages',
        path.join(library, 'HTTPStorages', bundleId),
        path.join(library, 'HTTPStorages'),
      ],
      [
        'webkit',
        path.join(library, 'WebKit', bundleId),
        path.join(library, 'WebKit'),
      ],
      [
        'cookies',
        path.join(library, 'Cookies', `${bundleId}.binarycookies`),
        path.join(library, 'Cookies'),
      ],
    ].map(([slot, candidatePath, root]) => ({
      slot,
      path: candidatePath,
      root,
      kind: 'associated' as const,
    }));
  }

  async function checkApp(appPath: string, expected?: AppUninstallSnapshot) {
    let checked:
      | { canonical: string; identity: Identity }
      | { code: string; message: string } = {
      code: 'outside-allowlist',
      message: 'App must be in /Applications or ~/Applications',
    };
    for (const candidate of appSpecs(appPath)) {
      checked = await inspectAllowed(candidate);
      if ('canonical' in checked) break;
    }
    if (!('canonical' in checked)) return checked;
    if (
      !checked.canonical.endsWith('.app') ||
      checked.identity.type !== 'directory'
    )
      return {
        code: 'invalid-app',
        message: 'Selected item is not an application bundle',
      };
    if (deps.nevermindAppPath) {
      const self = await inspectPath(deps.nevermindAppPath);
      if ('canonical' in self && self.canonical === checked.canonical)
        return { code: 'self', message: 'Nevermind cannot uninstall itself' };
    }
    let bundleId: string | null;
    try {
      bundleId = validateBundleId(await deps.readBundleId(checked.canonical));
    } catch (error) {
      return { code: 'plist', message: safeMessage(error) };
    }
    if (!bundleId)
      return {
        code: 'bundle-id',
        message: 'The app has no safe bundle identifier',
      };
    if (
      deps.nevermindBundleId &&
      normalizeBundleId(bundleId) === normalizeBundleId(deps.nevermindBundleId)
    )
      return { code: 'self', message: 'Nevermind cannot uninstall itself' };
    try {
      await deps.access(path.dirname(checked.canonical), constants.W_OK);
    } catch (error) {
      return { code: 'app-parent-not-writable', message: safeMessage(error) };
    }
    if (
      expected &&
      (!sameIdentity(checked.identity, expected.appIdentity) ||
        checked.canonical !== expected.appPath ||
        normalizeBundleId(bundleId) !== normalizeBundleId(expected.appBundleId))
    )
      return {
        code: 'app-changed',
        message: 'The application changed after this uninstall view was opened',
      };
    return { ...checked, bundleId };
  }

  async function checkNotRunning(appPath: string) {
    const running = await deps.runningAppPaths(appPath);
    const expected =
      deps.platform === 'darwin' ? appPath.toLowerCase() : appPath;
    if (
      Array.from(running).some(
        (candidate) =>
          (deps.platform === 'darwin' ? candidate.toLowerCase() : candidate) ===
          expected,
      )
    )
      return {
        code: 'running',
        message: 'Quit this application before uninstalling it',
      };
    return null;
  }

  async function discover(appPath: string): Promise<DiscoveryResult> {
    if (deps.platform !== 'darwin')
      return {
        status: 'unavailable',
        reasonCode: 'unsupported-platform',
        message: 'Uninstall is available on macOS only',
      };
    const app = await checkApp(appPath);
    if (!('canonical' in app))
      return {
        status: 'unavailable',
        reasonCode: app.code,
        message: app.message,
      };
    const running = await checkNotRunning(app.canonical);
    if (running)
      return {
        status: 'unavailable',
        reasonCode: running.code,
        message: running.message,
      };
    const appRoot = isWithin('/Applications', app.canonical)
      ? '/Applications'
      : path.join(deps.homeDirectory, 'Applications');
    const candidates: SnapshotCandidate[] = [
      {
        id: randomId(),
        path: app.canonical,
        kind: 'app',
        slot: 'app',
        root: appRoot,
        identity: app.identity,
      },
    ];
    const notes: AppUninstallNote[] = [];
    let missing = 0;
    for (const spec of associatedSpecs(app.bundleId)) {
      const checked = await inspectAllowed(spec);
      if (!('canonical' in checked)) {
        if (checked.code === 'missing') missing += 1;
        else
          notes.push({
            code: `${spec.slot}-${checked.code}`,
            message: `${spec.slot}: ${checked.message}`,
          });
        continue;
      }
      if (checked.identity.uid !== deps.currentUid) {
        notes.push({
          code: `${spec.slot}-owner`,
          message: `${spec.slot}: item is not owned by the current user`,
        });
        continue;
      }
      if (candidates.some((candidate) => candidate.path === checked.canonical))
        continue;
      candidates.push({
        id: randomId(),
        path: checked.canonical,
        kind: 'associated',
        slot: spec.slot,
        root: spec.root,
        identity: checked.identity,
      });
    }
    if (missing)
      notes.unshift({
        code: 'missing-associated',
        message: `${missing} conventional associated location${missing === 1 ? ' was' : 's were'} not present.`,
      });
    const snapshot: AppUninstallSnapshot = {
      appPath: app.canonical,
      appBundleId: app.bundleId,
      appIdentity: app.identity,
      candidates,
    };
    return { status: 'ready', snapshot, candidates, notes };
  }

  function selected(
    snapshot: AppUninstallSnapshot,
    values: Record<string, unknown> = {},
  ) {
    return snapshot.candidates.filter(
      (candidate) => values[candidate.id] === true,
    );
  }

  async function revalidateCandidate(
    snapshot: AppUninstallSnapshot,
    candidate: SnapshotCandidate,
  ) {
    const checked = await inspectAllowed({
      slot: candidate.slot,
      path: candidate.path,
      root: candidate.root,
      kind: candidate.kind,
    });
    if (!('canonical' in checked)) return checked;
    if (
      checked.canonical !== candidate.path ||
      !sameIdentity(checked.identity, candidate.identity)
    )
      return { code: 'changed', message: 'Item changed after confirmation' };
    if (
      candidate.kind === 'associated' &&
      checked.identity.uid !== deps.currentUid
    )
      return {
        code: 'owner',
        message: 'Item is no longer owned by the current user',
      };
    return checked;
  }

  async function trash(
    snapshot: AppUninstallSnapshot,
    values: Record<string, unknown> = {},
  ): Promise<TrashResult> {
    const selection = selected(snapshot, values);
    if (!selection.length)
      return {
        status: 'failed',
        moved: [],
        untouched: [],
        notes: [
          {
            code: 'zero-selection',
            message: 'Select at least one item to move to Trash.',
          },
        ],
      };
    const ordered = [
      ...selection.filter((item) => item.kind !== 'app'),
      ...selection.filter((item) => item.kind === 'app'),
    ];
    const moved: string[] = [];
    const untouched: TrashResult['untouched'] = [];
    const globalFailure = async () => {
      const app = await checkApp(snapshot.appPath, snapshot);
      if (!('canonical' in app)) return app;
      return checkNotRunning(app.canonical);
    };
    const initial = await globalFailure();
    if (initial)
      return {
        status: 'failed',
        moved,
        untouched: ordered.map((item) => ({
          path: item.path,
          code: initial.code,
          message: initial.message,
        })),
        notes: [],
      };
    for (let index = 0; index < ordered.length; index += 1) {
      const candidate = ordered[index];
      const global = await globalFailure();
      if (global) {
        untouched.push(
          ...ordered.slice(index).map((item) => ({
            path: item.path,
            code: global.code,
            message: global.message,
          })),
        );
        break;
      }
      const checked = await revalidateCandidate(snapshot, candidate);
      if (!('canonical' in checked)) {
        untouched.push({
          path: candidate.path,
          code: checked.code,
          message: checked.message,
        });
        continue;
      }
      try {
        await deps.trashItem(candidate.path);
        moved.push(candidate.path);
      } catch (error) {
        untouched.push({
          path: candidate.path,
          code: 'trash-failed',
          message: safeMessage(error),
        });
      }
    }
    return {
      status: untouched.length
        ? moved.length
          ? 'partial'
          : 'failed'
        : 'complete',
      moved,
      untouched,
      notes: [],
    };
  }

  return { discover, selected, trash };
}
