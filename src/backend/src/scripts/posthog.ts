import posthog from 'posthog-js';

type PosthogPageConfig = { key: string; autocapture: boolean };

// The server renders the project key into the page; with no key (previews,
// local development) analytics stay off.
function readConfig(): PosthogPageConfig | null {
  const element = document.getElementById('posthog-config');
  if (!element?.textContent) return null;
  try {
    return JSON.parse(element.textContent) as PosthogPageConfig;
  } catch {
    return null;
  }
}

const config = readConfig();
if (config) {
  posthog.init(config.key, {
    // Same-origin proxy (vercel.json rewrites) so blockers do not drop events.
    api_host: '/ingest',
    ui_host: 'https://eu.posthog.com',
    defaults: '2026-08-30',
    // No cookies or browser storage: nothing to consent to.
    cookieless_mode: 'always',
    person_profiles: 'identified_only',
    autocapture: config.autocapture,
    capture_pageview: 'history_change',
    capture_pageleave: true,
    capture_exceptions: true,
    disable_session_recording: true,
    disable_surveys: true,
  });
}
