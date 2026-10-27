import {
  DEFAULT_AI_CHAT_MODEL,
  normalizeAiChatModel,
} from '../../shared/ai-chat-model';

export const PORTABLE_CONFIG_VERSION = 1;

export type PortableSettingKinds = Record<string, 'boolean' | 'shortcut'>;
export type PortableSettingValue = string | boolean;

export type PortableConfig = {
  settings: Record<string, PortableSettingValue>;
  aliases: Record<string, string[]>;
  shortcuts: Record<string, string | null>;
  jobs: { enabled: Record<string, boolean> };
  ai: { defaultModel: string };
};

export type PortableConfigParseResult =
  | { ok: true; raw: Record<string, unknown>; config: PortableConfig }
  | { ok: false; reason: string };

type SectionResult = { ok: true } | { ok: false; reason: string };

const OK: SectionResult = { ok: true };

export function createDefaultPortableConfig(): PortableConfig {
  return {
    settings: {},
    aliases: {},
    shortcuts: {},
    jobs: { enabled: {} },
    ai: { defaultModel: DEFAULT_AI_CHAT_MODEL },
  };
}

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function isNonEmptyString(value: unknown): value is string {
  return typeof value === 'string' && value.trim().length > 0;
}

function describeType(value: unknown) {
  return Array.isArray(value) ? 'array' : typeof value;
}

function validateOptionalObject(
  raw: Record<string, unknown>,
  key: string,
): SectionResult {
  if (raw[key] === undefined || isPlainObject(raw[key])) return OK;
  return { ok: false, reason: `"${key}" must be an object` };
}

function validateSettings(
  raw: Record<string, unknown>,
  kinds: PortableSettingKinds,
): SectionResult {
  const section = validateOptionalObject(raw, 'settings');
  if (!section.ok) return section;
  for (const [id, value] of Object.entries(raw.settings ?? {})) {
    if (!(id in kinds)) continue;
    const expected = kinds[id] === 'boolean' ? 'boolean' : 'string';
    if (typeof value !== expected)
      return { ok: false, reason: `settings.${id} must be a ${expected}` };
  }
  return OK;
}

function validateAliases(raw: Record<string, unknown>): SectionResult {
  const section = validateOptionalObject(raw, 'aliases');
  if (!section.ok) return section;
  for (const [actionId, aliases] of Object.entries(raw.aliases ?? {})) {
    const valid =
      Array.isArray(aliases) &&
      aliases.every((alias) => isNonEmptyString(alias));
    if (!valid)
      return {
        ok: false,
        reason: `aliases.${actionId} must be an array of non-empty strings`,
      };
  }
  return OK;
}

function validateShortcuts(raw: Record<string, unknown>): SectionResult {
  const section = validateOptionalObject(raw, 'shortcuts');
  if (!section.ok) return section;
  for (const [actionId, accelerator] of Object.entries(raw.shortcuts ?? {})) {
    if (accelerator === null || isNonEmptyString(accelerator)) continue;
    return {
      ok: false,
      reason: `shortcuts.${actionId} must be an accelerator string or null`,
    };
  }
  return OK;
}

function validateJobs(raw: Record<string, unknown>): SectionResult {
  const section = validateOptionalObject(raw, 'jobs');
  if (!section.ok) return section;
  const enabled = (raw.jobs as Record<string, unknown> | undefined)?.enabled;
  if (enabled === undefined) return OK;
  if (!isPlainObject(enabled))
    return { ok: false, reason: '"jobs.enabled" must be an object' };
  for (const [jobId, value] of Object.entries(enabled)) {
    if (typeof value !== 'boolean')
      return { ok: false, reason: `jobs.enabled.${jobId} must be a boolean` };
  }
  return OK;
}

function validateAi(raw: Record<string, unknown>): SectionResult {
  const section = validateOptionalObject(raw, 'ai');
  if (!section.ok) return section;
  const model = (raw.ai as Record<string, unknown> | undefined)?.defaultModel;
  if (model === undefined || typeof model === 'string') return OK;
  return {
    ok: false,
    reason: `ai.defaultModel must be a string, got ${describeType(model)}`,
  };
}

function validatePortableRaw(
  raw: Record<string, unknown>,
  kinds: PortableSettingKinds,
): SectionResult {
  if (raw.version !== PORTABLE_CONFIG_VERSION)
    return {
      ok: false,
      reason: `Unsupported config version ${String(raw.version)}`,
    };
  for (const section of [
    validateSettings(raw, kinds),
    validateAliases(raw),
    validateShortcuts(raw),
    validateJobs(raw),
    validateAi(raw),
  ]) {
    if (!section.ok) return section;
  }
  return OK;
}

function configFromValidRaw(
  raw: Record<string, unknown>,
  kinds: PortableSettingKinds,
): PortableConfig {
  const settings: Record<string, PortableSettingValue> = {};
  for (const [id, value] of Object.entries(raw.settings ?? {})) {
    if (id in kinds) settings[id] = value as PortableSettingValue;
  }
  const jobs = raw.jobs as { enabled?: Record<string, boolean> } | undefined;
  const ai = raw.ai as { defaultModel?: unknown } | undefined;
  return {
    settings,
    aliases: (raw.aliases ?? {}) as Record<string, string[]>,
    shortcuts: (raw.shortcuts ?? {}) as Record<string, string | null>,
    jobs: { enabled: { ...(jobs?.enabled ?? {}) } },
    ai: {
      defaultModel: normalizeAiChatModel(
        typeof ai?.defaultModel === 'string' ? ai.defaultModel : undefined,
      ),
    },
  };
}

export function parsePortableConfigText(
  text: string,
  kinds: PortableSettingKinds,
): PortableConfigParseResult {
  let parsed: unknown;
  try {
    parsed = JSON.parse(text);
  } catch (error) {
    return {
      ok: false,
      reason: `Invalid JSON: ${error instanceof Error ? error.message : String(error)}`,
    };
  }
  if (!isPlainObject(parsed))
    return { ok: false, reason: 'Config must be a JSON object' };
  const validation = validatePortableRaw(parsed, kinds);
  if (!validation.ok) return validation;
  return { ok: true, raw: parsed, config: configFromValidRaw(parsed, kinds) };
}

export function serializePortableConfig(config: PortableConfig) {
  return {
    version: PORTABLE_CONFIG_VERSION,
    settings: { ...config.settings },
    aliases: { ...config.aliases },
    shortcuts: { ...config.shortcuts },
    jobs: { enabled: { ...config.jobs.enabled } },
    ai: { defaultModel: config.ai.defaultModel },
  };
}

export function portableConfigText(document: Record<string, unknown>) {
  return `${JSON.stringify(document, null, 2)}\n`;
}
