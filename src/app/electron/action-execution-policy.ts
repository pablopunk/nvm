import { createHmac, randomBytes, timingSafeEqual } from 'node:crypto';

interface ActionExecutionRecord {
  action: unknown;
  createdAt: number;
}

type ActionExecutionScope = 'root' | 'view';

const MUTABLE_ACTION_FIELDS = [
  'formValues',
  'editorContent',
  'selectedItemId',
  'value',
  'text',
] as const;

type MutableActionField = (typeof MUTABLE_ACTION_FIELDS)[number];
const ACTION_CAPABILITY_SECRET_BYTES = 32;
const ACTION_CAPABILITY_NONCE_BYTES = 12;
const ACTION_CAPABILITY_PARTS = 8;
const ACTION_CAPABILITY_METADATA_PARTS = 7;
const ACTION_CAPABILITY_MAX_DEPTH = 32;
const ACTION_CAPABILITY_MAX_NODES = 10_000;
const ACTION_CAPABILITY_MAX_BYTES = 4_194_304;
const ACTION_CAPABILITY_TIMESTAMP_RADIX = 36;

interface ActionCapabilityIssueOptions {
  scope: ActionExecutionScope;
  owner: string;
  ownerVersion: string;
  mutableFields?: readonly MutableActionField[];
  issuedAt?: number;
}

interface ActionCapabilityResolveOptions {
  scope: ActionExecutionScope;
  ownerIsCurrent: (owner: string, ownerVersion: string) => boolean;
  maxAgeMs?: number;
  now?: number;
}

interface CanonicalValueState {
  ancestors: Set<object>;
  bytes: number;
  nodes: number;
}

function arrayIsSparse(value: unknown[]) {
  for (let index = 0; index < value.length; index += 1) {
    if (!(index in value)) {
      return true;
    }
  }
  return false;
}

function canonicalPrimitive(value: unknown, state: CanonicalValueState) {
  if (value === undefined) {
    return 'undefined';
  }
  if (value === null || typeof value === 'boolean') {
    return JSON.stringify(value);
  }
  if (typeof value === 'string') {
    state.bytes += Buffer.byteLength(value);
    if (state.bytes > ACTION_CAPABILITY_MAX_BYTES) {
      throw new Error('Action capability payload is too large');
    }
    return JSON.stringify(value);
  }
  if (typeof value === 'number') {
    if (!Number.isFinite(value)) {
      throw new Error('Unsupported action number');
    }
    return Object.is(value, -0) ? '-0' : String(value);
  }
  throw new Error('Unsupported action value');
}

function canonicalObject(
  value: object,
  state: CanonicalValueState,
  depth: number,
) {
  if (state.ancestors.has(value)) {
    throw new Error('Cyclic action value');
  }
  state.ancestors.add(value);
  try {
    if (Array.isArray(value)) {
      if (value.length > ACTION_CAPABILITY_MAX_NODES) {
        throw new Error('Action capability payload is too complex');
      }
      if (arrayIsSparse(value)) {
        throw new Error('Sparse action array');
      }
      return `[${value
        .map((item) => canonicalValue(item, state, depth + 1))
        .join(',')}]`;
    }
    const prototype = Object.getPrototypeOf(value);
    if (prototype !== Object.prototype && prototype !== null) {
      throw new Error('Unsupported action object');
    }
    const record = value as Record<string, unknown>;
    const keys = Object.keys(record);
    if (keys.length > ACTION_CAPABILITY_MAX_NODES) {
      throw new Error('Action capability payload is too complex');
    }
    return `{${keys
      .sort()
      .map((key) => {
        state.bytes += Buffer.byteLength(key);
        if (state.bytes > ACTION_CAPABILITY_MAX_BYTES) {
          throw new Error('Action capability payload is too large');
        }
        return `${JSON.stringify(key)}:${canonicalValue(record[key], state, depth + 1)}`;
      })
      .join(',')}}`;
  } finally {
    state.ancestors.delete(value);
  }
}

function canonicalValue(
  value: unknown,
  state: CanonicalValueState = {
    ancestors: new Set<object>(),
    bytes: 0,
    nodes: 0,
  },
  depth = 0,
): string {
  state.nodes += 1;
  if (
    depth > ACTION_CAPABILITY_MAX_DEPTH ||
    state.nodes > ACTION_CAPABILITY_MAX_NODES
  ) {
    throw new Error('Action capability payload is too complex');
  }
  return typeof value === 'object' && value !== null
    ? canonicalObject(value, state, depth)
    : canonicalPrimitive(value, state);
}

function actionWithoutMutableFields(
  action: Record<string, unknown>,
  mutableFields: Set<string>,
) {
  const entries = Object.entries(action);
  if (entries.length > ACTION_CAPABILITY_MAX_NODES) {
    throw new Error('Action capability payload is too complex');
  }
  return Object.fromEntries(
    entries.filter(
      ([key]) =>
        key !== 'executionId' && key !== 'traceId' && !mutableFields.has(key),
    ),
  );
}

function mutableFieldToken(fields: readonly MutableActionField[]) {
  return [...new Set(fields)].sort().join(',');
}

function mutableFieldsFromToken(token: string) {
  const fields = token ? token.split(',') : [];
  if (
    new Set(fields).size !== fields.length ||
    fields.some(
      (field) => !MUTABLE_ACTION_FIELDS.includes(field as MutableActionField),
    )
  ) {
    return null;
  }
  return new Set(fields);
}

function capabilityDigest(
  secret: Buffer,
  metadata: string,
  action: Record<string, unknown>,
  mutableFields: Set<string>,
) {
  return createHmac('sha256', secret)
    .update(metadata)
    .update('\0')
    .update(canonicalValue(actionWithoutMutableFields(action, mutableFields)))
    .digest();
}

function issueActionCapability(
  secret: Buffer,
  action: Record<string, unknown>,
  options: ActionCapabilityIssueOptions,
) {
  const mutableToken = mutableFieldToken(options.mutableFields || []);
  const metadata = [
    'v1',
    options.scope,
    mutableToken,
    Buffer.from(options.owner).toString('base64url'),
    options.ownerVersion,
    (options.issuedAt ?? Date.now()).toString(
      ACTION_CAPABILITY_TIMESTAMP_RADIX,
    ),
    randomBytes(ACTION_CAPABILITY_NONCE_BYTES).toString('base64url'),
  ].join('.');
  const mutableFields = mutableFieldsFromToken(mutableToken) as Set<string>;
  const digest = capabilityDigest(secret, metadata, action, mutableFields);
  return `${metadata}.${digest.toString('base64url')}`;
}

function actionCapabilityParts(executionId: string, scope: string) {
  const parts = executionId.split('.');
  if (
    parts.length !== ACTION_CAPABILITY_PARTS ||
    parts[0] !== 'v1' ||
    parts[1] !== scope
  ) {
    return null;
  }
  return parts;
}

function actionCapabilityDigestMatches(
  suppliedToken: string,
  expected: Buffer,
) {
  const supplied = Buffer.from(suppliedToken, 'base64url');
  return (
    supplied.length === expected.length && timingSafeEqual(supplied, expected)
  );
}

function actionCapabilityIsCurrent(
  parts: string[],
  maxAgeMs?: number,
  now = Date.now(),
) {
  if (maxAgeMs === undefined) {
    return true;
  }
  const issuedAt = Number.parseInt(parts[5], ACTION_CAPABILITY_TIMESTAMP_RADIX);
  return (
    Number.isSafeInteger(issuedAt) &&
    issuedAt <= now &&
    now - issuedAt <= maxAgeMs
  );
}

function resolveActionCapability(
  secret: Buffer,
  action: unknown,
  options: ActionCapabilityResolveOptions,
) {
  if (!(action && typeof action === 'object')) {
    return null;
  }
  const input = action as Record<string, unknown>;
  if (typeof input.executionId !== 'string') {
    return null;
  }
  const parts = actionCapabilityParts(input.executionId, options.scope);
  if (!parts) {
    return null;
  }
  if (!actionCapabilityIsCurrent(parts, options.maxAgeMs, options.now)) {
    return null;
  }
  const mutableFields = mutableFieldsFromToken(parts[2]);
  if (!mutableFields) {
    return null;
  }
  const owner = Buffer.from(parts[3], 'base64url').toString();
  if (!options.ownerIsCurrent(owner, parts[4])) {
    return null;
  }
  const metadata = parts.slice(0, ACTION_CAPABILITY_METADATA_PARTS).join('.');
  const expected = capabilityDigest(secret, metadata, input, mutableFields);
  if (!actionCapabilityDigestMatches(parts[7], expected)) {
    return null;
  }
  const { executionId: _executionId, ...trusted } = input;
  return structuredClone(trusted);
}

export function createActionExecutionCapabilities(
  secret = randomBytes(ACTION_CAPABILITY_SECRET_BYTES),
) {
  return {
    issue: (
      action: Record<string, unknown>,
      options: ActionCapabilityIssueOptions,
    ) => issueActionCapability(secret, action, options),
    resolve: (action: unknown, options: ActionCapabilityResolveOptions) =>
      resolveActionCapability(secret, action, options),
  };
}

export function mutableViewActionFields(action: Record<string, unknown>) {
  return [
    'formValues',
    'editorContent',
    'selectedItemId',
    'value',
    ...('text' in action ? [] : ['text']),
  ] as MutableActionField[];
}

export function currentActionExecutionRecord(
  executionId: unknown,
  records: Map<string, ActionExecutionRecord>,
  now: number,
  ttlMs: number,
) {
  if (typeof executionId !== 'string') {
    return null;
  }
  const record = records.get(executionId);
  if (!record) {
    return null;
  }
  if (now - record.createdAt <= ttlMs) {
    return record;
  }
  records.delete(executionId);
  return null;
}

export function actionFromExecutionRecord(
  action: unknown,
  records: Map<string, ActionExecutionRecord>,
  options: {
    actionName: (action: Record<string, unknown>) => string;
    now?: number;
    ttlMs: number;
  },
) {
  if (!(action && typeof action === 'object')) {
    throw new Error('Untrusted action');
  }
  const input = action as Record<string, unknown>;
  const record = currentActionExecutionRecord(
    input.executionId,
    records,
    options.now ?? Date.now(),
    options.ttlMs,
  );
  if (!record) {
    throw new Error(`Untrusted ${options.actionName(input)} action`);
  }
  const trusted = structuredClone(record.action) as Record<string, unknown>;
  return {
    ...trusted,
    ...(typeof input.traceId === 'string' ? { traceId: input.traceId } : {}),
  };
}

export function nativeActionFromExecutionRecord(
  action: unknown,
  rootRecords: Map<string, ActionExecutionRecord>,
  options: { now?: number; ttlMs: number },
) {
  if (!(action && typeof action === 'object')) {
    throw new Error('Untrusted nativeAction action');
  }
  const input = action as Record<string, unknown>;
  if (input.type !== 'nativeAction') {
    throw new Error('Untrusted nativeAction action');
  }
  const traceId = typeof input.traceId === 'string' ? input.traceId : undefined;
  const nativeInput = input.nativeAction;
  if (!(nativeInput && typeof nativeInput === 'object')) {
    throw new Error('Untrusted nativeAction action');
  }
  const nativeAction = actionFromExecutionRecord(
    { ...nativeInput, ...(traceId ? { traceId } : {}) },
    rootRecords,
    {
      actionName: (nested) => String(nested.kind || 'root'),
      now: options.now,
      ttlMs: options.ttlMs,
    },
  );
  return {
    type: 'nativeAction',
    title: input.title,
    nativeAction,
    ...(traceId ? { traceId } : {}),
  };
}

export type { ActionExecutionRecord, ActionExecutionScope };
