import type { MaintenanceActivity } from '../platform/maintenance-activity';

type PendingViewLoaderDeps = {
  registry: { has: (id: string) => boolean; spawn: (id: string) => unknown };
  trace: {
    run: (
      name: string,
      attributes: Record<string, unknown>,
      task: () => unknown,
      context?: { traceId: string },
    ) => unknown;
  };
  activity: Pick<MaintenanceActivity, 'run'>;
};

export function createPendingViewLoaderSpawner(deps: PendingViewLoaderDeps) {
  return function spawnPendingViewLoaders(
    result: { view?: { id?: string } } | null,
    traceId?: string,
  ) {
    const viewId = result?.view?.id;
    if (!viewId || !deps.registry.has(viewId)) return;
    try {
      void deps.activity
        .run(() =>
          Promise.resolve(
            deps.trace.run(
              'view.loader.spawn',
              { viewId },
              () => deps.registry.spawn(viewId),
              traceId ? { traceId } : undefined,
            ),
          ),
        )
        .catch(() => {});
    } catch {
      // A maintenance barrier may close while the result is being prepared.
    }
  };
}
