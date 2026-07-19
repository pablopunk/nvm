import { randomBytes } from 'node:crypto';

export type DraftVersion = string | number;

export type ExtensionDraftRecord = {
  schemaVersion: 1;
  ownerExtensionId: string;
  draftKey: string;
  draftVersion: DraftVersion;
  content: string;
};

export type DraftConflict = {
  handle: string;
  key: string;
  storedVersion: DraftVersion;
  storedContent: string;
  currentVersion: DraftVersion;
  currentContent: string;
};

export type DraftResolution =
  | {
      type: 'draftResolution';
      key: string;
      resolution: 'reset' | 'restore-old';
    }
  | {
      type: 'draftResolution';
      key: string;
      resolution: 'migrate';
      content: string;
    };

type ConflictRecord = DraftConflict & {
  ownerExtensionId: string;
  expiresAt: number;
};

type DraftStoreDeps = {
  now?: () => number;
  conflictTtlMs?: number;
};

const STABLE_KEY = /^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/;
const MAX_VERSION_BYTES = 64;
const MAX_CONTENT_BYTES = 524_288;
const DEFAULT_CONFLICT_TTL_MS = 5 * 60_000;

function byteLength(value: string) {
  return Buffer.byteLength(value, 'utf8');
}

function assertStableKey(
  value: unknown,
  name: string,
): asserts value is string {
  if (typeof value !== 'string' || !STABLE_KEY.test(value))
    throw new Error(`Invalid ${name}`);
}

function assertVersion(value: unknown): asserts value is DraftVersion {
  if (
    typeof value === 'string' &&
    byteLength(value) >= 1 &&
    byteLength(value) <= MAX_VERSION_BYTES
  )
    return;
  if (
    typeof value === 'number' &&
    Number.isInteger(value) &&
    value >= 0 &&
    value <= 2_147_483_647
  )
    return;
  throw new Error('Invalid draft version');
}

function assertContent(value: unknown): asserts value is string {
  if (typeof value !== 'string' || byteLength(value) > MAX_CONTENT_BYTES)
    throw new Error('Invalid draft content');
}

function recordKey(ownerExtensionId: string, draftKey: string) {
  return `${ownerExtensionId}\u0000${draftKey}`;
}

/**
 * Host-owned draft state. The only lookup key is owner plus draft key; versions
 * intentionally participate in conflict detection rather than storage identity.
 */
export function createExtensionDraftStore(deps: DraftStoreDeps = {}) {
  const now = deps.now ?? Date.now;
  const conflictTtlMs = deps.conflictTtlMs ?? DEFAULT_CONFLICT_TTL_MS;
  const records = new Map<string, ExtensionDraftRecord>();
  const conflicts = new Map<string, ConflictRecord>();

  function validateInput(
    ownerExtensionId: unknown,
    key: unknown,
    version: unknown,
    content: unknown,
  ) {
    assertStableKey(ownerExtensionId, 'owner extension id');
    assertStableKey(key, 'draft key');
    assertVersion(version);
    assertContent(content);
  }

  function save(
    ownerExtensionId: string,
    draftKey: string,
    draftVersion: DraftVersion,
    content: string,
  ) {
    validateInput(ownerExtensionId, draftKey, draftVersion, content);
    const record: ExtensionDraftRecord = {
      schemaVersion: 1,
      ownerExtensionId,
      draftKey,
      draftVersion,
      content,
    };
    records.set(recordKey(ownerExtensionId, draftKey), record);
    return record;
  }

  function open(
    ownerExtensionId: string,
    draftKey: string,
    draftVersion: DraftVersion,
    currentContent: string,
  ) {
    validateInput(ownerExtensionId, draftKey, draftVersion, currentContent);
    const stored = records.get(recordKey(ownerExtensionId, draftKey));
    if (!stored) return { kind: 'empty' as const, content: currentContent };
    if (stored.draftVersion === draftVersion)
      return { kind: 'recovered' as const, content: stored.content };

    const handle = randomBytes(32).toString('base64url');
    const conflict: ConflictRecord = {
      handle,
      ownerExtensionId,
      key: draftKey,
      storedVersion: stored.draftVersion,
      storedContent: stored.content,
      currentVersion: draftVersion,
      currentContent,
      expiresAt: now() + conflictTtlMs,
    };
    conflicts.set(handle, conflict);
    return {
      kind: 'conflict' as const,
      content: currentContent,
      conflict: { ...conflict },
    };
  }

  function resolve(
    ownerExtensionId: string,
    handle: string,
    resolution: DraftResolution,
  ) {
    const conflict = conflicts.get(handle);
    conflicts.delete(handle);
    if (!conflict || conflict.expiresAt < now())
      throw new Error('Stale draft conflict');
    if (conflict.ownerExtensionId !== ownerExtensionId)
      throw new Error('Draft conflict owner mismatch');
    if (resolution.key !== conflict.key)
      throw new Error('Draft conflict key mismatch');
    const stored = records.get(recordKey(ownerExtensionId, conflict.key));
    if (
      !stored ||
      stored.draftVersion !== conflict.storedVersion ||
      stored.content !== conflict.storedContent
    )
      throw new Error('Stale draft conflict');

    let content: string;
    switch (resolution.resolution) {
      case 'reset':
        content = conflict.currentContent;
        break;
      case 'restore-old':
        content = conflict.storedContent;
        break;
      case 'migrate':
        content = resolution.content;
        break;
    }
    return save(
      ownerExtensionId,
      conflict.key,
      conflict.currentVersion,
      content,
    );
  }

  function remove(ownerExtensionId: string, draftKey: string) {
    assertStableKey(ownerExtensionId, 'owner extension id');
    assertStableKey(draftKey, 'draft key');
    records.delete(recordKey(ownerExtensionId, draftKey));
  }

  function purgeOwner(ownerExtensionId: string) {
    assertStableKey(ownerExtensionId, 'owner extension id');
    for (const [key, record] of records) {
      if (record.ownerExtensionId === ownerExtensionId) records.delete(key);
    }
    for (const [handle, conflict] of conflicts) {
      if (conflict.ownerExtensionId === ownerExtensionId)
        conflicts.delete(handle);
    }
  }

  return { records, save, open, resolve, remove, purgeOwner };
}
