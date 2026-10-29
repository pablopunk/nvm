export function createMaintenanceActivity() {
  const active = new Set<Promise<unknown>>();
  let paused = false;
  let draining = false;

  function run<T>(operation: () => T): T {
    if (paused) throw new Error('Nevermind is preparing a state backup');
    const result = operation();
    if (result && typeof (result as { then?: unknown }).then === 'function') {
      const promise = Promise.resolve(result);
      active.add(promise);
      const remove = () => active.delete(promise);
      void promise.then(remove, remove);
    }
    return result;
  }

  async function quiesce() {
    if (paused) throw new Error('Maintenance is already in progress');
    paused = true;
    draining = true;
    try {
      while (active.size) await Promise.allSettled([...active]);
    } finally {
      draining = false;
    }
  }

  function resume() {
    if (draining) throw new Error('Interactive work is still draining');
    paused = false;
  }

  return { run, quiesce, resume };
}

export type MaintenanceActivity = ReturnType<typeof createMaintenanceActivity>;
