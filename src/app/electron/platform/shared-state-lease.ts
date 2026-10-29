import crypto from 'node:crypto';
import fs from 'node:fs/promises';
import path from 'node:path';

export type SharedStateLease = {
  assertHeld: () => Promise<void>;
  release: () => Promise<void>;
};

// No age-based takeover: a suspended writer must never be mistaken for a dead one.
export async function acquireSharedStateLease(
  sharedStateRoot: string,
): Promise<SharedStateLease> {
  const root = path.resolve(sharedStateRoot);
  await fs.mkdir(path.dirname(root), { recursive: true });
  let canonicalRoot: string;
  try {
    canonicalRoot = await fs.realpath(root);
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code !== 'ENOENT') throw error;
    const entry = await fs.lstat(root).catch((statError) => {
      if ((statError as NodeJS.ErrnoException).code === 'ENOENT') return null;
      throw statError;
    });
    if (entry?.isSymbolicLink())
      throw new Error('Shared state root is a dangling symlink');
    canonicalRoot = path.join(
      await fs.realpath(path.dirname(root)),
      path.basename(root),
    );
  }
  const lockDirectory = `${canonicalRoot}.writer-lock`;
  const ownerFile = path.join(lockDirectory, 'owner.json');
  const token = crypto.randomUUID();
  try {
    await fs.mkdir(lockDirectory, { mode: 0o700 });
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code === 'EEXIST')
      throw new Error('Shared state is already in use or needs lock recovery');
    throw error;
  }
  try {
    await fs.writeFile(ownerFile, JSON.stringify({ token, pid: process.pid }), {
      flag: 'wx',
      mode: 0o600,
    });
  } catch (error) {
    await fs.rmdir(lockDirectory).catch(() => {});
    throw error;
  }
  let released = false;
  async function assertHeld() {
    if (released) throw new Error('Shared state lease was released');
    const owner = JSON.parse(await fs.readFile(ownerFile, 'utf8'));
    if (owner?.token !== token)
      throw new Error('Shared state lease ownership changed');
  }
  async function release() {
    if (released) return;
    await assertHeld();
    await fs.unlink(ownerFile);
    await fs.rmdir(lockDirectory);
    released = true;
  }
  return { assertHeld, release };
}
