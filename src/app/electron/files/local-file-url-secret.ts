import crypto from 'node:crypto';
import fs from 'node:fs/promises';
import path from 'node:path';
import { configureLocalFileUrlSecret } from './file-utils';

export function createLocalFileUrlSecretLoader(deps: {
  userDataPath: () => string;
}) {
  return async function ensureLocalFileUrlSecret() {
    const secretPath = path.join(deps.userDataPath(), 'local-file-url-secret');
    const existing = (
      await fs.readFile(secretPath, 'utf8').catch(() => '')
    ).trim();
    const secret = existing || crypto.randomBytes(32).toString('base64url');
    if (!existing) {
      await fs.mkdir(path.dirname(secretPath), { recursive: true });
      await fs.writeFile(secretPath, secret, { mode: 0o600 });
    }
    configureLocalFileUrlSecret(secret);
  };
}
