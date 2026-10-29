import path from 'node:path';
import { app } from 'electron';
import type { MaintenanceActivity } from '../platform/maintenance-activity';
import { defaultSharedStateRoot } from '../platform/shared-state-root';
import {
  freezeStateWrites,
  installStateWriteGuard,
} from '../platform/state-write-guard';
import { STATE_COMPATIBILITY_EPOCH } from '../state/state-compatibility';
import {
  createStateMaintenanceService,
  type StateMaintenanceDeps,
} from './state-maintenance-service';

type AppStateMaintenanceInput = Pick<
  StateMaintenanceDeps,
  'flushPendingState'
> & {
  isTestMode: boolean;
  /** Pauses interactive work and guards persisted writes that start elsewhere. */
  activity: MaintenanceActivity;
  writers: StateMaintenanceDeps['writers'];
  updateManager: { state: { installInFlight: boolean } };
};

export function createAppStateMaintenance(input: AppStateMaintenanceInput) {
  const sharedRoot = defaultSharedStateRoot();
  installStateWriteGuard(input.activity.run);
  return createStateMaintenanceService({
    writers: [input.activity, ...input.writers],
    flushPendingState: input.flushPendingState,
    freezeStateWrites,
    isUpdateInstalling: () => input.updateManager.state.installInFlight,
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
