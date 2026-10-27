import {
  createPortableConfigFileExclusively,
  type PortableDiskReading,
  portableConfigSignature,
  readPortableConfigFile,
  readPortableConfigText,
  replacePortableConfigFile,
} from './portable-config-file';
import {
  applyPortableChanges,
  diffPortableConfig,
} from './portable-config-projection';
import {
  createDefaultPortableConfig,
  type PortableConfig,
  type PortableSettingKinds,
  parsePortableConfigText,
  portableConfigText,
  serializePortableConfig,
} from './portable-config-schema';

export type PortableConfigSnapshot = {
  config: PortableConfig;
  writable: boolean;
  blockedReason: string | null;
};

export type PortablePersistResult =
  | { ok: true; config: PortableConfig }
  | { ok: false; reason: string; current: PortableConfig };

export type PortablePollResult = {
  adopted: PortableConfig | null;
  blockedReason: string | null;
};

export type PortableConfigStoreDeps = {
  filePath: () => string;
  settingKinds: PortableSettingKinds;
  onBlockedReasonChange: (reason: string | null) => void;
};

const MAX_PERSIST_ATTEMPTS = 3;

function blockedReasonForReading(
  reading: Exclude<PortableDiskReading, { kind: 'valid' }>,
) {
  if (reading.kind === 'invalid') return `Config is invalid: ${reading.reason}`;
  if (reading.kind === 'inaccessible')
    return `Config is inaccessible: ${reading.reason}`;
  return 'Config file was deleted after it loaded';
}

function valueAtPath(document: Record<string, unknown>, segments: string[]) {
  return segments.reduce<unknown>(
    (owner, key) =>
      owner && typeof owner === 'object'
        ? (owner as Record<string, unknown>)[key]
        : undefined,
    document,
  );
}

function sameJson(left: unknown, right: unknown) {
  return JSON.stringify(left) === JSON.stringify(right);
}

function conflictReason(paths: string[]) {
  return `${paths.join(', ')} changed in config.json after Nevermind loaded it. Nothing was saved; the file was reloaded.`;
}

// biome-ignore lint/complexity/noExcessiveLinesPerFunction: The store keeps load, poll, and compare-and-swap persist transitions on one serialized state machine.
export function createPortableConfigStore(deps: PortableConfigStoreDeps) {
  let lastValid: PortableConfig | null = null;
  let effective: PortableConfig = createDefaultPortableConfig();
  let fileEverSeen = false;
  let blockedReason: string | null = null;
  let lastSignature: string | null = null;
  let lastSeenText: string | null = null;
  let queue: Promise<unknown> = Promise.resolve();

  function enqueue<T>(task: () => Promise<T>): Promise<T> {
    const run = queue.then(task);
    queue = run.catch(() => undefined);
    return run;
  }

  function setBlockedReason(next: string | null) {
    if (next === blockedReason) return;
    blockedReason = next;
    deps.onBlockedReasonChange(next);
  }

  function adoptValidReading(
    reading: Extract<PortableDiskReading, { kind: 'valid' }>,
  ) {
    lastValid = reading.config;
    effective = reading.config;
    lastSeenText = reading.text;
    fileEverSeen = true;
    setBlockedReason(null);
  }

  function adoptReading(reading: PortableDiskReading) {
    if (reading.kind === 'valid') return adoptValidReading(reading);
    if (reading.kind !== 'missing') fileEverSeen = true;
    const isCreatable =
      reading.kind === 'missing' && !lastValid && !fileEverSeen;
    setBlockedReason(isCreatable ? null : blockedReasonForReading(reading));
  }

  function adoptWrittenText(text: string) {
    const parsed = parsePortableConfigText(text, deps.settingKinds);
    if (!parsed.ok) return null;
    lastValid = parsed.config;
    effective = parsed.config;
    lastSeenText = text;
    fileEverSeen = true;
    setBlockedReason(null);
    return parsed.config;
  }

  function snapshot(): PortableConfigSnapshot {
    return {
      config: effective,
      writable: blockedReason === null,
      blockedReason,
    };
  }

  function writtenOutcome(text: string): PortablePersistResult {
    const config = adoptWrittenText(text);
    if (config) return { ok: true, config };
    return {
      ok: false,
      reason: 'Nevermind wrote an unreadable config.json; nothing was kept.',
      current: effective,
    };
  }

  function conflictsWithBase(
    base: PortableConfig,
    disk: PortableConfig,
    changes: ReturnType<typeof diffPortableConfig>,
  ) {
    const baseDocument = serializePortableConfig(base);
    const diskDocument = serializePortableConfig(disk);
    return changes
      .filter(
        (change) =>
          !sameJson(
            valueAtPath(diskDocument, change.path),
            valueAtPath(baseDocument, change.path),
          ),
      )
      .map((change) => change.path.join('.'));
  }

  async function createFirstFile(
    filePath: string,
    next: PortableConfig,
  ): Promise<PortablePersistResult | 'retry'> {
    const text = portableConfigText(serializePortableConfig(next));
    const created = await createPortableConfigFileExclusively(filePath, text);
    if (created === 'exists') return 'retry';
    return writtenOutcome(text);
  }

  async function replaceValidFile(
    filePath: string,
    reading: Extract<PortableDiskReading, { kind: 'valid' }>,
    next: PortableConfig,
  ): Promise<PortablePersistResult | 'retry'> {
    const base = lastValid ?? createDefaultPortableConfig();
    const changes = diffPortableConfig(base, next);
    const conflicts = conflictsWithBase(base, reading.config, changes);
    if (conflicts.length) {
      adoptValidReading(reading);
      return {
        ok: false,
        reason: conflictReason(conflicts),
        current: reading.config,
      };
    }
    if (!changes.length) {
      adoptValidReading(reading);
      return { ok: true, config: reading.config };
    }
    const text = portableConfigText(applyPortableChanges(reading.raw, changes));
    const replaced = await replacePortableConfigFile(
      filePath,
      text,
      async () => (await readPortableConfigText(filePath)) === reading.text,
    );
    return replaced === 'changed' ? 'retry' : writtenOutcome(text);
  }

  async function persistOnce(
    next: PortableConfig,
  ): Promise<PortablePersistResult | 'retry'> {
    const filePath = deps.filePath();
    const reading = await readPortableConfigFile(filePath, deps.settingKinds);
    if (reading.kind === 'missing' && !lastValid && !fileEverSeen)
      return createFirstFile(filePath, next);
    if (reading.kind !== 'valid') {
      adoptReading(reading);
      return {
        ok: false,
        reason: blockedReason ?? 'Config is not writable',
        current: effective,
      };
    }
    return replaceValidFile(filePath, reading, next);
  }

  function load() {
    return enqueue(async () => {
      adoptReading(
        await readPortableConfigFile(deps.filePath(), deps.settingKinds),
      );
      lastSignature = await portableConfigSignature(deps.filePath());
      return snapshot();
    });
  }

  function persist(next: PortableConfig) {
    return enqueue(async (): Promise<PortablePersistResult> => {
      try {
        for (let attempt = 0; attempt < MAX_PERSIST_ATTEMPTS; attempt += 1) {
          const outcome = await persistOnce(next);
          if (outcome !== 'retry') return outcome;
        }
      } catch (error) {
        return {
          ok: false,
          reason: `Could not save config.json: ${error instanceof Error ? error.message : String(error)}`,
          current: effective,
        };
      }
      return {
        ok: false,
        reason:
          'config.json kept changing while Nevermind was saving. Nothing was saved; try again.',
        current: effective,
      };
    });
  }

  function pollForExternalChange() {
    return enqueue(async (): Promise<PortablePollResult> => {
      const filePath = deps.filePath();
      const signature = await portableConfigSignature(filePath);
      if (signature === lastSignature) return { adopted: null, blockedReason };
      lastSignature = signature;
      const reading = await readPortableConfigFile(filePath, deps.settingKinds);
      if (reading.kind === 'valid' && reading.text === lastSeenText)
        return { adopted: null, blockedReason };
      const previous = effective;
      adoptReading(reading);
      const adopted =
        reading.kind === 'valid' && !sameJson(previous, effective)
          ? effective
          : null;
      return { adopted, blockedReason };
    });
  }

  return { load, persist, pollForExternalChange, snapshot };
}
