const DEEP_LINK_SCHEME = 'nvm';
const AUTH_HOST = 'auth';

type ParsedAuthDeepLink = {
  code: string;
  baseUrl: string;
  intent: 'connect' | 'reconnect';
};

const PRODUCTION_BASE_URL = 'https://api.nvm.fyi';
const LOCAL_BASE_URL = 'http://localhost:4321';
const PAT_PREFIX = 'nvm_';
const JWT_PATTERN = /^eyJ/;
import {
  migrateLegacyDesktopOrigin,
  parsePublicOrigin,
} from '../shared/public-origin';

let logWarn: (message: string, data?: unknown) => void = () => {};

function setDeepLinkLogger(logger: {
  warn: (message: string, data?: unknown) => void;
}) {
  logWarn = logger.warn.bind(logger);
}

function isPatLike(code: string): boolean {
  return code.startsWith(PAT_PREFIX) || JWT_PATTERN.test(code);
}

function encodedOrigin(baseUrl: string): string {
  try {
    return encodeURIComponent(new URL(baseUrl).origin);
  } catch {
    return '';
  }
}

function buildAuthDeepLinkUrl(
  code: string,
  baseUrl: string,
  mode: 'connect' | 'reconnect',
): string {
  const params = new URLSearchParams();
  params.set('code', code);
  params.set('base_url', encodedOrigin(baseUrl));
  if (mode === 'reconnect') params.set('mode', 'reconnect');
  return `${DEEP_LINK_SCHEME}://${AUTH_HOST}?${params.toString()}`;
}

function parseAuthDeepLink(
  url: string,
  activeBaseUrl: string,
): ParsedAuthDeepLink | null {
  let parsed: URL;
  try {
    parsed = new URL(url);
  } catch {
    return null;
  }
  if (
    parsed.protocol !== `${DEEP_LINK_SCHEME}:` ||
    parsed.hostname !== AUTH_HOST
  ) {
    return null;
  }
  const code = parsed.searchParams.get('code');
  if (!code) return null;
  if (isPatLike(code)) {
    logWarn('deep_link_pat_rejected');
    return null;
  }
  const encodedBaseUrl = parsed.searchParams.get('base_url');
  const rawMode = parsed.searchParams.get('mode') || '';
  const intent = rawMode === 'reconnect' ? 'reconnect' : 'connect';

  let resolvedBaseUrl = activeBaseUrl;
  if (encodedBaseUrl) {
    let decoded = '';
    try {
      decoded = decodeURIComponent(encodedBaseUrl).replace(/\/$/, '');
    } catch {
      logWarn('deep_link_unparsable_base_url', { encoded: encodedBaseUrl });
    }
    if (decoded) {
      const migrated = migrateLegacyDesktopOrigin(decoded);
      if (migrated) decoded = migrated;
      const allowedOrigins = [PRODUCTION_BASE_URL, LOCAL_BASE_URL];
      if (
        (allowedOrigins.includes(decoded) &&
          (() => {
            try {
              parsePublicOrigin(
                decoded,
                decoded === LOCAL_BASE_URL ? 'local' : 'production_api',
              );
              return true;
            } catch {
              return false;
            }
          })()) ||
        decoded === activeBaseUrl.replace(/\/$/, '')
      ) {
        resolvedBaseUrl = decoded;
      } else {
        logWarn('deep_link_untrusted_base_url', {
          decoded,
          active: activeBaseUrl,
        });
      }
    }
  }
  return { code, baseUrl: resolvedBaseUrl, intent };
}

export {
  DEEP_LINK_SCHEME,
  parseAuthDeepLink,
  buildAuthDeepLinkUrl,
  isPatLike,
  setDeepLinkLogger,
  type ParsedAuthDeepLink,
};
