// biome-ignore-all lint/style/useNamingConvention: PostHog event and exception properties are snake_case on the wire.
// biome-ignore-all lint/complexity/noVoid: Fire-and-forget loads must not block the renderer.
import posthog from 'posthog-js/dist/module.full.no-external';

// Unit tests import this module under plain Node, where Vite's env is absent.
const env = import.meta.env as ImportMetaEnv | undefined;
const posthogKey = env?.VITE_POSTHOG_KEY;
const posthogHost = env?.VITE_POSTHOG_HOST;
// Development builds stay local, matching the host process.
const posthogConfigured = Boolean(env?.PROD && posthogKey && posthogHost);
let consentEnabled = false;
let consentGeneration = 0;
let consentBroadcastVersion = 0;
let authVersion = 0;
let initialized = false;
let identifiedUserId: string | undefined;
let latestAuth: AuthStatus = { authed: false };

type AuthStatus = {
  authed: boolean;
  userId?: string;
  email?: string;
  role?: string;
};

function optInPosthog() {
  posthog.opt_in_capturing({ captureEventName: false });
}

// reset() clears consent along with the identity, so it only ever runs while
// consent is granted and is followed by a fresh opt-in.
function resetPosthogIdentity() {
  posthog.reset();
  optInPosthog();
  identifiedUserId = undefined;
}

function applyPosthogIdentity() {
  if (!consentEnabled) {
    return;
  }
  const { authed, userId, email, role } = latestAuth;
  if (!(authed && userId)) {
    if (identifiedUserId) {
      resetPosthogIdentity();
    }
    return;
  }
  if (identifiedUserId === userId) {
    return;
  }
  if (identifiedUserId) {
    resetPosthogIdentity();
  }
  posthog.identify(userId, {
    ...(email ? { email } : {}),
    ...(role ? { role } : {}),
  });
  identifiedUserId = userId;
}

function initPosthog(key: string) {
  posthog.init(key, {
    api_host: posthogHost,
    defaults: '2026-08-30',
    // No identity or consent is restored from browser storage across launches.
    persistence: 'memory',
    // reset() returns to this default, so a reset never re-enables capturing.
    opt_out_capturing_by_default: true,
    // Errors and identity only: element text can contain prompts or model output.
    autocapture: false,
    capture_pageview: false,
    capture_pageleave: false,
    capture_dead_clicks: false,
    capture_heatmaps: false,
    disable_session_recording: true,
    disable_surveys: true,
    disable_web_experiments: true,
    advanced_disable_flags: true,
    capture_exceptions: {
      capture_unhandled_errors: true,
      capture_unhandled_rejections: true,
      capture_console_errors: false,
    },
  });
  initialized = true;
}

function applyPosthogConsent(enabled: boolean) {
  if (enabled === consentEnabled) {
    return;
  }
  consentEnabled = enabled;
  consentGeneration += 1;
  if (enabled) {
    // The SDK stays unloaded until the host setting says it may send, so stored
    // browser consent from an earlier launch can never act on its own.
    if (!initialized && posthogKey) {
      initPosthog(posthogKey);
    }
    optInPosthog();
    applyPosthogIdentity();
    return;
  }
  // Opting out also drops the in-memory identity, so a later opt-in starts clean.
  if (initialized) {
    posthog.opt_out_capturing();
    posthog.reset();
  }
  identifiedUserId = undefined;
}

function applyAuthStatus(status: AuthStatus) {
  authVersion += 1;
  latestAuth = status;
  applyPosthogIdentity();
}

function onConsentBroadcast(enabled: boolean) {
  consentBroadcastVersion += 1;
  applyPosthogConsent(enabled);
}

async function syncPosthogConsent() {
  const versionAtRequest = consentBroadcastVersion;
  try {
    const enabled = (await window.nvm.getSetting('errorReporting')) === true;
    // A broadcast that arrived meanwhile is newer than this snapshot.
    if (consentBroadcastVersion === versionAtRequest) {
      applyPosthogConsent(enabled);
    }
  } catch {
    // Without a readable setting, capturing stays opted out.
  }
}

if (posthogConfigured) {
  window.nvm.onDiagnosticReportingChanged(onConsentBroadcast);
  window.nvm.onNevermindAuthChanged(applyAuthStatus);
  const authVersionAtRequest = authVersion;
  window.nvm
    .getNevermindAuthStatus()
    .then((status) => {
      // A broadcast that arrived meanwhile is newer than this snapshot.
      if (authVersion === authVersionAtRequest) {
        applyAuthStatus(status);
      }
    })
    .catch(() => undefined);
  void syncPosthogConsent();
}

export function posthogConsentGeneration() {
  return consentGeneration;
}

// A capture prepared under an earlier consent decision must not outlive it.
export function capturePosthogException(
  error: unknown,
  properties: Record<string, unknown>,
  generation: number,
) {
  if (initialized && consentEnabled && generation === consentGeneration) {
    posthog.captureException(error, properties);
  }
}
