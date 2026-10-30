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
let initialized = false;

function initPosthog(key: string) {
  posthog.init(key, {
    api_host: posthogHost,
    defaults: '2026-08-30',
    // No identity or consent is restored from browser storage across launches.
    persistence: 'memory',
    // Exceptions leave the renderer anonymously, with no person profile. The host
    // process owns the install identity; a `support_ref` property joins the two.
    person_profiles: 'identified_only',
    // reset() returns to this default, so a reset never re-enables capturing.
    opt_out_capturing_by_default: true,
    // Errors only: element text can contain prompts or model output.
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
    posthog.opt_in_capturing({ captureEventName: false });
    return;
  }
  if (initialized) {
    posthog.opt_out_capturing();
    posthog.reset();
  }
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
