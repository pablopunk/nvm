type StateWriteGuard = <T>(write: () => T) => T;

let installedGuard: StateWriteGuard = (write) => write();

/**
 * Installs the gate that state maintenance uses to pause persisted writes that
 * start outside an interactive request, such as sign-in polling or transcription.
 */
export function installStateWriteGuard(guard: StateWriteGuard) {
  installedGuard = guard;
}

/** Persisted writes are asynchronous, so a paused guard rejects instead of throwing. */
export async function guardStateWrite<T>(
  write: () => Promise<T> | T,
): Promise<T> {
  return installedGuard(write);
}
