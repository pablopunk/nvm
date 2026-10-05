import * as Sentry from '@sentry/astro';
import { sanitizeBackendEvent } from './lib/diagnostics';

Sentry.init({
  dsn: process.env.SENTRY_DSN,
  release: process.env.VERCEL_GIT_COMMIT_SHA,
  environment: process.env.VERCEL_ENV ?? 'development',
  tracesSampleRate: 0.1,
  sendDefaultPii: false,
  sendClientReports: false,
  enableLogs: false,
  tracePropagationTargets: [],
  integrations: defaults => defaults.filter(integration => !['Console', 'ConsoleLogs', 'ContextLines', 'LocalVariables', 'RequestData', 'Undici'].includes(integration.name)),
  beforeBreadcrumb: () => null,
  beforeSend: sanitizeBackendEvent,
  beforeSendTransaction: sanitizeBackendEvent,
});
