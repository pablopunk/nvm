type StateWriteGuard = <T>(write: () => T) => T;

let installedGuard: StateWriteGuard = (write) => write();

/**
 * Installs the gate that state maintenance uses to pause persisted writes that
 * start outside an interactive request, such as sign-in polling or transcription.
 */
export function installStateWriteGuard(guard: StateWriteGuard) {
  installedGuard = guard;
}

export function guardStateWrite<T>(write: () => T): T {
  return installedGuard(write);
}
