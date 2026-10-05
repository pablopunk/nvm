import fs from 'node:fs/promises';
import path from 'node:path';

export async function auditLegacyDiagnosticCache(userData: string) {
  const cache = path.join(userData, 'sentry');
  const knownPaths = ['queue/queue-v2.json', 'scope_v3.json'];
  const present = await Promise.all(
    knownPaths.map(async function legacyCachePresent(relativePath) {
      try {
        return (await fs.stat(path.join(cache, relativePath))).isFile();
      } catch {
        return false;
      }
    }),
  );
  return present.some(Boolean);
}
