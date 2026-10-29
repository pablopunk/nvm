// biome-ignore-all lint: Moved verbatim from main.ts; keeps its established host-side conventions.

import {
  error as logError,
  info as logInfo,
  warn as logWarn,
} from '../platform/logger';
import {
  consumeDeviceCode,
  getDefaultNevermindBaseUrl,
  getNevermindAuth,
  isSigningIn,
  setActiveNevermindAuthBaseUrl,
} from './nevermind-auth';
import { warmNevermindCompatibilityCache } from './nevermind-compatibility';

export type ParsedAuthDeepLink = {
  code: string;
  baseUrl: string;
  intent: 'connect' | 'reconnect';
};

export type NevermindDeepLinksDeps = {
  parseAuthDeepLink: (
    url: string,
    activeBaseUrl: string,
  ) => ParsedAuthDeepLink | null;
  bufferedDeepLinks: string[];
  getActiveBaseUrl: () => string | null;
  setActiveBaseUrl: (baseUrl: string | null) => void;
  invalidateExtensionRootItems: () => void;
  broadcastAuthChanged: (status: {
    authed: boolean;
    userId?: string;
    email?: string;
    role?: string;
  }) => void;
};

export function createNevermindDeepLinks(deps: NevermindDeepLinksDeps) {
  const {
    parseAuthDeepLink,
    bufferedDeepLinks,
    getActiveBaseUrl,
    setActiveBaseUrl,
    invalidateExtensionRootItems,
    broadcastAuthChanged,
  } = deps;

  async function handleAuthDeepLink(parsed: ParsedAuthDeepLink) {
    const baseUrl = parsed.baseUrl || getDefaultNevermindBaseUrl();
    logInfo('deep_link.handle', {
      source: 'deep_link',
      baseUrl,
      intent: parsed.intent,
    });
    try {
      if (isSigningIn()) {
        logWarn('deep_link.signin_in_progress', undefined, {
          source: 'host',
          scope: 'deep_link',
        });
        return;
      }
      const existing = await getNevermindAuth();
      if (existing) {
        logInfo(
          'deep_link.already_authed',
          { email: existing.email },
          { source: 'host', scope: 'deep_link' },
        );
        return;
      }
      const result = await consumeDeviceCode({ code: parsed.code, baseUrl });
      if (result.ok) {
        setActiveBaseUrl(result.auth.baseUrl);
        setActiveNevermindAuthBaseUrl(result.auth.baseUrl);
        warmNevermindCompatibilityCache(result.auth.baseUrl);
        invalidateExtensionRootItems();
        broadcastAuthChanged({
          authed: true,
          userId: result.auth.userId,
          email: result.auth.email,
          role: result.auth.role,
        });
        logInfo(
          'deep_link.auth_success',
          { email: result.auth.email },
          { source: 'host', scope: 'deep_link' },
        );
      } else {
        logWarn(
          'deep_link.auth_failed',
          { reason: 'Device authorization was not completed' },
          { source: 'host', scope: 'deep_link' },
        );
      }
    } catch {
      logError('deep_link.handle.failed', undefined, {
        source: 'host',
        scope: 'deep_link',
      });
    }
  }

  function processBufferedDeepLink(rawUrl: string) {
    const baseUrl = getActiveBaseUrl() || getDefaultNevermindBaseUrl();
    const parsed = parseAuthDeepLink(rawUrl, baseUrl);
    if (!parsed) return;
    void handleAuthDeepLink(parsed);
  }

  function flushBufferedDeepLinks() {
    const links = bufferedDeepLinks.splice(0);
    for (const link of links) processBufferedDeepLink(link);
  }

  return {
    handleAuthDeepLink,
    processBufferedDeepLink,
    flushBufferedDeepLinks,
  };
}
