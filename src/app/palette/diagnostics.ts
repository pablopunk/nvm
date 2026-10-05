import * as Sentry from '@sentry/electron/renderer';
import { RENDERER_DIAGNOSTIC_INTEGRATIONS } from '../shared/diagnostic-privacy';
import { isDiagnosticId } from '../shared/diagnostics';

let initialized = false;

export function initRendererDiagnostics() {
  if (initialized || !import.meta.env.PROD) return;
  try {
    Sentry.init({
      sendDefaultPii: false,
      sendClientReports: false,
      enableLogs: false,
      autoSessionTracking: false,
      beforeBreadcrumb: () => null,
      integrations: (defaults) =>
        defaults.filter((integration) =>
          RENDERER_DIAGNOSTIC_INTEGRATIONS.has(integration.name),
        ),
    });
    initialized = true;
  } catch {}
}

export function captureRendererFailure(error: unknown) {
  if (!initialized) return;
  void window.nvm
    .prepareDiagnosticFailure()
    .then(function captureWithReference(reference) {
      Sentry.captureException(error, {
        contexts: isDiagnosticId(reference)
          ? { diagnostic: { failure_handle: reference } }
          : {},
      });
    })
    .catch(function captureWithoutReference() {
      Sentry.captureException(error);
    });
}
