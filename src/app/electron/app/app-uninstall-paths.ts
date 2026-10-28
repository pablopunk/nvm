import path from 'node:path';
import type { CandidateSpec } from './app-uninstall-types';

export const macPath = path.posix;
export const MAX_PATTERN_MATCHES = 20;
const MIN_NAME_MATCH_LENGTH = 3;
const SYSTEM_APPLICATIONS = '/Applications';

export interface PatternRoot {
  slot: string;
  root: string;
  allowsNameMatch: boolean;
}

export type PatternMatch = 'prefix' | 'name';

export function isWithin(root: string, target: string) {
  const relative = macPath.relative(root, target);
  return (
    relative === '' ||
    (!macPath.isAbsolute(relative) &&
      relative !== '..' &&
      !relative.startsWith(`..${macPath.sep}`))
  );
}

export function userApplicationsRoot(homeDirectory: string) {
  return macPath.join(homeDirectory, 'Applications');
}

export function applicationRootFor(appPath: string, homeDirectory: string) {
  return isWithin(SYSTEM_APPLICATIONS, appPath)
    ? SYSTEM_APPLICATIONS
    : userApplicationsRoot(homeDirectory);
}

export function appSpecs(
  appPath: string,
  homeDirectory: string,
): CandidateSpec[] {
  return [
    {
      slot: 'app',
      path: appPath,
      root: SYSTEM_APPLICATIONS,
      kind: 'app',
    },
    {
      slot: 'app',
      path: appPath,
      root: userApplicationsRoot(homeDirectory),
      kind: 'app',
    },
  ];
}

export function exactAssociatedSpecs(
  homeDirectory: string,
  bundleId: string,
): CandidateSpec[] {
  const library = macPath.join(homeDirectory, 'Library');
  return [
    [
      'application-support',
      macPath.join(library, 'Application Support', bundleId),
      macPath.join(library, 'Application Support'),
    ],
    [
      'caches',
      macPath.join(library, 'Caches', bundleId),
      macPath.join(library, 'Caches'),
    ],
    [
      'preferences',
      macPath.join(library, 'Preferences', `${bundleId}.plist`),
      macPath.join(library, 'Preferences'),
    ],
    [
      'saved-state',
      macPath.join(
        library,
        'Saved Application State',
        `${bundleId}.savedState`,
      ),
      macPath.join(library, 'Saved Application State'),
    ],
    [
      'containers',
      macPath.join(library, 'Containers', bundleId),
      macPath.join(library, 'Containers'),
    ],
    [
      'application-scripts',
      macPath.join(library, 'Application Scripts', bundleId),
      macPath.join(library, 'Application Scripts'),
    ],
    [
      'http-storages',
      macPath.join(library, 'HTTPStorages', bundleId),
      macPath.join(library, 'HTTPStorages'),
    ],
    [
      'webkit',
      macPath.join(library, 'WebKit', bundleId),
      macPath.join(library, 'WebKit'),
    ],
    [
      'cookies',
      macPath.join(library, 'Cookies', `${bundleId}.binarycookies`),
      macPath.join(library, 'Cookies'),
    ],
  ].map(([slot, candidatePath, root]) => ({
    slot,
    path: candidatePath,
    root,
    kind: 'associated' as const,
  }));
}

export function patternRoots(homeDirectory: string): PatternRoot[] {
  const library = macPath.join(homeDirectory, 'Library');
  return [
    {
      slot: 'application-support',
      root: macPath.join(library, 'Application Support'),
      allowsNameMatch: true,
    },
    {
      slot: 'caches',
      root: macPath.join(library, 'Caches'),
      allowsNameMatch: true,
    },
    {
      slot: 'preferences',
      root: macPath.join(library, 'Preferences'),
      allowsNameMatch: false,
    },
    {
      slot: 'saved-state',
      root: macPath.join(library, 'Saved Application State'),
      allowsNameMatch: false,
    },
    {
      slot: 'containers',
      root: macPath.join(library, 'Containers'),
      allowsNameMatch: false,
    },
    {
      slot: 'http-storages',
      root: macPath.join(library, 'HTTPStorages'),
      allowsNameMatch: false,
    },
    {
      slot: 'webkit',
      root: macPath.join(library, 'WebKit'),
      allowsNameMatch: false,
    },
  ];
}

export function patternMatchFor(
  entryName: string,
  bundleId: string,
  appName: string,
  allowsNameMatch: boolean,
): PatternMatch | null {
  const lowerName = entryName.toLowerCase();
  if (lowerName.startsWith(`${bundleId.toLowerCase()}.`)) {
    return 'prefix';
  }
  const canNameMatch =
    allowsNameMatch && appName.length >= MIN_NAME_MATCH_LENGTH;
  return canNameMatch && lowerName === appName.toLowerCase() ? 'name' : null;
}
