import { AsyncLocalStorage } from 'node:async_hooks';
import { PostHog } from 'posthog-node';
import { backendCommitSha } from './deployment-identity';

type RequestContext = Record<string, string | undefined>;
type EventProperties = Record<string, unknown>;

const BACKEND_DISTINCT_ID = 'nvm-backend';
const DEFAULT_POSTHOG_HOST = 'https://eu.i.posthog.com';
const requestContext = new AsyncLocalStorage<RequestContext>();
const posthog = createPosthogClient();

function createPosthogClient() {
  const key = process.env.POSTHOG_KEY;
  if (!key) return null;
  return new PostHog(key, {
    host: process.env.POSTHOG_HOST || DEFAULT_POSTHOG_HOST,
    // Serverless functions freeze after responding: send each event immediately.
    flushAt: 1,
    flushInterval: 0,
  });
}

function baseProperties(properties?: EventProperties): EventProperties {
  return {
    $process_person_profile: false,
    release: backendCommitSha() ?? process.env.VERCEL_GIT_COMMIT_SHA,
    environment: process.env.VERCEL_ENV ?? 'development',
    ...requestContext.getStore(),
    ...properties,
  };
}

export function runWithRequestContext<T>(
  context: RequestContext,
  task: () => T,
): T {
  return requestContext.run({ ...context }, task);
}

export function setRequestContext(values: RequestContext) {
  const store = requestContext.getStore();
  if (store) Object.assign(store, values);
}

export function captureBackendException(
  error: unknown,
  properties?: EventProperties,
) {
  posthog?.captureException(
    error,
    BACKEND_DISTINCT_ID,
    baseProperties(properties),
  );
}

// Operational alerts (failed cron checks, exhausted provider chains) are plain
// events so PostHog alerts and dashboards can filter on `alert`.
export function captureBackendAlert(
  alert: string,
  properties?: EventProperties,
) {
  posthog?.capture({
    distinctId: BACKEND_DISTINCT_ID,
    event: 'backend_alert',
    properties: baseProperties({ alert, ...properties }),
  });
}

export function captureBackendRequest({
  durationMs,
  ...request
}: {
  route: string;
  method: string;
  status: number;
  durationMs: number;
}) {
  posthog?.capture({
    distinctId: BACKEND_DISTINCT_ID,
    event: 'backend_request',
    properties: baseProperties({ ...request, duration_ms: durationMs }),
  });
}

export async function flushErrorTracking() {
  await posthog?.flush();
}
