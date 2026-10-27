import { recordWithoutPortableFields } from './portable-config-projection';
import { createUserStateSaveScheduler, writeUserStateFile } from './user-state';
import type { UserStateRecord } from './user-state-store';

interface UserStatePersistenceDeps {
  getUserState: () => UserStateRecord;
  getStatePath: () => string;
  beforeWrite: () => void;
  persistPortableConfig: (record: UserStateRecord) => Promise<void>;
  onSaveError: (error: unknown) => void;
}

function createUserStatePersistence(deps: UserStatePersistenceDeps) {
  const userStateSaveScheduler = createUserStateSaveScheduler({
    save: writeCurrentUserState,
    onSaveError: deps.onSaveError,
  });

  function scheduleSaveState() {
    userStateSaveScheduler.schedule();
  }

  async function saveUserState() {
    try {
      await persistUserState();
    } catch (error) {
      deps.onSaveError(error);
    }
  }

  function persistUserState() {
    userStateSaveScheduler.schedule();
    return userStateSaveScheduler.flushPendingSave();
  }

  async function writeCurrentUserState() {
    deps.beforeWrite();
    const record = deps.getUserState();
    await deps.persistPortableConfig(record);
    await writeUserStateFile(
      deps.getStatePath(),
      recordWithoutPortableFields(record),
    );
  }

  return {
    scheduleSaveState,
    saveUserState,
    persistUserState,
    flushPendingSave: () => userStateSaveScheduler.flushPendingSave(),
  };
}

export type { UserStatePersistenceDeps };
export { createUserStatePersistence };
