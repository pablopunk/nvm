export function diagnosticTestCollectorOrigin(dsn: string | undefined) {
  const url = dsn ? new URL(dsn) : undefined;
  if (
    !url ||
    url.protocol !== 'http:' ||
    url.hostname !== '127.0.0.1' ||
    !url.port ||
    url.pathname !== '/1' ||
    url.username !== 'qa' ||
    url.password ||
    url.search ||
    url.hash
  )
    throw new Error('Diagnostic tests require a loopback-only collector DSN');
  return url.origin;
}

export function isDiagnosticTestCollectorRequest(
  value: string,
  origin: string | undefined,
) {
  if (!origin) return false;
  const url = new URL(value);
  return url.origin === origin && url.pathname === '/api/1/envelope/';
}
