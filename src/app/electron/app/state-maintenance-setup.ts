import path from 'node:path';
import { app } from 'electron';
import { defaultSharedStateRoot } from '../platform/shared-state-root';
import { STATE_COMPATIBILITY_EPOCH } from '../state/state-compatibility';
import {
  createStateMaintenanceService,
  type StateMaintenanceDeps,
} from './state-maintenance-service';

type AppStateMaintenanceInput = Pick<
  StateMaintenanceDeps,
  'writers' | 'flushPendingState' | 'isUpdateInstalling' | 'onChange'
> & { isTestMode: boolean };

export function createAppStateMaintenance(input: AppStateMaintenanceInput) {
  const sharedRoot = defaultSharedStateRoot();
  return createStateMaintenanceService({
    writers: input.writers,
    flushPendingState: input.flushPendingState,
    isUpdateInstalling: input.isUpdateInstalling,
    onChange: input.onChange,
    epoch: STATE_COMPATIBILITY_EPOCH,
    ownsSingleInstance: () => app.hasSingleInstanceLock(),
    roots: () =>
      input.isTestMode
        ? null
        : {
            privateRoot: app.getPath('userData'),
            sharedRoot,
            backupParent: path.join(
              path.dirname(sharedRoot),
              'nevermind-state-backups',
            ),
          },
  });
}
