type Update = { apply: () => void };

/** Keep focused rows intact, and publish their new period only with the new rows. */
export function createDashboardTableUpdates() {
  const pending = new WeakMap<object, Update>();
  return {
    update(key: object, focused: boolean, apply: () => void): boolean {
      if (focused) { pending.set(key, { apply }); return false; }
      pending.delete(key);
      apply();
      return true;
    },
    flush(key: object, focused: boolean): void {
      const update = pending.get(key);
      if (!update || focused) return;
      pending.delete(key);
      update.apply();
    },
    cancel(key: object): void { pending.delete(key); },
  };
}
