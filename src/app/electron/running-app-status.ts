export type RunningAppCandidate = { id?: string; path?: string; name?: string };

type RunningSnapshot = { updatedAt: number; paths: Set<string> };

type MeasureAsync = <T>(
  name: string,
  metadata: Record<string, unknown>,
  work: () => Promise<T>,
) => Promise<T>;
type Mark = (name: string, metadata?: Record<string, unknown>) => void;

export type RunningAppStatusService = {
  getForRenderer: (appPaths: unknown) => Promise<string[]>;
  invalidate: () => void;
  scheduleRefresh: (reason: string) => void;
  refresh: (reason: string) => Promise<Set<string>>;
};

export type RunningAppStatusServiceOptions = {
  ttlMs: number;
  platform?: NodeJS.Platform;
  now?: () => number;
  getCandidates: () => RunningAppCandidate[];
  detectRunningAppPaths: (
    candidates: RunningAppCandidate[],
  ) => Promise<Set<string>>;
  notifyChanged: () => void;
  measure?: MeasureAsync;
  mark?: Mark;
  onRefreshFailed?: (error: unknown) => void;
};

const defaultMeasure: MeasureAsync = (_name, _metadata, work) => work();
const defaultMark: Mark = () => {};

function normalizedRunningPath(value: unknown, platform: NodeJS.Platform) {
  const text = String(value || '').trim();
  return platform === 'darwin' || platform === 'win32'
    ? text.toLowerCase()
    : text;
}

function sameRunningPathSet(a: Set<string> | undefined, b: Set<string>) {
  if (!a || a.size !== b.size) return false;
  for (const item of a) if (!b.has(item)) return false;
  return true;
}

function normalizedRunningPathSet(
  paths: Set<string>,
  platform: NodeJS.Platform,
) {
  return new Set(
    Array.from(paths)
      .map((item) => normalizedRunningPath(item, platform))
      .filter(Boolean),
  );
}

function rendererRequestedPaths(appPaths: unknown) {
  return Array.from(
    new Set(
      (Array.isArray(appPaths) ? appPaths : [])
        .map((item) => String(item || '').trim())
        .filter(Boolean),
    ),
  ).slice(0, 30);
}

export function createRunningAppStatusService(
  options: RunningAppStatusServiceOptions,
): RunningAppStatusService {
  const platform = options.platform || process.platform;
  const now = options.now || Date.now;
  const measure = options.measure || defaultMeasure;
  const mark = options.mark || defaultMark;
  let snapshot: RunningSnapshot | null = null;
  let refreshPromise: Promise<Set<string>> | null = null;

  function snapshotIsFresh() {
    return Boolean(snapshot && now() - snapshot.updatedAt < options.ttlMs);
  }

  function refresh(reason: string) {
    if (refreshPromise) return refreshPromise;
    const previousPaths = snapshot?.paths;
    const candidates = options.getCandidates();
    refreshPromise = measure(
      'apps.running.snapshot',
      { indexedCount: candidates.length, reason, alwaysLog: true },
      async () => {
        const paths = normalizedRunningPathSet(
          await options.detectRunningAppPaths(candidates),
          platform,
        );
        snapshot = { updatedAt: now(), paths };
        mark('apps.running.snapshot.result', { count: paths.size, reason });
        if (!sameRunningPathSet(previousPaths, paths)) options.notifyChanged();
        return paths;
      },
    ).finally(() => {
      refreshPromise = null;
    });
    return refreshPromise;
  }

  function scheduleRefresh(reason: string) {
    if (snapshotIsFresh() || refreshPromise) return;
    void refresh(reason).catch((error) => options.onRefreshFailed?.(error));
  }

  async function getForRenderer(appPaths: unknown) {
    return measure(
      'apps.running.get',
      {
        requestedCount: Array.isArray(appPaths) ? appPaths.length : 0,
        cached: Boolean(snapshot),
        alwaysLog: true,
      },
      async () => {
        const requestedPaths = rendererRequestedPaths(appPaths);
        if (!requestedPaths.length) return [];
        scheduleRefresh('renderer-request');
        const runningPaths = snapshot?.paths || new Set<string>();
        return requestedPaths.filter((appPath) =>
          runningPaths.has(normalizedRunningPath(appPath, platform)),
        );
      },
    );
  }

  return {
    getForRenderer,
    invalidate() {
      snapshot = null;
    },
    scheduleRefresh,
    refresh,
  };
}
