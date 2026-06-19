export type DataLoaderHandle = {
  _loader: true;
  _fn: () => Promise<any[]>;
  _retry: boolean;
};

type ViewLoaderEntry = { fn: () => Promise<any[]>; retry: boolean; entry: any };

export function createDataLoaderHandle(
  fn: () => Promise<any[]>,
  options: { retry?: boolean } = {},
): DataLoaderHandle {
  return { _loader: true, _fn: fn, _retry: Boolean(options.retry) };
}

export function isLoaderHandle(value: unknown): value is DataLoaderHandle {
  return Boolean(
    value &&
      typeof value === 'object' &&
      '_loader' in value &&
      typeof (value as Record<string, unknown>)._fn === 'function',
  );
}

export function normalizeLoaderItems(items: unknown) {
  return isLoaderHandle(items) ? [] : items;
}

export function resolveLoaderEmptyView(
  emptyView: any,
  loaderHandle?: DataLoaderHandle,
) {
  if (!loaderHandle || emptyView) return emptyView;
  return { title: 'No items', subtitle: '' };
}

export function createViewLoaderRegistry(deps: {
  sendHydrate: (viewId: string, payload: Record<string, unknown>) => void;
  normalizeItems: (items: any[], entry: any) => any[];
  warn?: (viewId: string, message: string) => void;
}) {
  const registry = new Map<string, ViewLoaderEntry>();

  async function spawn(viewId: string) {
    const loader = registry.get(viewId);
    if (!loader) return undefined;
    try {
      const items = await loader.fn();
      // Guard: skip if a newer loader was registered while we awaited
      if (registry.get(viewId) !== loader) return { ok: true as const, items };
      registry.delete(viewId);
      deps.sendHydrate(viewId, {
        items: Array.isArray(items)
          ? deps.normalizeItems(items, loader.entry)
          : [],
        isLoading: false,
      });
      return { ok: true as const, items };
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      // Guard: skip mutations if a newer loader was registered while we awaited
      if (registry.get(viewId) !== loader)
        return { ok: false as const, error: message, retry: loader.retry };
      if (!loader.retry) registry.delete(viewId);
      deps.sendHydrate(viewId, { error: { message }, retry: loader.retry });
      deps.warn?.(viewId, message);
      return { ok: false as const, error: message, retry: loader.retry };
    }
  }

  return {
    register(viewId: string, handle: DataLoaderHandle, entry: any) {
      registry.set(viewId, { fn: handle._fn, retry: handle._retry, entry });
    },
    has(viewId: string) {
      return registry.has(viewId);
    },
    retry(viewId: string) {
      return spawn(viewId);
    },
    spawn,
  };
}
