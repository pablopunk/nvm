import { createHash, randomUUID } from 'node:crypto';
import { log } from './log';

export const DESKTOP_API_VERSION = 1;
export const SUPPORTED_API_VERSIONS = [1] as const;

export type DesktopClient = {
  name: string | null;
  version: string | null;
  apiVersion: number | null;
  platform: string | null;
  arch: string | null;
};

const DEFAULT_FEATURES: Record<string, FeatureFlagRule> = {
  active_model_descriptor: true,
  proxy_streaming: true,
};

type FeatureFlagRule = boolean | {
  enabled?: boolean;
  minDesktopVersion?: string;
  maxDesktopVersion?: string;
  users?: string[];
  plans?: string[];
  rolloutPercent?: number;
};

export type FeatureFlagContext = {
  userId?: string | null;
  plan?: string | null;
  requestId?: string | null;
  route?: string;
};

export type CompatibilityManifest = {
  backend: {
    version: string;
    environment: string;
  };
  api: {
    currentVersion: number;
    supportedVersions: number[];
  };
  desktop: {
    minimumSupportedVersion: string;
    latestVersion: string | null;
    updateUrl: string;
    supportPolicy: string;
  };
  client: DesktopClient & {
    compatible: boolean;
    unsupportedReason: string | null;
  };
  features: Record<string, boolean>;
  notices: Array<{ type: 'info' | 'warning' | 'force_update'; message: string }>;
};

const DEFAULT_UPDATE_URL = 'https://github.com/pablopunk/nvm/releases/latest';
const SUPPORT_POLICY = 'latest_two_minor_versions_or_90_days';

export function backendVersion() {
  return process.env.VERCEL_GIT_COMMIT_SHA?.slice(0, 7) || 'dev';
}

export function backendEnvironment() {
  return process.env.VERCEL_ENV || process.env.NODE_ENV || 'development';
}

export function minimumSupportedDesktopVersion() {
  return process.env.NEVERMIND_MIN_DESKTOP_VERSION || '0.0.0';
}

export function latestDesktopVersion() {
  return process.env.NEVERMIND_LATEST_DESKTOP_VERSION || null;
}

export function desktopUpdateUrl() {
  return process.env.NEVERMIND_DESKTOP_UPDATE_URL || DEFAULT_UPDATE_URL;
}

export function requestIdFromHeaders(headers: Headers) {
  return headers.get('x-request-id') || headers.get('x-nevermind-request-id') || randomUUID();
}

export function desktopClientFromRequest(request: Request): DesktopClient {
  return {
    name: blankToNull(request.headers.get('x-nevermind-client')),
    version: blankToNull(request.headers.get('x-nevermind-client-version')),
    apiVersion: parsePositiveInteger(request.headers.get('x-nevermind-api-version')),
    platform: blankToNull(request.headers.get('x-nevermind-platform')),
    arch: blankToNull(request.headers.get('x-nevermind-arch')),
  };
}

export function compatibilityManifestForRequest(request: Request, context: FeatureFlagContext = {}): CompatibilityManifest {
  const client = desktopClientFromRequest(request);
  const unsupportedReason = unsupportedClientReason(client);
  const features = compatibilityFeaturesForClient(client, context);
  logFeatureEvaluations(features, client, context);
  return {
    backend: {
      version: backendVersion(),
      environment: backendEnvironment(),
    },
    api: {
      currentVersion: DESKTOP_API_VERSION,
      supportedVersions: [...SUPPORTED_API_VERSIONS],
    },
    desktop: {
      minimumSupportedVersion: minimumSupportedDesktopVersion(),
      latestVersion: latestDesktopVersion(),
      updateUrl: desktopUpdateUrl(),
      supportPolicy: SUPPORT_POLICY,
    },
    client: {
      ...client,
      compatible: !unsupportedReason,
      unsupportedReason,
    },
    features,
    notices: [],
  };
}

export function compatibilityHeaders(requestId?: string) {
  const headers = new Headers();
  headers.set('x-nevermind-backend-version', backendVersion());
  if (requestId) headers.set('x-request-id', requestId);
  return headers;
}

export function compatibilityFeaturesForClient(client: DesktopClient, context: FeatureFlagContext = {}) {
  const definitions = featureFlagDefinitions();
  return Object.fromEntries(Object.entries(definitions).map(([name, rule]) => [name, evaluateFeatureFlag(name, rule, client, context)]));
}

export function backendKillSwitchEnabled(name: string) {
  const raw = process.env.NEVERMIND_KILL_SWITCHES?.trim();
  if (!raw) return false;
  if (!raw.startsWith('{')) return raw.split(',').map((value) => value.trim()).includes(name);
  try {
    const parsed = JSON.parse(raw) as Record<string, boolean>;
    return parsed?.[name] === true;
  } catch (error) {
    log.warn('kill_switches_parse_failed', { error });
    return false;
  }
}

export function killSwitchResponse(name: string, message: string, requestId?: string) {
  log.warn('kill_switch_triggered', { kill_switch: name, request_id: requestId });
  return Response.json(
    { error: { type: 'service_unavailable', message } },
    { status: 503, headers: compatibilityHeaders(requestId) },
  );
}

export function unsupportedClientReason(client: DesktopClient) {
  if (client.apiVersion !== null && !SUPPORTED_API_VERSIONS.includes(client.apiVersion as 1)) return 'unsupported_api_version';
  if (client.version && compareVersions(client.version, minimumSupportedDesktopVersion()) < 0) return 'unsupported_desktop_version';
  return null;
}

export function compatibilityError(request: Request, message = 'This version of Nevermind is no longer supported.') {
  const requestId = requestIdFromHeaders(request.headers);
  return Response.json(
    {
      error: {
        type: 'unsupported_client',
        message,
        minimum_supported_desktop_version: minimumSupportedDesktopVersion(),
        latest_desktop_version: latestDesktopVersion(),
        update_url: desktopUpdateUrl(),
        request_id: requestId,
      },
    },
    { status: 426, headers: compatibilityHeaders(requestId) },
  );
}

export function compareVersions(left: string, right: string) {
  const leftParts = versionParts(left);
  const rightParts = versionParts(right);
  for (let index = 0; index < Math.max(leftParts.length, rightParts.length); index += 1) {
    const diff = (leftParts[index] || 0) - (rightParts[index] || 0);
    if (diff !== 0) return diff > 0 ? 1 : -1;
  }
  return 0;
}

function featureFlagDefinitions(): Record<string, FeatureFlagRule> {
  const raw = process.env.NEVERMIND_FEATURE_FLAGS?.trim();
  if (!raw) return DEFAULT_FEATURES;
  if (!raw.startsWith('{')) {
    const envFeatures = Object.fromEntries(raw.split(',').map((name) => [name.trim(), true]).filter(([name]) => Boolean(name)));
    return { ...DEFAULT_FEATURES, ...envFeatures };
  }
  try {
    const parsed = JSON.parse(raw) as Record<string, FeatureFlagRule>;
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? { ...DEFAULT_FEATURES, ...parsed } : DEFAULT_FEATURES;
  } catch (error) {
    log.warn('feature_flags_parse_failed', { error });
    return DEFAULT_FEATURES;
  }
}

function evaluateFeatureFlag(name: string, rule: FeatureFlagRule, client: DesktopClient, context: FeatureFlagContext) {
  if (typeof rule === 'boolean') return rule;
  if (!rule || rule.enabled === false) return false;
  if (rule.minDesktopVersion && (!client.version || compareVersions(client.version, rule.minDesktopVersion) < 0)) return false;
  if (rule.maxDesktopVersion && (!client.version || compareVersions(client.version, rule.maxDesktopVersion) > 0)) return false;
  if (rule.users?.length && (!context.userId || !rule.users.includes(context.userId))) return false;
  if (rule.plans?.length && (!context.plan || !rule.plans.includes(context.plan))) return false;
  if (typeof rule.rolloutPercent === 'number' && rolloutBucket(name, client, context) >= Math.max(0, Math.min(100, rule.rolloutPercent))) return false;
  return true;
}

function rolloutBucket(name: string, client: DesktopClient, context: FeatureFlagContext) {
  const key = [name, context.userId, context.plan, client.name, client.version, client.platform, client.arch].filter(Boolean).join(':') || name;
  const hex = createHash('sha256').update(key).digest('hex').slice(0, 8);
  return Number.parseInt(hex, 16) % 100;
}

function logFeatureEvaluations(features: Record<string, boolean>, client: DesktopClient, context: FeatureFlagContext) {
  if (!Object.keys(features).length) return;
  log.info('feature_flags_evaluated', {
    request_id: context.requestId || undefined,
    route: context.route || 'compatibility',
    user_id: context.userId || undefined,
    plan: context.plan || undefined,
    client_name: client.name,
    client_version: client.version,
    client_api_version: client.apiVersion,
    features,
  });
}

function versionParts(version: string) {
  return String(version || '')
    .replace(/^v/i, '')
    .split(/[.-]/)
    .map((part) => Number.parseInt(part, 10))
    .filter((part) => Number.isFinite(part));
}

function blankToNull(value: string | null) {
  const trimmed = value?.trim();
  return trimmed ? trimmed : null;
}

function parsePositiveInteger(value: string | null) {
  const parsed = Number.parseInt(value || '', 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : null;
}
