import { bundleIdsOverlap } from './app-uninstall-bundle-id';
import { isMissing } from './app-uninstall-inspect';
import { macPath, userApplicationsRoot } from './app-uninstall-paths';
import type { AppUninstallDependencies } from './app-uninstall-types';

const OWNER_READ_CONCURRENCY = 8;
const SYSTEM_APPLICATIONS = '/Applications';

export type OwnershipVerdict =
  | { status: 'confirmed' }
  | { status: 'shared'; owner: string }
  | { status: 'unverified'; message: string };

type OwnerReading =
  | { status: 'self' }
  | { status: 'bundle'; name: string; bundleId: string }
  | { status: 'unreadable'; name: string };

type OwnershipDependencies = Pick<
  AppUninstallDependencies,
  | 'homeDirectory'
  | 'listDirectory'
  | 'realpath'
  | 'readBundleId'
  | 'findMatchingApps'
>;

async function mapInChunks<T, R>(
  items: T[],
  size: number,
  mapper: (item: T) => Promise<R>,
): Promise<R[]> {
  const results: R[] = [];
  for (let index = 0; index < items.length; index += size) {
    const chunk = items.slice(index, index + size);
    results.push(...(await Promise.all(chunk.map(mapper))));
  }
  return results;
}

export function createOwnershipVerifier(deps: OwnershipDependencies) {
  const installRoots = [
    SYSTEM_APPLICATIONS,
    userApplicationsRoot(deps.homeDirectory),
  ];

  async function listRootApps(root: string) {
    try {
      const names = await deps.listDirectory(root);
      return {
        readable: true,
        appPaths: names
          .filter((name) => name.endsWith('.app'))
          .map((name) => macPath.join(root, name)),
      };
    } catch (error) {
      return { readable: isMissing(error), appPaths: [] };
    }
  }

  async function readOwner(
    appPath: string,
    appCanonicalPath: string,
  ): Promise<OwnerReading> {
    const name = macPath.basename(appPath, '.app');
    try {
      if ((await deps.realpath(appPath)) === appCanonicalPath) {
        return { status: 'self' };
      }
      const bundleId = await deps.readBundleId(appPath);
      if (typeof bundleId !== 'string' || !bundleId) {
        return { status: 'unreadable', name };
      }
      return { status: 'bundle', name, bundleId };
    } catch {
      return { status: 'unreadable', name };
    }
  }

  return async function verifyBundleIdOwnership(
    bundleId: string,
    appCanonicalPath: string,
  ): Promise<OwnershipVerdict> {
    if (deps.findMatchingApps) {
      try {
        const paths = await deps.findMatchingApps(bundleId);
        const canonicalPaths = await Promise.all(
          paths.map((item) => deps.realpath(item)),
        );
        if (!canonicalPaths.includes(appCanonicalPath)) {
          return {
            status: 'unverified',
            message: 'The application index is incomplete',
          };
        }
        const other = canonicalPaths.find((item) => item !== appCanonicalPath);
        return other
          ? { status: 'shared', owner: macPath.basename(other, '.app') }
          : { status: 'confirmed' };
      } catch {
        return {
          status: 'unverified',
          message: 'The application index could not be checked',
        };
      }
    }
    const listings = await Promise.all(installRoots.map(listRootApps));
    if (listings.some((listing) => !listing.readable)) {
      return {
        status: 'unverified',
        message: 'The applications folders could not be fully read',
      };
    }
    const appPaths = listings.flatMap((listing) => listing.appPaths);
    const readings = await mapInChunks(
      appPaths,
      OWNER_READ_CONCURRENCY,
      (appPath) => readOwner(appPath, appCanonicalPath),
    );
    const sharedOwners = readings.flatMap((reading) =>
      reading.status === 'bundle' &&
      bundleIdsOverlap(reading.bundleId, bundleId)
        ? [reading.name]
        : [],
    );
    if (sharedOwners.length > 0) {
      return { status: 'shared', owner: sharedOwners[0] };
    }
    if (readings.some((reading) => reading.status === 'unreadable')) {
      return {
        status: 'unverified',
        message: 'Another installed app could not be read',
      };
    }
    return { status: 'confirmed' };
  };
}
