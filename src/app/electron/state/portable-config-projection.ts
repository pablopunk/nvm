import { normalizeAiChatModel } from '../../shared/ai-chat-model';
import type {
  PortableConfig,
  PortableSettingKinds,
} from './portable-config-schema';
import { localBundleOf } from './portable-keys';

export type PortableChange = {
  path: string[];
  removed: boolean;
  value?: unknown;
};

const PORTABLE_RECORD_KEYS = [
  'settings',
  'aliases',
  'shortcuts',
  'removedShortcuts',
  'jobSettings',
  'aiChatDefaultModel',
] as const;

type RecordLike = Record<string, any>;

function copyKnownSettings(
  settings: RecordLike | undefined,
  kinds: PortableSettingKinds,
) {
  const copy: PortableConfig['settings'] = {};
  for (const id of Object.keys(kinds)) {
    if (settings?.[id] !== undefined) copy[id] = settings[id];
  }
  return copy;
}

function copyNonEmptyAliases(aliases: RecordLike | undefined) {
  const copy: PortableConfig['aliases'] = {};
  for (const [actionId, values] of Object.entries(aliases ?? {})) {
    if (Array.isArray(values) && values.length) copy[actionId] = [...values];
  }
  return copy;
}

function shortcutsFromRecord(record: RecordLike) {
  const shortcuts: PortableConfig['shortcuts'] = {};
  for (const [actionId, accelerator] of Object.entries(
    record.shortcuts ?? {},
  )) {
    if (accelerator) shortcuts[actionId] = String(accelerator);
  }
  for (const [actionId, removed] of Object.entries(
    record.removedShortcuts ?? {},
  )) {
    if (removed) shortcuts[actionId] = null;
  }
  return shortcuts;
}

export function portableConfigFromRecord(
  record: RecordLike,
  kinds: PortableSettingKinds,
): PortableConfig {
  return {
    settings: copyKnownSettings(record.settings, kinds),
    aliases: copyNonEmptyAliases(record.aliases),
    shortcuts: shortcutsFromRecord(record),
    jobs: { enabled: { ...(record.jobSettings?.enabled ?? {}) } },
    ai: { defaultModel: normalizeAiChatModel(record.aiChatDefaultModel) },
  };
}

function shortcutRecordsFromConfig(shortcuts: PortableConfig['shortcuts']) {
  const accelerators: Record<string, string> = {};
  const removed: Record<string, true> = {};
  for (const [actionId, accelerator] of Object.entries(shortcuts)) {
    if (accelerator === null) removed[actionId] = true;
    else accelerators[actionId] = accelerator;
  }
  return { accelerators, removed };
}

export function applyPortableConfigToRecord(
  record: RecordLike,
  config: PortableConfig,
) {
  const shortcuts = shortcutRecordsFromConfig(config.shortcuts);
  record.settings = { ...config.settings };
  record.aliases = { ...config.aliases };
  record.shortcuts = shortcuts.accelerators;
  record.removedShortcuts = shortcuts.removed;
  record.jobSettings = {
    ...(record.jobSettings ?? {}),
    enabled: { ...config.jobs.enabled },
  };
  record.aiChatDefaultModel = config.ai.defaultModel;
}

export function recordWithoutPortableFields(record: RecordLike) {
  const privateRecord: RecordLike = { ...record };
  for (const key of PORTABLE_RECORD_KEYS) delete privateRecord[key];
  return privateRecord;
}

function sameValue(left: unknown, right: unknown) {
  return JSON.stringify(left) === JSON.stringify(right);
}

function diffSection(
  path: string[],
  base: Record<string, unknown>,
  next: Record<string, unknown>,
): PortableChange[] {
  const keys = new Set([...Object.keys(base), ...Object.keys(next)]);
  const changes: PortableChange[] = [];
  for (const key of keys) {
    if (sameValue(base[key], next[key])) continue;
    changes.push(
      next[key] === undefined
        ? { path: [...path, key], removed: true }
        : { path: [...path, key], removed: false, value: next[key] },
    );
  }
  return changes;
}

export function diffPortableConfig(
  base: PortableConfig,
  next: PortableConfig,
): PortableChange[] {
  return [
    ...diffSection(['settings'], base.settings, next.settings),
    ...diffSection(['aliases'], base.aliases, next.aliases),
    ...diffSection(['shortcuts'], base.shortcuts, next.shortcuts),
    ...diffSection(['jobs', 'enabled'], base.jobs.enabled, next.jobs.enabled),
    ...(sameValue(base.ai.defaultModel, next.ai.defaultModel)
      ? []
      : [
          {
            path: ['ai', 'defaultModel'],
            removed: false,
            value: next.ai.defaultModel,
          },
        ]),
  ];
}

export function changedPortablePaths(changes: PortableChange[]) {
  return changes.map((change) => change.path.join('.'));
}

function ensureObjectAt(parent: RecordLike, key: string) {
  if (!isRecord(parent[key])) parent[key] = {};
  return parent[key] as RecordLike;
}

function isRecord(value: unknown): value is RecordLike {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

export function applyPortableChanges(
  document: RecordLike,
  changes: PortableChange[],
) {
  const next: RecordLike = structuredClone(document);
  for (const change of changes) {
    const leafKey = change.path[change.path.length - 1];
    const parent = change.path
      .slice(0, -1)
      .reduce<RecordLike>((owner, key) => ensureObjectAt(owner, key), next);
    if (change.removed) delete parent[leafKey];
    else parent[leafKey] = change.value;
  }
  return next;
}

export function recordForPrivateState(record: RecordLike) {
  return { ...recordWithoutPortableFields(record), ...localBundleOf(record) };
}
