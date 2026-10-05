import { RENDERER_DIAGNOSTIC_INTEGRATIONS } from '../shared/diagnostic-privacy';
import { isDiagnosticId } from '../shared/diagnostics';

type RendererSdk = typeof import('@sentry/electron/renderer');
type PendingFailure = { error: unknown; reference: Promise<string | null> };
let sdk: RendererSdk | undefined;
let scheduled = false;
const pendingFailures: PendingFailure[] = [];

export function initRendererDiagnostics() {
  if (scheduled || !import.meta.env.PROD) return;
  scheduled = true;
  window.addEventListener('error', captureEarlyError);
  window.addEventListener('unhandledrejection', captureEarlyRejection);
  requestAnimationFrame(function afterRendererFirstFrame() {
    setTimeout(loadRendererSdk, 0);
  });
}

function captureEarlyError(event: ErrorEvent) {
  if (event.error) captureRendererFailure(event.error);
}

function captureEarlyRejection(event: PromiseRejectionEvent) {
  captureRendererFailure(event.reason);
}

async function loadRendererSdk() {
  try {
    const Sentry = await import('@sentry/electron/renderer');
    Sentry.init({
      sendDefaultPii: false,
      sendClientReports: false,
      enableLogs: false,
      beforeBreadcrumb: () => null,
      integrations: (defaults) =>
        defaults.filter((integration) =>
          RENDERER_DIAGNOSTIC_INTEGRATIONS.has(integration.name),
        ),
    });
    sdk = Sentry;
    window.removeEventListener('error', captureEarlyError);
    window.removeEventListener('unhandledrejection', captureEarlyRejection);
    for (const failure of pendingFailures.splice(0))
      deliverRendererFailure(failure);
  } catch {}
}

export function captureRendererFailure(error: unknown) {
  if (!import.meta.env.PROD) return;
  const reference = window.nvm
    .prepareDiagnosticFailure()
    .catch(function missingFailureReference() {
      return null;
    });
  const failure = { error, reference };
  if (sdk) deliverRendererFailure(failure);
  else if (pendingFailures.length < 20) pendingFailures.push(failure);
}

function deliverRendererFailure(failure: PendingFailure) {
  void failure.reference
    .then(function captureWithReference(reference) {
      sdk?.captureException(failure.error, {
        contexts: isDiagnosticId(reference)
          ? { diagnostic: { failure_handle: reference } }
          : {},
      });
    })
    .catch(function ignoreSdkFailure() {});
}
