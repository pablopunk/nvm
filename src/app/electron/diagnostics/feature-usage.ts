import { createHash } from 'node:crypto';
import type {
  DiagnosticComponent,
  DiagnosticOperation,
} from '../../shared/diagnostics';

export type FeatureUsage = { feature: string; variant?: string };

type FeatureContext = {
  operation: DiagnosticOperation;
  outcome: string;
  label?: string;
  component?: DiagnosticComponent;
};

// Operations that start because a person did something. Everything else
// (extension loads, update polling, background jobs, backend requests) is
// housekeeping that must never count as activity.
const USER_FEATURES: Partial<Record<DiagnosticOperation, string>> = {
  'command.execute': 'command.run',
  'view.action': 'view.action',
  'ai.stream': 'ai.chat',
  'extension.generate': 'extension.generate',
  'extension.install': 'extension.install',
  'auth.signin': 'auth.signin',
  'update.install': 'update.install',
};

const SAFE_ID = /^[a-z0-9][a-z0-9._:-]{0,63}$/i;
const USER_EXTENSION_LABEL = 'user-extension';

function safeId(value: unknown) {
  return typeof value === 'string' && SAFE_ID.test(value) ? value : undefined;
}

/**
 * The canonical "someone used the app" fact. Variants come only from closed
 * sets: action kinds and built-in command ids from this repository, or the
 * diagnostic component enum. User extension names and ids never appear.
 */
export function featureUsage(
  context: FeatureContext,
): FeatureUsage | undefined {
  const feature = USER_FEATURES[context.operation];
  if (!feature || context.outcome === 'cancelled') return undefined;
  const variant = context.label ?? context.component;
  return variant ? { feature, variant } : { feature };
}

/**
 * Names the action that ran. Only built-in extensions may contribute their own
 * ids; anything user-authored collapses to one fixed bucket.
 */
export function actionFeatureLabel(
  action: { kind?: unknown; extensionId?: unknown; commandId?: unknown } | null,
  builtIn: boolean,
) {
  const kind = safeId(action?.kind) ?? 'unknown';
  if (!builtIn) return action?.extensionId ? USER_EXTENSION_LABEL : kind;
  return [kind, safeId(action?.extensionId), safeId(action?.commandId)]
    .filter(Boolean)
    .join(':');
}

/** One-way identifier for an account, so analytics can group installs by owner. */
export function hashOwnerId(value: string) {
  return createHash('sha256').update(value).digest('hex');
}

// Stable releases are 0.x; dev and nightly builds are CalVer (2026.1030.911).
// The build's own channel is what separates internal builds from real users.
export function buildChannel(version: string) {
  return Number(version.split('.')[0]) >= 2000 ? 'prerelease' : 'stable';
}

// Background jobs tick every second or so; only their failures are worth an event.
export function isRoutineBackgroundSuccess(context: FeatureContext) {
  return context.operation === 'job.run' && context.outcome === 'success';
}
