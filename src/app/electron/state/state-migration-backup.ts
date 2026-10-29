import crypto from 'node:crypto';
import { createReadStream } from 'node:fs';
import fs from 'node:fs/promises';
import path from 'node:path';
import type { SharedStateLease } from '../platform/shared-state-lease';

type BackupRoot = { name: 'private' | 'shared'; path: string };
type BackupEntry = {
  path: string;
  kind: 'directory' | 'file';
  sha256?: string;
};

async function hashFile(file: string) {
  const hash = crypto.createHash('sha256');
  for await (const chunk of createReadStream(file)) hash.update(chunk);
  return hash.digest('hex');
}

async function inventory(directory: string): Promise<BackupEntry[]> {
  const entries: BackupEntry[] = [];
  async function visit(relative: string) {
    const current = path.join(directory, relative);
    const stat = await fs.lstat(current);
    if (stat.isSymbolicLink())
      throw new Error('Backup contains a symbolic link');
    if (stat.isDirectory()) {
      if (relative) entries.push({ path: relative, kind: 'directory' });
      for (const child of (await fs.readdir(current)).sort())
        await visit(path.join(relative, child));
      return;
    }
    if (!stat.isFile())
      throw new Error('Backup contains an unsupported file type');
    entries.push({
      path: relative,
      kind: 'file',
      sha256: await hashFile(current),
    });
  }
  await visit('');
  return entries;
}

function pathsOverlap(left: string, right: string) {
  const relative = path.relative(left, right);
  return (
    relative === '' ||
    (!relative.startsWith(`..${path.sep}`) &&
      relative !== '..' &&
      !path.isAbsolute(relative))
  );
}

export async function createVerifiedStateBackup(input: {
  roots: readonly BackupRoot[];
  backupParent: string;
  lease: SharedStateLease;
  verifyNoUncoordinatedWriters: () => Promise<boolean>;
}): Promise<string> {
  if (
    input.roots.length !== 2 ||
    !input.roots.some((root) => root.name === 'private') ||
    !input.roots.some((root) => root.name === 'shared')
  )
    throw new Error('Private and shared backup roots are required');
  await input.lease.assertHeld();
  if (!(await input.verifyNoUncoordinatedWriters()))
    throw new Error('Cannot rule out other state writers');
  const roots = await Promise.all(
    input.roots.map(async (root) => ({
      ...root,
      path: await fs.realpath(root.path),
    })),
  );
  const backupParent = await fs.realpath(input.backupParent);
  for (const root of roots) {
    if (
      pathsOverlap(root.path, backupParent) ||
      pathsOverlap(backupParent, root.path)
    )
      throw new Error('Backup location overlaps a state root');
  }
  if (
    pathsOverlap(roots[0].path, roots[1].path) ||
    pathsOverlap(roots[1].path, roots[0].path)
  )
    throw new Error('State roots overlap');
  const destination = await fs.mkdtemp(
    path.join(backupParent, 'nevermind-state-'),
  );
  await fs.chmod(destination, 0o700);
  try {
    const manifest: Record<string, BackupEntry[]> = {};
    for (const root of roots) {
      const before = await inventory(root.path);
      const target = path.join(destination, root.name);
      await fs.cp(root.path, target, {
        recursive: true,
        dereference: false,
        errorOnExist: true,
        force: false,
      });
      const copied = await inventory(target);
      const after = await inventory(root.path);
      if (
        JSON.stringify(before) !== JSON.stringify(copied) ||
        JSON.stringify(before) !== JSON.stringify(after)
      )
        throw new Error('State changed during backup');
      manifest[root.name] = copied;
    }
    await input.lease.assertHeld();
    if (!(await input.verifyNoUncoordinatedWriters()))
      throw new Error('Cannot rule out other state writers');
    for (const root of roots) {
      if (
        JSON.stringify(await inventory(root.path)) !==
        JSON.stringify(manifest[root.name])
      )
        throw new Error('State changed during backup');
    }
    await input.lease.assertHeld();
    await fs.writeFile(
      path.join(destination, 'manifest.json'),
      `${JSON.stringify({ version: 1, roots: manifest }, null, 2)}\n`,
      { flag: 'wx', mode: 0o600 },
    );
    return destination;
  } catch (error) {
    await fs.rm(destination, { recursive: true, force: true }).catch(() => {});
    throw error;
  }
}
