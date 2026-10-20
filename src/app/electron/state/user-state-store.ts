// biome-ignore lint/suspicious/noExplicitAny: user state is a loosely shaped persisted document read by property across the host
type UserStateRecord = Record<string, any>;

function createUserStateStore(initial: UserStateRecord) {
  let current = initial;
  let loadedStatePath: string | null = null;

  return {
    get: (): UserStateRecord => current,
    replace(next: UserStateRecord) {
      current = next;
    },
    /** Called once after a valid state file was read; never reset by later failures. */
    markLoadedFromDisk(statePath: string) {
      loadedStatePath = statePath;
    },
    /** Path of the state file this process started from, or null when the load was not trusted. */
    loadedStatePath: (): string | null => loadedStatePath,
  };
}

type UserStateStore = ReturnType<typeof createUserStateStore>;

export type { UserStateRecord, UserStateStore };
export { createUserStateStore };
