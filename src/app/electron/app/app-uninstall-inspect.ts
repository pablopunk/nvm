// biome-ignore-all lint/performance/noAwaitInLoops: each path component must be inspected in order before descending.
import { isWithin, macPath } from './app-uninstall-paths';
import type {
  AppUninstallDependencies,
  CandidateSpec,
  Identity,
  Inspection,
  Stat,
} from './app-uninstall-types';

export function isMissing(error: unknown) {
  return Boolean(
    error &&
      typeof error === 'object' &&
      (error as { code?: string }).code === 'ENOENT',
  );
}

export function safeMessage(error: unknown) {
  return isMissing(error)
    ? 'This item is no longer available'
    : 'This item cannot be accessed safely';
}

function statType(stat: Stat): Identity['type'] | null {
  if (stat.isSymbolicLink()) {
    return null;
  }
  return stat.isDirectory() ? 'directory' : 'file';
}

export function identityFromStat(stat: Stat): Identity | null {
  const type = statType(stat);
  return type ? { type, dev: stat.dev, ino: stat.ino, uid: stat.uid } : null;
}

export function sameIdentity(left: Identity, right: Identity) {
  return (
    left.type === right.type && left.dev === right.dev && left.ino === right.ino
  );
}

export function createPathInspector(
  deps: Pick<AppUninstallDependencies, 'lstat' | 'realpath'>,
) {
  async function inspectComponents(value: string): Promise<Inspection | null> {
    const parsed = macPath.parse(macPath.resolve(value));
    const components = macPath
      .relative(parsed.root, macPath.resolve(value))
      .split(macPath.sep)
      .filter(Boolean);
    let current = parsed.root;
    for (const component of components) {
      current = macPath.join(current, component);
      try {
        const stat = await deps.lstat(current);
        if (stat.isSymbolicLink()) {
          return {
            code: 'symlink',
            message: `Refusing symbolic-link path component: ${current}`,
          };
        }
      } catch (error) {
        return {
          code: isMissing(error) ? 'missing' : 'inaccessible',
          message: safeMessage(error),
        };
      }
    }
    return null;
  }

  async function inspectPath(value: string): Promise<Inspection> {
    const componentRefusal = await inspectComponents(value);
    if (componentRefusal) {
      return componentRefusal;
    }
    try {
      const itemIdentity = identityFromStat(await deps.lstat(value));
      if (!itemIdentity) {
        return { code: 'symlink', message: `Refusing symbolic link: ${value}` };
      }
      return { canonical: await deps.realpath(value), identity: itemIdentity };
    } catch (error) {
      return {
        code: isMissing(error) ? 'missing' : 'inaccessible',
        message: safeMessage(error),
      };
    }
  }

  async function inspectAllowed(spec: CandidateSpec): Promise<Inspection> {
    const [root, target] = await Promise.all([
      inspectPath(spec.root),
      inspectPath(spec.path),
    ]);
    if (!('canonical' in root)) {
      return root;
    }
    if (!('canonical' in target)) {
      return target;
    }
    if (!isWithin(root.canonical, target.canonical)) {
      return {
        code: 'outside-allowlist',
        message: `Refusing path outside ${spec.root}`,
      };
    }
    return target;
  }

  return { inspectPath, inspectAllowed };
}

export type PathInspector = ReturnType<typeof createPathInspector>;
