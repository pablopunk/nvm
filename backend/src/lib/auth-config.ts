import { env } from './env';

export class AuthConfigurationError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'AuthConfigurationError';
  }
}

const PRODUCTION_AUTH_KEYS = [
  'WORKOS_API_KEY',
  'WORKOS_CLIENT_ID',
  'WORKOS_REDIRECT_URI',
  'WORKOS_COOKIE_PASSWORD',
] as const;

function configuredValue(name: string) {
  return env(name)?.trim() ?? '';
}

function productionWorkosCallbackOrigin() {
  try {
    return new URL(env('PRODUCTION_ORIGIN') ?? 'https://nvm.fyi').origin;
  } catch {
    throw new AuthConfigurationError(
      'PRODUCTION_ORIGIN must be a valid absolute URL',
    );
  }
}

function isDeployedRuntime() {
  const vercelEnvironment = env('VERCEL_ENV');
  if (vercelEnvironment) {
    return vercelEnvironment === 'production' || vercelEnvironment === 'preview';
  }
  return env('NODE_ENV') === 'production';
}

export function assertProductionAuthConfiguration() {
  const missing = PRODUCTION_AUTH_KEYS.filter((key) => !configuredValue(key));
  if (missing.length) {
    throw new AuthConfigurationError(
      `Production auth configuration is incomplete: ${missing.join(',')}`,
    );
  }
  const padded = PRODUCTION_AUTH_KEYS.filter(
    (key) => env(key) !== configuredValue(key),
  );
  if (padded.length) {
    throw new AuthConfigurationError(
      `Production auth configuration contains surrounding whitespace: ${padded.join(',')}`,
    );
  }

  if (configuredValue('WORKOS_COOKIE_PASSWORD').length < 32) {
    throw new AuthConfigurationError(
      'WORKOS_COOKIE_PASSWORD must contain at least 32 characters',
    );
  }

  let redirect: URL;
  try {
    redirect = new URL(configuredValue('WORKOS_REDIRECT_URI'));
  } catch {
    throw new AuthConfigurationError(
      'WORKOS_REDIRECT_URI must be a valid absolute URL',
    );
  }
  if (
    redirect.pathname !== '/api/auth/callback' ||
    redirect.search ||
    redirect.hash ||
    redirect.username ||
    redirect.password
  ) {
    throw new AuthConfigurationError(
      'WORKOS_REDIRECT_URI must point directly to /api/auth/callback',
    );
  }
  const isLoopback = ['localhost', '127.0.0.1', '::1'].includes(
    redirect.hostname,
  );
  if (
    (redirect.protocol !== 'https:' &&
      !(redirect.protocol === 'http:' && isLoopback && !isDeployedRuntime())) ||
    (isDeployedRuntime() && redirect.origin !== productionWorkosCallbackOrigin())
  ) {
    throw new AuthConfigurationError(
      'WORKOS_REDIRECT_URI must use the canonical HTTPS production origin',
    );
  }
  return true;
}

export function productionAuthConfigured() {
  try {
    return assertProductionAuthConfiguration();
  } catch {
    return false;
  }
}

export function assertPreviewAuthConfiguration() {
  assertProductionAuthConfiguration();
  const required = ['DATABASE_URL', 'PREVIEW_GATEWAY_ORIGIN', 'PREVIEW_START_KEY', 'GATEWAY_STATE_KEY', 'GATEWAY_STATE_REDIS_URL', 'GATEWAY_STATE_REDIS_TOKEN', 'UPSTASH_REDIS_REST_URL', 'UPSTASH_REDIS_REST_TOKEN', 'PREVIEW_SESSION_KEY'];
  const missing = required.filter((key) => !env(key));
  if (missing.length) throw new AuthConfigurationError(`Preview auth configuration is incomplete: ${missing.join(',')}`);
  if (env('PREVIEW_GATEWAY_ORIGIN') !== 'https://nvm.fyi') throw new AuthConfigurationError('Preview gateway origin is not production canonical');
  if (env('PREVIEW_SESSION_KEY') === env('WORKOS_COOKIE_PASSWORD')) throw new AuthConfigurationError('Preview and production session keys must differ');
  if (env('GATEWAY_STATE_REDIS_TOKEN') === env('UPSTASH_REDIS_REST_TOKEN')) throw new AuthConfigurationError('Gateway and production Redis ACL identities must differ');
  return true;
}

export function previewAuthConfigured() {
  try { return assertPreviewAuthConfiguration(); } catch { return false; }
}

export function isProductionGatewayOrigin(origin: string) {
  const configuredOrigin = env('PRODUCTION_ORIGIN') ?? 'https://nvm.fyi';
  return origin === configuredOrigin || (configuredOrigin === 'https://nvm.fyi' && origin === 'https://www.nvm.fyi');
}
