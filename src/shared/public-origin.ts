export type PublicOriginPolicy =
  | 'production_api'
  | 'production_web'
  | 'preview'
  | 'local'
  | 'smoke';

export class PublicOriginError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'PublicOriginError';
  }
}

const LOOPBACK_HOSTS = new Set(['localhost', '127.0.0.1', '[::1]']);
const TRUSTED_PREVIEW_HOST =
  /^nvm-[a-z0-9-]+-pablo-varelas-projects-4f86af8b\.vercel\.app$/;

function rawPath(value: string): string {
  const schemeEnd = value.indexOf('://');
  const authorityEnd =
    schemeEnd < 0 ? -1 : value.slice(schemeEnd + 3).search(/[/?#]/);
  if (authorityEnd < 0) return '';
  const suffix = value.slice(schemeEnd + 3 + authorityEnd);
  const end = suffix.search(/[?#]/);
  return (end < 0 ? suffix : suffix.slice(0, end)) || '';
}

export function parsePublicOrigin(
  input: string,
  policy: PublicOriginPolicy,
  expectedPreviewOrigin?: string,
): string {
  const value = input.trim();
  let parsed: URL;
  try {
    parsed = new URL(value);
  } catch {
    throw new PublicOriginError('origin must be an absolute URL');
  }
  if (!['http:', 'https:'].includes(parsed.protocol)) {
    throw new PublicOriginError('origin must use HTTP or HTTPS');
  }
  if (parsed.username || parsed.password)
    throw new PublicOriginError('origin must not include credentials');
  if (
    parsed.search ||
    parsed.hash ||
    value.includes('?') ||
    value.includes('#')
  )
    throw new PublicOriginError('origin must not include a query or fragment');
  if (rawPath(value) !== '' && rawPath(value) !== '/') {
    throw new PublicOriginError('origin must not include a path');
  }
  if (parsed.pathname !== '/')
    throw new PublicOriginError('origin must use the root path');

  const host = parsed.hostname.toLowerCase();
  const isLoopback = LOOPBACK_HOSTS.has(host);
  if (policy === 'local') {
    if (!isLoopback || parsed.protocol !== 'http:')
      throw new PublicOriginError('local origin must be HTTP loopback');
  } else if (parsed.protocol !== 'https:') {
    throw new PublicOriginError(
      'production and Preview origins must use HTTPS',
    );
  }
  if (policy !== 'local' && parsed.port)
    throw new PublicOriginError(
      'production and Preview origins must use the default port',
    );
  if (policy === 'production_api' && host !== 'api.nvm.fyi')
    throw new PublicOriginError(
      'production API origin must be https://api.nvm.fyi',
    );
  if (policy === 'production_web' && host !== 'www.nvm.fyi')
    throw new PublicOriginError(
      'production web origin must be https://www.nvm.fyi',
    );
  if (policy === 'preview') {
    if (!TRUSTED_PREVIEW_HOST.test(host))
      throw new PublicOriginError('origin is not a trusted Preview deployment');
    if (!expectedPreviewOrigin)
      throw new PublicOriginError(
        'Preview origin requires an exact expected deployment origin',
      );
    const expected = parsePublicOrigin(expectedPreviewOrigin, 'smoke');
    if (parsed.origin !== expected)
      throw new PublicOriginError(
        'origin does not match the exact Preview deployment',
      );
  }
  return parsed.origin;
}

export function joinPublicApiUrl(origin: string, pathname: string): string {
  const base = parsePublicOrigin(origin, 'production_api');
  const path = pathname.startsWith('/') ? pathname : `/${pathname}`;
  if (!path.startsWith('/api/') && path !== '/api')
    throw new PublicOriginError('API path must start with /api');
  return `${base}${path}`;
}

export function migrateLegacyDesktopOrigin(input: string): string | null {
  const value = input.trim().replace(/\/+$/, '');
  if (
    /^https:\/\/(?:nvm\.fyi|www\.nvm\.fyi|api\.nvm\.fyi)(?:\/api)?$/i.test(
      value,
    )
  )
    return 'https://api.nvm.fyi';
  return null;
}
