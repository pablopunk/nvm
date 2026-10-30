import os from 'node:os';
import path from 'node:path';

export function defaultFileIndexRoots() {
  return [
    'Desktop',
    'Documents',
    'Downloads',
    'Pictures',
    'Videos',
    'Movies',
  ].map((name) => path.join(os.homedir(), name));
}
