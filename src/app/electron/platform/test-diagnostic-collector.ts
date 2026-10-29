export function diagnosticTestCollectorOrigin(host: string | undefined) {
  const url = host ? new URL(host) : undefined;
  if (
    url?.protocol !== 'http:' ||
    url.hostname !== '127.0.0.1' ||
    !url.port ||
    url.pathname !== '/' ||
    url.username ||
    url.password ||
    url.search ||
    url.hash
  ) {
    throw new Error('Diagnostic tests require a loopback-only collector host');
  }
  return url.origin;
}

export function isDiagnosticTestCollectorRequest(
  value: string,
  origin: string | undefined,
) {
  if (!origin) {
    return false;
  }
  return new URL(value).origin === origin;
}
