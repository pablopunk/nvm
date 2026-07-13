import { env } from './env';

export class AuthConfigurationError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'AuthConfigurationError';
  }
}

export function assertPreviewDatabaseBinding() {
  const databaseUrl = env('DATABASE_URL');
  const previewDatabaseUrl = env('PREVIEW_DATABASE_URL');
  const productionDatabaseUrl = env('PRODUCTION_DATABASE_URL');
  if (!databaseUrl || !previewDatabaseUrl || !productionDatabaseUrl) {
    throw new AuthConfigurationError('Preview database binding is incomplete');
  }
  if (databaseUrl !== previewDatabaseUrl || databaseUrl === productionDatabaseUrl) {
    throw new AuthConfigurationError('Preview runtime database must be the isolated Preview database');
  }
  return true;
}

export function assertPreviewAuthConfiguration() {
  const required = ['PREVIEW_GATEWAY_ORIGIN', 'PREVIEW_START_KEY', 'GATEWAY_STATE_KEY', 'GATEWAY_STATE_REDIS_URL', 'GATEWAY_STATE_REDIS_TOKEN', 'PREVIEW_GRANT_REDIS_URL', 'PREVIEW_GRANT_REDIS_TOKEN', 'PREVIEW_REDIS_URL', 'PREVIEW_REDIS_TOKEN', 'PREVIEW_SESSION_KEY'];
  const missing = required.filter((key) => !env(key));
  if (missing.length) throw new AuthConfigurationError(`Preview auth configuration is incomplete: ${missing.join(',')}`);
  if (env('PREVIEW_GATEWAY_ORIGIN') !== 'https://nvm.fyi') throw new AuthConfigurationError('Preview gateway origin is not production canonical');
  if (env('PREVIEW_SESSION_KEY') === env('WORKOS_COOKIE_PASSWORD')) throw new AuthConfigurationError('Preview and production session keys must differ');
  assertPreviewDatabaseBinding();
  if (env('PREVIEW_REDIS_URL') === env('GATEWAY_STATE_REDIS_URL') || env('PREVIEW_GRANT_REDIS_URL') === env('GATEWAY_STATE_REDIS_URL')) throw new AuthConfigurationError('Preview and gateway Redis endpoints must differ');
  const redisTokens = ['GATEWAY_STATE_REDIS_TOKEN', 'PREVIEW_GRANT_REDIS_TOKEN', 'PREVIEW_REDIS_TOKEN'].map((key) => env(key));
  if (new Set(redisTokens).size !== redisTokens.length) throw new AuthConfigurationError('Preview Redis ACL identities must be pairwise distinct');
  if (env('WORKOS_PREVIEW_CLIENT_ID') && env('WORKOS_PREVIEW_CLIENT_ID') === env('WORKOS_CLIENT_ID')) throw new AuthConfigurationError('Preview and production WorkOS clients must differ');
  return true;
}

export function previewAuthConfigured() {
  try { return assertPreviewAuthConfiguration(); } catch { return false; }
}
