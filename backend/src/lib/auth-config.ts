import { env } from './env';
import { parsePublicOrigin, PublicOriginError } from '../../../src/shared/public-origin';

export class AuthConfigurationError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'AuthConfigurationError';
  }
}

export function productionWebOrigin(): string {
  const production = parsePublicOrigin(env('PRODUCTION_ORIGIN') ?? 'https://www.nvm.fyi', 'production_web');
  for (const aliasName of ['PUBLIC_DASHBOARD_URL', 'PREVIEW_GATEWAY_ORIGIN']) {
    const alias = env(aliasName);
    if (!alias || /^https?:\/\/localhost(?::\d+)?\/?$/.test(alias)) continue;
    if (parsePublicOrigin(alias, 'production_web') !== production) {
      throw new AuthConfigurationError(`${aliasName} does not match production canonical origin`);
    }
  }
  return production;
}

export function assertPreviewAuthConfiguration() {
  const required = ['DATABASE_URL', 'PREVIEW_GATEWAY_ORIGIN', 'PREVIEW_START_KEY', 'GATEWAY_STATE_KEY', 'GATEWAY_STATE_REDIS_URL', 'GATEWAY_STATE_REDIS_TOKEN', 'UPSTASH_REDIS_REST_URL', 'UPSTASH_REDIS_REST_TOKEN', 'PREVIEW_SESSION_KEY'];
  const missing = required.filter((key) => !env(key));
  if (missing.length) throw new AuthConfigurationError(`Preview auth configuration is incomplete: ${missing.join(',')}`);
  try {
    const production = productionWebOrigin();
    const gateway = parsePublicOrigin(env('PREVIEW_GATEWAY_ORIGIN') ?? production, 'production_web');
    if (gateway !== production) throw new AuthConfigurationError('Preview gateway origin is not production canonical');
  } catch (error) {
    if (error instanceof AuthConfigurationError) throw error;
    throw new AuthConfigurationError(error instanceof PublicOriginError ? error.message : 'Invalid public origin configuration');
  }
  if (env('PREVIEW_SESSION_KEY') === env('WORKOS_COOKIE_PASSWORD')) throw new AuthConfigurationError('Preview and production session keys must differ');
  if (env('GATEWAY_STATE_REDIS_TOKEN') === env('UPSTASH_REDIS_REST_TOKEN')) throw new AuthConfigurationError('Gateway and production Redis ACL identities must differ');
  return true;
}

export function previewAuthConfigured() {
  try { return assertPreviewAuthConfiguration(); } catch { return false; }
}

export function isProductionGatewayOrigin(origin: string) {
  try {
    return parsePublicOrigin(origin, 'production_web') === productionWebOrigin();
  } catch {
    return false;
  }
}
