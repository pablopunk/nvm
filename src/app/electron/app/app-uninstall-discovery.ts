import type { AppGuards, CheckedApp } from './app-uninstall-guards';
import type { PathInspector } from './app-uninstall-inspect';
import type { OwnershipVerdict } from './app-uninstall-ownership';
import {
  applicationRootFor,
  exactAssociatedSpecs,
  MAX_PATTERN_MATCHES,
  macPath,
  type PatternMatch,
  patternMatchFor,
  patternRoots,
} from './app-uninstall-paths';
import type {
  AppUninstallDependencies,
  AppUninstallNote,
  AppUninstallSnapshot,
  CandidateSpec,
  DiscoveryResult,
  SnapshotCandidate,
} from './app-uninstall-types';

const REASONS = {
  application: 'Application bundle',
  runningApplication:
    'Running. It will be quit gracefully before anything is moved to Trash.',
  exactData: 'Exact bundle ID data',
  unverifiedData: 'Other installed apps could not be ruled out; left unchecked',
  prefixMatch: 'Bundle ID prefix match; may belong to another app',
  nameMatch: 'Name match; may belong to another app',
};

function exactDataReason(verdict: OwnershipVerdict) {
  if (verdict.status === 'confirmed') {
    return REASONS.exactData;
  }
  if (verdict.status === 'shared') {
    return `May be shared with ${verdict.owner}; left unchecked`;
  }
  return REASONS.unverifiedData;
}

function applicationReason(appRunning: boolean) {
  return appRunning ? REASONS.runningApplication : REASONS.application;
}

interface DiscoveryInputs {
  deps: AppUninstallDependencies;
  randomId: () => string;
  inspector: PathInspector;
  guards: AppGuards;
  verifyOwnership: (
    bundleId: string,
    appCanonicalPath: string,
  ) => Promise<OwnershipVerdict>;
}

function missingLocationsNote(missing: number): AppUninstallNote[] {
  if (missing === 0) {
    return [];
  }
  return [
    {
      code: 'missing-associated',
      message: `${missing} conventional associated location${missing === 1 ? ' was' : 's were'} not present.`,
    },
  ];
}

export function createDiscovery({
  deps,
  randomId,
  inspector,
  guards,
  verifyOwnership,
}: DiscoveryInputs) {
  function candidateFrom(
    fields: Omit<SnapshotCandidate, 'id'>,
  ): SnapshotCandidate {
    return { id: randomId(), ...fields };
  }

  async function collectExactCandidates(
    app: CheckedApp,
    verdict: OwnershipVerdict,
  ) {
    const specs = exactAssociatedSpecs(deps.homeDirectory, app.bundleId);
    const checks = await Promise.all(
      specs.map(async (spec) => ({
        spec,
        checked: await inspector.inspectAllowed(spec),
      })),
    );
    const candidates: SnapshotCandidate[] = [];
    const notes: AppUninstallNote[] = [];
    let missing = 0;
    for (const { spec, checked } of checks) {
      if (!('canonical' in checked)) {
        if (checked.code === 'missing') {
          missing += 1;
        } else {
          notes.push({
            code: `${spec.slot}-${checked.code}`,
            message: `${spec.slot}: ${checked.message}`,
          });
        }
        continue;
      }
      if (checked.identity.uid !== deps.currentUid) {
        notes.push({
          code: `${spec.slot}-owner`,
          message: `${spec.slot}: item is not owned by the current user`,
        });
        continue;
      }
      if (candidates.some((item) => item.path === checked.canonical)) {
        continue;
      }
      candidates.push(
        candidateFrom({
          path: checked.canonical,
          kind: 'associated',
          slot: spec.slot,
          root: spec.root,
          identity: checked.identity,
          selectedByDefault: verdict.status === 'confirmed',
          reason: exactDataReason(verdict),
        }),
      );
    }
    return { candidates, notes: [...missingLocationsNote(missing), ...notes] };
  }

  async function listEntryNames(root: string) {
    try {
      return { names: await deps.listDirectory(root), inaccessible: false };
    } catch (error) {
      return {
        names: [],
        inaccessible: !(
          error &&
          typeof error === 'object' &&
          'code' in error &&
          error.code === 'ENOENT'
        ),
      };
    }
  }

  async function collectPatternCandidates(
    app: CheckedApp,
    knownPaths: string[],
  ) {
    const appName = macPath.basename(app.canonical, '.app');
    const listings = await Promise.all(
      patternRoots(deps.homeDirectory).map(async (root) => ({
        root,
        ...(await listEntryNames(root.root)),
      })),
    );
    const matches: Array<{ spec: CandidateSpec; match: PatternMatch }> = [];
    for (const { root, names } of listings) {
      for (const name of names) {
        const match = patternMatchFor(
          name,
          app.bundleId,
          appName,
          root.allowsNameMatch,
        );
        const spec: CandidateSpec = {
          slot: root.slot,
          path: macPath.join(root.root, name),
          root: root.root,
          kind: 'associated',
        };
        if (match && !knownPaths.includes(spec.path)) {
          matches.push({ spec, match });
        }
      }
    }
    const bounded = matches.slice(0, MAX_PATTERN_MATCHES);
    const checks = await Promise.all(
      bounded.map(async ({ spec, match }) => ({
        spec,
        match,
        checked: await inspector.inspectAllowed(spec),
      })),
    );
    const candidates = checks.flatMap(({ spec, match, checked }) => {
      if (
        !('canonical' in checked) ||
        checked.identity.uid !== deps.currentUid
      ) {
        return [];
      }
      return [
        candidateFrom({
          path: checked.canonical,
          kind: 'associated',
          slot: spec.slot,
          root: spec.root,
          identity: checked.identity,
          selectedByDefault: false,
          reason: match === 'prefix' ? REASONS.prefixMatch : REASONS.nameMatch,
        }),
      ];
    });
    const limitNotes: AppUninstallNote[] =
      matches.length > MAX_PATTERN_MATCHES
        ? [
            {
              code: 'pattern-limit',
              message: `Only the first ${MAX_PATTERN_MATCHES} possible matches are listed.`,
            },
          ]
        : [];
    if (listings.some((listing) => listing.inaccessible)) {
      limitNotes.push({
        code: 'incomplete-pattern-scan',
        message:
          'Some locations could not be scanned; related files may be missing.',
      });
    }
    return { candidates, notes: limitNotes };
  }

  async function discover(appPath: string): Promise<DiscoveryResult> {
    if (deps.platform !== 'darwin') {
      return {
        status: 'unavailable',
        reasonCode: 'unsupported-platform',
        message: 'Uninstall is available on macOS only',
      };
    }
    const app = await guards.checkApp(appPath);
    if (!('canonical' in app)) {
      return {
        status: 'unavailable',
        reasonCode: app.code,
        message: app.message,
      };
    }
    const [appRunning, verdict] = await Promise.all([
      guards.isRunning(app.canonical),
      verifyOwnership(app.bundleId, app.canonical),
    ]);
    const exact = await collectExactCandidates(app, verdict);
    const knownPaths = [
      app.canonical,
      ...exact.candidates.map((candidate) => candidate.path),
    ];
    const patterns = await collectPatternCandidates(app, knownPaths);
    const applicationCandidate = candidateFrom({
      path: app.canonical,
      kind: 'app',
      slot: 'app',
      root: applicationRootFor(app.canonical, deps.homeDirectory),
      identity: app.identity,
      selectedByDefault: true,
      reason: applicationReason(appRunning),
    });
    const candidates = [
      applicationCandidate,
      ...exact.candidates,
      ...patterns.candidates,
    ];
    const snapshot: AppUninstallSnapshot = {
      appPath: app.canonical,
      appBundleId: app.bundleId,
      appIdentity: app.identity,
      candidates,
    };
    return {
      status: 'ready',
      snapshot,
      candidates,
      notes: [...exact.notes, ...patterns.notes],
      appRunning,
    };
  }

  return { discover };
}
