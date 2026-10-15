// biome-ignore lint/suspicious/noExplicitAny: user state is a loosely shaped persisted document read by property across the host
type UserStateRecord = Record<string, any>;

function createUserStateStore(initial: UserStateRecord) {
  let current = initial;

  return {
    get: (): UserStateRecord => current,
    replace(next: UserStateRecord) {
      current = next;
    },
  };
}

type UserStateStore = ReturnType<typeof createUserStateStore>;

export type { UserStateRecord, UserStateStore };
export { createUserStateStore };
