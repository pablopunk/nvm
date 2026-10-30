// biome-ignore-all lint/style/useNamingConvention: PostHog event and exception properties are snake_case on the wire.
// biome-ignore-all lint/style/useExportsLast: Exports are grouped by concern.
import { randomUUID } from 'node:crypto';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { app } from 'electron';
import { PostHog } from 'posthog-node';
import { isNvmDiagnosticTestMode } from '../platform/test-mode-flags';
import { buildChannel, hashOwnerId } from './feature-usage';

const INSTALL_ID_FILE = 'analytics-id';
const TEST_PROJECT_KEY = 'phc_diagnostic_test';
const WITHDRAWN_RESPONSE = {
  status: 200,
  text: () => Promise.resolve('{}'),
  json: () => Promise.resolve({}),
};
let client: PostHog | null | undefined;
let currentUserId: () => string | undefined = () => undefined;
const reportedOwners = new Set<string>();
// Provisional until the persisted identifier loads; both are random UUIDs.
let installId: string = randomUUID();
let consentGranted = false;

export function configurePosthogIdentity(provider: () => string | undefined) {
  currentUserId = provider;
  loadInstallId();
}

function posthogConfig() {
  if (!(app.isPackaged || isNvmDiagnosticTestMode)) {
    return null;
  }
  const host = isNvmDiagnosticTestMode
    ? // biome-ignore lint/style/noProcessEnv: The loopback collector is only configurable at runtime in diagnostic test mode.
      process.env.POSTHOG_HOST_DESKTOP
    : import.meta.env.VITE_POSTHOG_HOST;
  const key = isNvmDiagnosticTestMode
    ? TEST_PROJECT_KEY
    : import.meta.env.VITE_POSTHOG_KEY;
  return key && host ? { key, host } : null;
}

// Mirrors the "Send Technical Diagnostics" setting for every capture path.
export function setPosthogConsent(granted: boolean) {
  consentGranted = granted;
}

export function posthogClient() {
  return consentGranted ? configuredClient() : null;
}

export async function flushPosthog() {
  await configuredClient()?.flush();
}

function configuredClient() {
  if (client !== undefined) {
    return client;
  }
  const config = posthogConfig();
  client = config
    ? new PostHog(config.key, {
        host: config.host,
        flushAt: 10,
        flushInterval: 10_000,
        // Bounded memory and bounded waiting: an unreachable endpoint must never
        // pile events up or hold the quit flush.
        maxQueueSize: 1_000,
        requestTimeout: 10_000,
        fetchRetryCount: 2,
        fetchRetryDelay: 3_000,
        // Queued batches must not outlive the user's decision to stop sending.
        fetch: (url, options) =>
          consentGranted
            ? fetch(url, options as RequestInit)
            : Promise.resolve(WITHDRAWN_RESPONSE),
      })
    : null;
  return client;
}

async function loadInstallId() {
  const file = path.join(app.getPath('userData'), INSTALL_ID_FILE);
  try {
    const stored = (await fs.readFile(file, 'utf8')).trim();
    if (stored) {
      installId = stored;
      return;
    }
  } catch {
    // First launch: no identifier has been written yet.
  }
  try {
    await fs.writeFile(file, installId, { mode: 0o600 });
  } catch {
    // An unwritable profile only makes this launch's identifier ephemeral.
  }
}

// One stable identifier per install, whether or not anyone is signed in, so a
// device's history never splits at sign-in. No event ever builds a person profile.
export function posthogDistinctId() {
  return installId;
}

export function commonEventProperties() {
  return {
    $process_person_profile: false,
    app_version: app.getVersion(),
    os_type: os.type(),
    os_release: os.release(),
    arch: process.arch,
    channel: buildChannel(app.getVersion()),
  };
}

// Who an install belongs to is its own fact, sent once per owner per run as a
// hashed account id, so analytics rules about owners can change after the fact.
function observeInstallOwner(client: PostHog) {
  const userId = currentUserId();
  if (!userId) {
    return;
  }
  const ownerId = hashOwnerId(userId);
  if (reportedOwners.has(ownerId)) {
    return;
  }
  reportedOwners.add(ownerId);
  client.capture({
    distinctId: installId,
    event: 'install.owner_observed',
    properties: {
      ...commonEventProperties(),
      owner_source: 'account',
      owner_id: ownerId,
    },
  });
}

export function captureDesktopEvent(
  event: string,
  properties: Record<string, unknown> = {},
) {
  const client = posthogClient();
  if (!client) {
    return;
  }
  observeInstallOwner(client);
  client.capture({
    distinctId: installId,
    event,
    properties: { ...commonEventProperties(), ...properties },
  });
}
