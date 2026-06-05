import { randomUUID } from 'node:crypto';

export const DESKTOP_API_VERSION = 1;
export const SUPPORTED_API_VERSIONS = [1] as const;

export type DesktopClient = {
  name: string | null;
  version: string | null;
  apiVersion: number | null;
  platform: string | null;
  arch: string | null;
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

export function compatibilityManifestForRequest(request: Request): CompatibilityManifest {
  const client = desktopClientFromRequest(request);
  const unsupportedReason = unsupportedClientReason(client);
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
    features: {},
    notices: [],
  };
}

export function compatibilityHeaders(requestId?: string) {
  const headers = new Headers();
  headers.set('x-nevermind-backend-version', backendVersion());
  if (requestId) headers.set('x-request-id', requestId);
  return headers;
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
