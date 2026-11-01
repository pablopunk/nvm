import os from 'node:os';
import path from 'node:path';
import { app } from 'electron';

/** Per-user cache directory following each platform's convention. */
export function osCacheRoot() {
  const appName = app.getName();
  if (process.platform === 'darwin')
    return path.join(os.homedir(), 'Library', 'Caches', appName);
  if (process.platform === 'win32')
    return path.join(
      process.env.LOCALAPPDATA || app.getPath('appData'),
      appName,
      'Cache',
    );
  return path.join(
    process.env.XDG_CACHE_HOME || path.join(os.homedir(), '.cache'),
    appName,
  );
}
