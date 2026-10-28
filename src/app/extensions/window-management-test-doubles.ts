// biome-ignore-all lint/suspicious/noExplicitAny: fake extension contexts mirror the untyped runtime bridge.

function recordingCall(
  calls: unknown[][],
  implementation: ((...args: unknown[]) => Promise<unknown>) | undefined,
) {
  return (...args: unknown[]) => {
    calls.push(args);
    return (implementation ?? (() => Promise.resolve(undefined)))(...args);
  };
}

function fakeCapabilities(hasWindowManagement: boolean) {
  return {
    capabilities: {
      has: (capability: string) =>
        hasWindowManagement && capability === 'window-management',
    },
  };
}

function fakeDesktop(
  options: FakeWindowContextOptions,
  apply: unknown,
  appleScript: unknown,
) {
  return {
    ...(options.withWindowManagementFacet === false
      ? {}
      : { windowManagement: { apply } }),
    shell: { appleScript },
  };
}

function fakeWindowUi(indicators: any[]) {
  return {
    indicator: {
      show: (input: any) => {
        indicators.push(input);
      },
      hide: () => undefined,
    },
    item: (item: any) => item,
    list: (view: any) => ({ type: 'list', ...view }),
    form: (view: any) => ({ type: 'form', ...view }),
  };
}

function fakeWindowActions(
  invalidations: string[],
  contributionRefreshCalls: unknown[][],
  refreshImplementation: FakeWindowContextOptions['refreshContributions'],
) {
  return {
    input: {
      prompt: (input: any) => ({ type: 'prompt-action', ...input }),
    },
    actions: {
      run: (title: string, handler: any, options: any = {}) => ({
        title,
        handler,
        ...options,
      }),
      refreshContributions: recordingCall(
        contributionRefreshCalls,
        refreshImplementation ?? (() => Promise.resolve({ ok: true })),
      ),
    },
    views: {
      refresh: () => ({ type: 'refresh-action' }),
      invalidate: () => {
        invalidations.push('root-items');
      },
    },
  };
}

export function memoryExtensionStorage(initial: Record<string, unknown> = {}) {
  const values = new Map<string, unknown>(Object.entries(initial));
  return {
    values,
    get: (key: string, fallback?: unknown) =>
      Promise.resolve(values.has(key) ? values.get(key) : fallback),
    set: (key: string, value: unknown) => {
      values.set(key, value);
      return Promise.resolve(value);
    },
    delete: (key: string) => {
      values.delete(key);
      return Promise.resolve();
    },
    clear: () => {
      values.clear();
      return Promise.resolve();
    },
  };
}

export interface FakeWindowContextOptions {
  storage?: Record<string, unknown>;
  apply?: (...args: unknown[]) => Promise<unknown>;
  appleScript?: (...args: unknown[]) => Promise<unknown>;
  withWindowManagementFacet?: boolean;
  withWindowManagementCapability?: boolean;
  refreshContributions?: (...args: unknown[]) => Promise<unknown>;
}

export function fakeWindowManagementContext(
  options: FakeWindowContextOptions = {},
) {
  const indicators: any[] = [];
  const logs: any[] = [];
  const applyCalls: unknown[][] = [];
  const appleScriptCalls: unknown[][] = [];
  const invalidations: string[] = [];
  const contributionRefreshCalls: unknown[][] = [];
  const storage = memoryExtensionStorage(options.storage);
  const ctx = {
    storage,
    system: fakeCapabilities(options.withWindowManagementCapability !== false),
    desktop: fakeDesktop(
      options,
      recordingCall(applyCalls, options.apply),
      recordingCall(appleScriptCalls, options.appleScript),
    ),
    logs: {
      error: (message: string, data?: unknown) =>
        logs.push({ level: 'error', message, data }),
      info: () => undefined,
    },
    ui: fakeWindowUi(indicators),
    ...fakeWindowActions(
      invalidations,
      contributionRefreshCalls,
      options.refreshContributions,
    ),
  };
  return {
    ctx: ctx as any,
    storage,
    indicators,
    applyCalls,
    appleScriptCalls,
    logs,
    invalidations,
    contributionRefreshCalls,
  };
}
