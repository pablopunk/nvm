import { isDiagnosticId, isRequestId } from './diagnostics';

export function isDiagnosticBackend(url: string) {
  try {
    const parsed = new URL(url);
    return (
      parsed.protocol === 'https:' &&
      !parsed.username &&
      !parsed.password &&
      ['api.nvm.fyi', 'www.nvm.fyi'].includes(parsed.hostname) &&
      !parsed.port &&
      parsed.pathname.startsWith('/api/')
    );
  } catch {
    return false;
  }
}

export function backendDiagnosticHeaders(
  url: string,
  context?: {
    actionId: string;
    journeyId: string;
  },
) {
  const headers: Record<string, string> = {};
  if (!(isDiagnosticBackend(url) && context)) return headers;
  if (isDiagnosticId(context.actionId))
    headers['x-nevermind-action-id'] = context.actionId;
  if (isDiagnosticId(context.journeyId))
    headers['x-nevermind-journey-id'] = context.journeyId;
  return headers;
}

export function responseRequestId(headers: Headers | Record<string, string>) {
  const value =
    headers instanceof Headers
      ? headers.get('x-request-id')
      : headers['x-request-id'];
  return isRequestId(value) ? value : undefined;
}
