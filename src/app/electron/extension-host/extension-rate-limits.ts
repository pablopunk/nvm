// biome-ignore-all lint: Extension identity payloads stay dynamic at this host boundary.
const EXTENSION_REFRESH_MAX_BURST = 5;
const EXTENSION_REFRESH_BURST_WINDOW_MS = 2000;
const EXTENSION_AI_CALLS_PER_MINUTE = 30;
const EXTENSION_AI_RATE_WINDOW_MS = 60_000;

const extensionRefreshBurstWindow = new Map<string, number[]>();
const extensionAiCallWindow = new Map<string, number[]>();

export function checkRefreshBurst(
  extension: any,
  logWarn: (message: string, data: unknown, options: unknown) => void,
) {
  const id = extension?.id || 'unknown';
  const now = Date.now();
  const recent = (extensionRefreshBurstWindow.get(id) || []).filter(
    (time) => now - time < EXTENSION_REFRESH_BURST_WINDOW_MS,
  );
  if (recent.length >= EXTENSION_REFRESH_MAX_BURST) {
    logWarn(
      'extension.refresh.throttled',
      { count: recent.length },
      { source: 'host', scope: 'extension', extensionId: id },
    );
    return false;
  }
  recent.push(now);
  extensionRefreshBurstWindow.set(id, recent);
  return true;
}

export function checkAiRateLimit(extension: any) {
  const id = extension?.id || 'unknown';
  const now = Date.now();
  const recent = (extensionAiCallWindow.get(id) || []).filter(
    (time) => now - time < EXTENSION_AI_RATE_WINDOW_MS,
  );
  if (recent.length >= EXTENSION_AI_CALLS_PER_MINUTE) return false;
  recent.push(now);
  extensionAiCallWindow.set(id, recent);
  return true;
}
