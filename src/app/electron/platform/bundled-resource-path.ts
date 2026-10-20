import fs from 'node:fs/promises';
import path from 'node:path';
import { app } from 'electron';

/** Resolves a bundled resource across dev, asar and unpacked layouts. */
export async function bundledResourcePath(...relativePath: string[]) {
  const candidates = [
    path.join(app.getAppPath(), 'src', 'app', 'resources', ...relativePath),
    path.join(
      process.resourcesPath,
      'app.asar',
      'src',
      'app',
      'resources',
      ...relativePath,
    ),
    path.join(
      process.resourcesPath,
      'src',
      'app',
      'resources',
      ...relativePath,
    ),
  ];
  for (const candidate of candidates) {
    try {
      await fs.access(candidate);
      return candidate;
    } catch {}
  }
  return candidates[0];
}
