// biome-ignore-all lint/suspicious/noExplicitAny: fake extension contexts mirror the untyped runtime bridge.
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
  withWindowManagementFacet?: boolean;
}

export function fakeWindowManagementContext(
  options: FakeWindowContextOptions = {},
) {
  const indicators: any[] = [];
  const applyCalls: unknown[][] = [];
  const storage = memoryExtensionStorage(options.storage);
  const apply = (...args: unknown[]) => {
    applyCalls.push(args);
    return (options.apply ?? (() => Promise.resolve(undefined)))(...args);
  };
  const ctx = {
    storage,
    desktop:
      options.withWindowManagementFacet === false
        ? {}
        : { windowManagement: { apply } },
    ui: {
      indicator: {
        show: (input: any) => {
          indicators.push(input);
        },
        hide: () => undefined,
      },
      item: (item: any) => item,
      list: (view: any) => ({ type: 'list', ...view }),
    },
    input: {
      prompt: (input: any) => ({ type: 'prompt-action', ...input }),
    },
    actions: {
      run: (title: string, handler: any) => ({ title, handler }),
    },
    views: { refresh: () => ({ type: 'refresh-action' }) },
  };
  return { ctx: ctx as any, storage, indicators, applyCalls };
}
