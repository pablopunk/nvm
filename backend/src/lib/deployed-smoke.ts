type JsonRecord = Record<string, unknown>;
import { parsePublicOrigin } from '../../../src/shared/public-origin';

type CompatibilityManifest = {
  backend: { environment: string; version: string };
  api: { currentVersion: number; supportedVersions: number[] };
  desktop: {
    minimumSupportedVersion: string;
    latestVersion: string | null;
    updateUrl: string;
  };
  client: {
    compatible: boolean;
    unsupportedReason: string | null;
  };
};

export type DeployedSmokeResult = {
  target: string;
  health: { version: string; db: boolean; upstream: boolean };
  compatibility: {
    backendEnvironment: string;
    backendVersion: string;
    minimumDesktopVersion: string;
    supportedDesktopVersion: string;
    unsupportedDesktopVersion: string;
    unsupportedReason: string;
  };
};

const REQUEST_TIMEOUT_MS = 10_000;

export async function runDeployedSmoke(
  rawBaseUrl: string | undefined,
  fetchImplementation: typeof fetch = fetch,
): Promise<DeployedSmokeResult> {
  const target = normalizeSmokeTarget(rawBaseUrl);
  const health = await requestJson(
    target,
    '/api/health',
    {},
    fetchImplementation,
  );
  assertHealthContract(target, health);

  const discovery = assertCompatibilityContract(
    target,
    await requestJson(
      target,
      '/api/compatibility',
      {},
      fetchImplementation,
    ),
  );
  const supportedDesktopVersion = discovery.desktop.minimumSupportedVersion;
  const supportedHeaders = desktopHeaders(
    supportedDesktopVersion,
    discovery.api.currentVersion,
  );
  const supported = assertCompatibilityContract(
    target,
    await requestJson(
      target,
      '/api/compatibility',
      supportedHeaders,
      fetchImplementation,
    ),
  );
  if (!supported.client.compatible) {
    throw new Error(
      `${target}/api/compatibility rejected supported desktop ${supportedDesktopVersion}: ${supported.client.unsupportedReason || 'unknown reason'}`,
    );
  }

  const olderVersion = olderDesktopVersion(supportedDesktopVersion);
  const unsupportedDesktopVersion = olderVersion || supportedDesktopVersion;
  const unsupportedApiVersion = olderVersion
    ? discovery.api.currentVersion
    : firstUnsupportedApiVersion(discovery.api.supportedVersions);
  const unsupported = assertCompatibilityContract(
    target,
    await requestJson(
      target,
      '/api/compatibility',
      desktopHeaders(unsupportedDesktopVersion, unsupportedApiVersion),
      fetchImplementation,
    ),
  );
  if (unsupported.client.compatible || !unsupported.client.unsupportedReason) {
    const dimension = olderVersion
      ? `desktop ${unsupportedDesktopVersion}`
      : `API ${unsupportedApiVersion}`;
    throw new Error(
      `${target}/api/compatibility accepted deliberately unsupported ${dimension}`,
    );
  }

  return {
    target,
    health: {
      version: health.version,
      db: health.db,
      upstream: health.upstream,
    },
    compatibility: {
      backendEnvironment: discovery.backend.environment,
      backendVersion: discovery.backend.version,
      minimumDesktopVersion: discovery.desktop.minimumSupportedVersion,
      supportedDesktopVersion,
      unsupportedDesktopVersion,
      unsupportedReason: unsupported.client.unsupportedReason,
    },
  };
}

export function normalizeSmokeTarget(rawBaseUrl: string | undefined) {
  if (!rawBaseUrl?.trim()) {
    throw new Error('NVM_SMOKE_BASE_URL is required');
  }
  let parsed: URL;
  try {
    parsed = new URL(rawBaseUrl.trim());
  } catch {
    throw new Error('NVM_SMOKE_BASE_URL must be a valid absolute URL');
  }
  const isLoopback = ['localhost', '127.0.0.1', '::1'].includes(
    parsed.hostname,
  );
  if (parsed.protocol !== 'https:' && !(parsed.protocol === 'http:' && isLoopback)) {
    throw new Error(
      'NVM_SMOKE_BASE_URL must use HTTPS (HTTP is allowed only for a loopback fixture)',
    );
  }
  if (parsed.username || parsed.password) {
    throw new Error('NVM_SMOKE_BASE_URL must not include credentials');
  }
  if (parsed.search || parsed.hash) {
    throw new Error('NVM_SMOKE_BASE_URL must not include a query or fragment');
  }
  try {
    return parsePublicOrigin(rawBaseUrl.trim(), isLoopback ? 'local' : 'smoke');
  } catch (error) {
    throw new Error(`NVM_SMOKE_BASE_URL is invalid: ${error instanceof Error ? error.message : 'unknown error'}`);
  }
}

async function requestJson(
  target: string,
  pathname: string,
  headers: Record<string, string>,
  fetchImplementation: typeof fetch,
) {
  const url = `${target}${pathname}`;
  let response: Response;
  try {
    response = await fetchImplementation(url, {
      headers,
      redirect: 'manual',
      signal: AbortSignal.timeout(REQUEST_TIMEOUT_MS),
    });
  } catch (error) {
    throw new Error(
      `${url} request failed: ${error instanceof Error ? error.message : 'unknown network error'}`,
    );
  }
  const body = await response.text();
  if (response.status >= 300 && response.status < 400) {
    throw new Error(`${url} returned an unexpected redirect (${response.status})`);
  }
  if (!response.ok) {
    throw new Error(
      `${url} returned ${response.status}: ${redactResponseBody(body)}`,
    );
  }
  try {
    return JSON.parse(body) as JsonRecord;
  } catch {
    throw new Error(`${url} returned invalid JSON: ${redactResponseBody(body)}`);
  }
}

function assertHealthContract(
  target: string,
  value: JsonRecord,
): asserts value is JsonRecord & {
  ok: true;
  db: boolean;
  upstream: boolean;
  version: string;
} {
  if (
    value.ok !== true ||
    value.db !== true ||
    value.upstream !== true ||
    !isNonEmptyString(value.version)
  ) {
    throw new Error(
      `${target}/api/health returned an invalid health contract: ${redactResponseBody(JSON.stringify(value))}`,
    );
  }
}

function assertCompatibilityContract(
  target: string,
  value: JsonRecord,
): CompatibilityManifest {
  if (
    !isRecord(value.backend) ||
    !isNonEmptyString(value.backend.environment) ||
    !isNonEmptyString(value.backend.version) ||
    !isRecord(value.api) ||
    !Number.isInteger(value.api.currentVersion) ||
    !Array.isArray(value.api.supportedVersions) ||
    !value.api.supportedVersions.every((item) => Number.isInteger(item)) ||
    !isRecord(value.desktop) ||
    !isNonEmptyString(value.desktop.minimumSupportedVersion) ||
    !isNonEmptyString(value.desktop.updateUrl) ||
    !isRecord(value.client) ||
    typeof value.client.compatible !== 'boolean' ||
    !(
      value.client.unsupportedReason === null ||
      isNonEmptyString(value.client.unsupportedReason)
    )
  ) {
    throw new Error(
      `${target}/api/compatibility returned an invalid manifest: ${redactResponseBody(JSON.stringify(value))}`,
    );
  }
  return value as unknown as CompatibilityManifest;
}

function desktopHeaders(version: string, apiVersion: number) {
  return {
    'X-Nevermind-Client': 'desktop-smoke',
    'X-Nevermind-Client-Version': version,
    'X-Nevermind-API-Version': String(apiVersion),
    'X-Nevermind-Platform': process.platform,
    'X-Nevermind-Arch': process.arch,
  };
}

function olderDesktopVersion(version: string) {
  const parts = version
    .replace(/^v/i, '')
    .split(/[.-]/)
    .slice(0, 3)
    .map((part) => Number.parseInt(part, 10));
  if (parts.length === 0 || parts.some((part) => !Number.isInteger(part))) {
    return null;
  }
  while (parts.length < 3) parts.push(0);
  for (let index = parts.length - 1; index >= 0; index -= 1) {
    if (parts[index] > 0) {
      parts[index] -= 1;
      for (let suffix = index + 1; suffix < parts.length; suffix += 1) {
        parts[suffix] = 999;
      }
      return parts.join('.');
    }
  }
  return null;
}

function firstUnsupportedApiVersion(supportedVersions: number[]) {
  const supported = new Set(supportedVersions);
  let candidate = Math.max(0, ...supportedVersions) + 1;
  while (supported.has(candidate)) candidate += 1;
  return candidate;
}

function redactResponseBody(body: string) {
  return body
    .slice(0, 1_000)
    .replace(
      /("?(?:authorization|token|secret|api[_-]?key|cookie)"?\s*[:=]\s*")([^"]+)(")/gi,
      '$1[redacted]$3',
    )
    .replace(/Bearer\s+[A-Za-z0-9._~+/-]+=*/gi, 'Bearer [redacted]')
    .replace(
      /\beyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\b/g,
      '[redacted-jwt]',
    );
}

function isRecord(value: unknown): value is JsonRecord {
  return Boolean(value && typeof value === 'object' && !Array.isArray(value));
}

function isNonEmptyString(value: unknown): value is string {
  return typeof value === 'string' && value.trim().length > 0;
}
