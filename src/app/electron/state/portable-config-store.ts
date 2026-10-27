import {
  type PortableDiskReading,
  portableConfigSignature,
  readPortableConfigFile,
  writePortableConfigText,
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

export type PortableConfigPersistResult =
  | { ok: true; config: PortableConfig; conflicts: string[] }
  | { ok: false; reason: string; config: PortableConfig };

export type PortableConfigPollResult = {
  adopted: PortableConfig | null;
  blockedReason: string | null;
};

export type PortableConfigStoreDeps = {
  filePath: () => string;
  settingKinds: PortableSettingKinds;
  onConflict: (paths: string[]) => void;
  onBlockedReasonChange: (reason: string | null) => void;
};

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

// biome-ignore lint/complexity/noExcessiveLinesPerFunction: The store keeps load, poll, and persist transitions on one serialized state machine.
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

  function snapshot(): PortableConfigSnapshot {
    return {
      config: effective,
      writable: blockedReason === null,
      blockedReason,
    };
  }

  function rejectWrite(): PortableConfigPersistResult {
    return {
      ok: false,
      reason: blockedReason ?? 'Config is not writable',
      config: effective,
    };
  }

  async function mergeWithDisk(
    reading: Extract<PortableDiskReading, { kind: 'valid' }>,
    changes: ReturnType<typeof diffPortableConfig>,
  ) {
    const base = serializePortableConfig(
      lastValid ?? createDefaultPortableConfig(),
    );
    const conflicts = changes
      .filter(
        (change) =>
          !sameJson(
            valueAtPath(reading.raw, change.path),
            valueAtPath(base, change.path),
          ),
      )
      .map((change) => change.path.join('.'));
    return {
      text: portableConfigText(applyPortableChanges(reading.raw, changes)),
      conflicts,
    };
  }

  async function writeAndAdopt(
    text: string,
    conflicts: string[],
  ): Promise<PortableConfigPersistResult> {
    try {
      await writePortableConfigText(deps.filePath(), text);
    } catch (error) {
      return {
        ok: false,
        reason: `Could not write config: ${error instanceof Error ? error.message : String(error)}`,
        config: effective,
      };
    }
    const parsed = parsePortableConfigText(text, deps.settingKinds);
    if (!parsed.ok)
      return { ok: false, reason: parsed.reason, config: effective };
    lastValid = parsed.config;
    effective = parsed.config;
    lastSeenText = text;
    fileEverSeen = true;
    setBlockedReason(null);
    lastSignature = await portableConfigSignature(deps.filePath());
    if (conflicts.length) deps.onConflict(conflicts);
    return { ok: true, config: effective, conflicts };
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

  function persist(desired: PortableConfig) {
    return enqueue(async (): Promise<PortableConfigPersistResult> => {
      const changes = diffPortableConfig(
        lastValid ?? createDefaultPortableConfig(),
        desired,
      );
      if (!changes.length)
        return { ok: true, config: effective, conflicts: [] };
      if (blockedReason) return rejectWrite();
      const reading = await readPortableConfigFile(
        deps.filePath(),
        deps.settingKinds,
      );
      if (reading.kind === 'valid') {
        const merged = await mergeWithDisk(reading, changes);
        return writeAndAdopt(merged.text, merged.conflicts);
      }
      if (reading.kind === 'missing' && !lastValid && !fileEverSeen)
        return writeAndAdopt(
          portableConfigText(serializePortableConfig(desired)),
          [],
        );
      adoptReading(reading);
      return rejectWrite();
    });
  }

  function pollForExternalChange() {
    return enqueue(async (): Promise<PortableConfigPollResult> => {
      const signature = await portableConfigSignature(deps.filePath());
      if (signature === lastSignature) return { adopted: null, blockedReason };
      lastSignature = signature;
      const reading = await readPortableConfigFile(
        deps.filePath(),
        deps.settingKinds,
      );
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

  return {
    load,
    persist,
    pollForExternalChange,
    snapshot,
  };
}
