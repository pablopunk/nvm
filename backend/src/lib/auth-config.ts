import { env } from './env';

export class AuthConfigurationError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'AuthConfigurationError';
  }
}

export function assertPreviewAuthConfiguration() {
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
