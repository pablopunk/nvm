import { constants } from 'node:fs';
import { validateBundleId } from './app-uninstall-bundle-id';
import type { PathInspector } from './app-uninstall-inspect';
import { sameIdentity } from './app-uninstall-inspect';
import { appSpecs, macPath } from './app-uninstall-paths';
import type {
  AppUninstallDependencies,
  AppUninstallSnapshot,
  Identity,
  Inspection,
  Refusal,
  SnapshotCandidate,
} from './app-uninstall-types';

export interface CheckedApp {
  canonical: string;
  identity: Identity;
  bundleId: string;
}

function normalizeAppPathForPlatform(platform: string, value: string) {
  return platform === 'darwin' ? value.toLowerCase() : value;
}

export function createAppGuards(
  deps: AppUninstallDependencies,
  inspector: PathInspector,
) {
  async function locateAppBundle(appPath: string): Promise<Inspection> {
    let located: Inspection = {
      code: 'outside-allowlist',
      message: 'App must be in /Applications or ~/Applications',
    };
    for (const spec of appSpecs(appPath, deps.homeDirectory)) {
      located = await inspector.inspectAllowed(spec);
      if ('canonical' in located) {
        break;
      }
    }
    return located;
  }

  async function isNevermindCopy(canonical: string) {
    if (!deps.nevermindAppPath) {
      return false;
    }
    const self = await inspector.inspectPath(deps.nevermindAppPath);
    return 'canonical' in self && self.canonical === canonical;
  }

  async function readTrustedBundleId(
    canonical: string,
  ): Promise<string | Refusal> {
    let bundleId: string | null;
    try {
      bundleId = validateBundleId(await deps.readBundleId(canonical));
    } catch {
      return {
        code: 'plist',
        message: 'This app’s metadata could not be read',
      };
    }
    if (!bundleId) {
      return {
        code: 'bundle-id',
        message: 'The app has no safe bundle identifier',
      };
    }
    return bundleId;
  }

  function isNevermindBundleId(bundleId: string) {
    return Boolean(
      deps.nevermindBundleId &&
        bundleId.toLowerCase() === deps.nevermindBundleId.toLowerCase(),
    );
  }

  async function isParentWritable(canonical: string) {
    try {
      await deps.access(macPath.dirname(canonical), constants.W_OK);
      return true;
    } catch {
      return false;
    }
  }

  function matchesExpectedApp(
    checked: CheckedApp,
    expected: AppUninstallSnapshot,
  ) {
    return (
      sameIdentity(checked.identity, expected.appIdentity) &&
      checked.canonical === expected.appPath &&
      checked.bundleId.toLowerCase() === expected.appBundleId.toLowerCase()
    );
  }

  async function checkApp(
    appPath: string,
    expected?: AppUninstallSnapshot,
  ): Promise<CheckedApp | Refusal> {
    const located = await locateAppBundle(appPath);
    if (!('canonical' in located)) {
      return located;
    }
    if (
      !located.canonical.endsWith('.app') ||
      located.identity.type !== 'directory'
    ) {
      return {
        code: 'invalid-app',
        message: 'Selected item is not an application bundle',
      };
    }
    if (await isNevermindCopy(located.canonical)) {
      return { code: 'self', message: 'Nevermind cannot uninstall itself' };
    }
    const bundleId = await readTrustedBundleId(located.canonical);
    if (typeof bundleId !== 'string') {
      return bundleId;
    }
    if (isNevermindBundleId(bundleId)) {
      return { code: 'self', message: 'Nevermind cannot uninstall itself' };
    }
    if (!(await isParentWritable(located.canonical))) {
      return {
        code: 'app-parent-not-writable',
        message: 'This app cannot be moved to Trash from its current folder',
      };
    }
    const checked = { ...located, bundleId };
    if (expected && !matchesExpectedApp(checked, expected)) {
      return {
        code: 'app-changed',
        message: 'The application changed after this uninstall view was opened',
      };
    }
    return checked;
  }

  async function isRunning(appPath: string) {
    try {
      const running = await deps.runningAppPaths(appPath);
      const expected = normalizeAppPathForPlatform(deps.platform, appPath);
      return Array.from(running).some(
        (candidate) =>
          normalizeAppPathForPlatform(deps.platform, candidate) === expected,
      );
    } catch {
      return true;
    }
  }

  async function revalidateCandidate(
    candidate: SnapshotCandidate,
  ): Promise<Inspection> {
    const checked = await inspector.inspectAllowed({
      slot: candidate.slot,
      path: candidate.path,
      root: candidate.root,
      kind: candidate.kind,
    });
    if (!('canonical' in checked)) {
      return checked;
    }
    if (
      checked.canonical !== candidate.path ||
      !sameIdentity(checked.identity, candidate.identity)
    ) {
      return { code: 'changed', message: 'Item changed after confirmation' };
    }
    if (
      candidate.kind === 'associated' &&
      checked.identity.uid !== deps.currentUid
    ) {
      return {
        code: 'owner',
        message: 'Item is no longer owned by the current user',
      };
    }
    return checked;
  }

  return { checkApp, isRunning, revalidateCandidate };
}

export type AppGuards = ReturnType<typeof createAppGuards>;
