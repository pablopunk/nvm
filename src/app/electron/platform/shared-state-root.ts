import os from 'node:os';
import path from 'node:path';

/** Per-user state shared by every installed Nevermind build, regardless of channel. */
export function defaultSharedStateRoot() {
  return path.join(os.homedir(), '.config', 'nevermind');
}
