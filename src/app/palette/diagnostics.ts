// biome-ignore-all lint/style/useNamingConvention: PostHog event and exception properties are snake_case on the wire.
// biome-ignore-all lint/complexity/noVoid: Fire-and-forget loads must not block the diagnostic path.
import { isDiagnosticId } from '../shared/diagnostics';
import { capturePosthogException, posthogConsentGeneration } from './posthog';

// posthog-js captures unhandled errors and rejections itself; this reports the
// failures that React boundaries and explicit catches convert to handled ones,
// tagged with the host-minted support reference.
export function captureRendererFailure(error: unknown) {
  if (!import.meta.env.PROD) {
    return;
  }
  const generation = posthogConsentGeneration();
  void window.nvm
    .prepareDiagnosticFailure()
    .catch(function missingFailureReference() {
      return null;
    })
    .then(function captureWithReference(reference) {
      capturePosthogException(
        error,
        {
          process: 'renderer',
          ...(isDiagnosticId(reference) ? { support_ref: reference } : {}),
        },
        generation,
      );
    });
}
