import {
  parsePublicOrigin,
  PRODUCTION_WEB_ORIGIN,
  PublicOriginError,
} from '../../../src/shared/public-origin';
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

function isDeployedRuntime() {
  const vercelEnvironment = env('VERCEL_ENV');
  if (vercelEnvironment) {
    return vercelEnvironment === 'production' || vercelEnvironment === 'preview';
  }
  return env('NODE_ENV') === 'production';
}

function isLoopbackHost(hostname: string) {
  return ['localhost', '127.0.0.1', '::1'].includes(hostname);
}

export function productionWebOrigin(): string {
  try {
    const configuredProduction = configuredValue('PRODUCTION_ORIGIN');
    if (!configuredProduction) {
      throw new AuthConfigurationError('PRODUCTION_ORIGIN is required');
    }
    const production = parsePublicOrigin(configuredProduction, 'production_web');
    for (const aliasName of ['PUBLIC_DASHBOARD_URL', 'PREVIEW_GATEWAY_ORIGIN']) {
      const alias = configuredValue(aliasName);
      if (!alias || /^https?:\/\/localhost(?::\d+)?\/?$/i.test(alias)) continue;
      if (parsePublicOrigin(alias, 'production_web') !== production) {
        throw new AuthConfigurationError(
          `${aliasName} does not match production canonical origin`,
        );
      }
    }
    return production;
  } catch (error) {
    if (error instanceof AuthConfigurationError) throw error;
    throw new AuthConfigurationError(
      error instanceof PublicOriginError
        ? error.message
        : 'Invalid production origin configuration',
    );
  }
}

export function resolveAuthRedirectConfiguration(): {
  productionOrigin: string;
  redirectUri: string;
} {
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

  const productionOrigin = productionWebOrigin();
  if (productionOrigin !== PRODUCTION_WEB_ORIGIN) {
    throw new AuthConfigurationError('Production origin is not canonical');
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
  const localCallback =
    redirect.protocol === 'http:' && isLoopbackHost(redirect.hostname);
  if (localCallback && !isDeployedRuntime()) {
    return { productionOrigin, redirectUri: redirect.toString() };
  }
  if (
    redirect.protocol !== 'https:' ||
    redirect.origin !== productionOrigin ||
    redirect.toString() !== `${PRODUCTION_WEB_ORIGIN}/api/auth/callback`
  ) {
    throw new AuthConfigurationError(
      'WORKOS_REDIRECT_URI must use the canonical HTTPS production origin',
    );
  }
  return { productionOrigin, redirectUri: redirect.toString() };
}

export function assertProductionAuthConfiguration() {
  resolveAuthRedirectConfiguration();
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
  const required = [
    'DATABASE_URL',
    'PREVIEW_GATEWAY_ORIGIN',
    'PREVIEW_START_KEY',
    'GATEWAY_STATE_KEY',
    'GATEWAY_STATE_REDIS_URL',
    'GATEWAY_STATE_REDIS_TOKEN',
    'UPSTASH_REDIS_REST_URL',
    'UPSTASH_REDIS_REST_TOKEN',
    'PREVIEW_SESSION_KEY',
  ];
  const missing = required.filter((key) => !env(key));
  if (missing.length) {
    throw new AuthConfigurationError(
      `Preview auth configuration is incomplete: ${missing.join(',')}`,
    );
  }
  try {
    const production = productionWebOrigin();
    const gateway = parsePublicOrigin(
      env('PREVIEW_GATEWAY_ORIGIN') ?? production,
      'production_web',
    );
    if (gateway !== production) {
      throw new AuthConfigurationError(
        'Preview gateway origin is not production canonical',
      );
    }
  } catch (error) {
    if (error instanceof AuthConfigurationError) throw error;
    throw new AuthConfigurationError(
      error instanceof PublicOriginError
        ? error.message
        : 'Invalid public origin configuration',
    );
  }
  if (env('PREVIEW_SESSION_KEY') === env('WORKOS_COOKIE_PASSWORD')) {
    throw new AuthConfigurationError(
      'Preview and production session keys must differ',
    );
  }
  if (env('GATEWAY_STATE_REDIS_TOKEN') === env('UPSTASH_REDIS_REST_TOKEN')) {
    throw new AuthConfigurationError(
      'Gateway and production Redis ACL identities must differ',
    );
  }
  return true;
}

export function previewAuthConfigured() {
  try {
    return assertPreviewAuthConfiguration();
  } catch {
    return false;
  }
}

export function isProductionGatewayOrigin(origin: string) {
  try {
    return parsePublicOrigin(origin, 'production_web') === productionWebOrigin();
  } catch {
    return false;
  }
}
