import type { PortableChange } from './portable-config-projection';
import type { PortableConfig } from './portable-config-schema';

const PORTABLE_ACTION_ID_PATTERN =
  /^(?:extension|extension-action|extension-root):[A-Za-z0-9][A-Za-z0-9._-]{0,127}(?::[A-Za-z0-9][A-Za-z0-9._-]{0,127}){1,3}$|^default:[a-z][a-z-]{0,63}$/;

type IdMap<T> = Record<string, T>;

export function isPortableActionId(actionId: string) {
  return PORTABLE_ACTION_ID_PATTERN.test(actionId);
}

function keyOfChange(change: PortableChange) {
  const [section, id, job] = change.path;
  if (section === 'settings') return `setting:${id}`;
  if (section === 'aliases') return `alias:${id}`;
  if (section === 'shortcuts') return `shortcut:${id}`;
  if (section === 'jobs') return `job:${job}`;
  return 'model';
}

export function changedKeysOf(changes: PortableChange[]) {
  return [...new Set(changes.map(keyOfChange))];
}

export function isLocalChange(
  change: PortableChange,
  localIds: ReadonlySet<string>,
) {
  const [section, id] = change.path;
  return (section === 'aliases' || section === 'shortcuts') && localIds.has(id);
}

function keepOnly<T>(map: IdMap<T>, ids: ReadonlySet<string>): IdMap<T> {
  return Object.fromEntries(Object.entries(map).filter(([id]) => ids.has(id)));
}

function dropIds<T>(map: IdMap<T>, ids: ReadonlySet<string>): IdMap<T> {
  return Object.fromEntries(Object.entries(map).filter(([id]) => !ids.has(id)));
}

export function diskViewOf(
  view: PortableConfig,
  localIds: ReadonlySet<string>,
): PortableConfig {
  return {
    ...view,
    aliases: dropIds(view.aliases, localIds),
    shortcuts: dropIds(view.shortcuts, localIds),
  };
}

export function mergeDiskView(
  local: PortableConfig,
  disk: PortableConfig,
  localIds: ReadonlySet<string>,
): PortableConfig {
  return {
    settings: { ...disk.settings },
    aliases: {
      ...dropIds(disk.aliases, localIds),
      ...keepOnly(local.aliases, localIds),
    },
    shortcuts: {
      ...dropIds(disk.shortcuts, localIds),
      ...keepOnly(local.shortcuts, localIds),
    },
    jobs: { enabled: { ...disk.jobs.enabled } },
    ai: { defaultModel: disk.ai.defaultModel },
  };
}

export function localBundleOf(record: Record<string, any>) {
  const localIds = new Set<string>(record.localActionIds ?? []);
  return {
    localActionIds: [...localIds],
    localAliases: keepOnly(record.aliases ?? {}, localIds),
    localShortcuts: keepOnly(record.shortcuts ?? {}, localIds),
    localRemovedShortcuts: keepOnly(record.removedShortcuts ?? {}, localIds),
  };
}
