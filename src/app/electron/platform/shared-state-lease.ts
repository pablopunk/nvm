import crypto from 'node:crypto';
import fs from 'node:fs/promises';
import path from 'node:path';

export type SharedStateLease = {
  assertHeld: () => Promise<void>;
  release: () => Promise<void>;
};

async function processNamespace(): Promise<string | null> {
  try {
    const [boot, namespace] = await Promise.all([
      fs.readFile('/proc/sys/kernel/random/boot_id', 'utf8'),
      fs.readlink('/proc/self/ns/pid'),
    ]);
    const bootId = boot.trim();
    return bootId && namespace ? `${bootId}:${namespace}` : null;
  } catch {
    return null;
  }
}

async function recoverDeadLease(lockDirectory: string): Promise<boolean> {
  const claim = path.join(lockDirectory, 'recovery.claim');
  try {
    const file = await fs.open(claim, 'wx', 0o600);
    await file.close();
  } catch (error) {
    const namespace = await processNamespace();
    if (
      ['EEXIST', 'ENOENT'].includes((error as NodeJS.ErrnoException).code ?? '')
    )
      return false;
    throw error;
  }
  let moved = false;
  try {
    const owner = JSON.parse(
      await fs.readFile(path.join(lockDirectory, 'owner.json'), 'utf8'),
    );
    if (
      !Number.isSafeInteger(owner?.pid) ||
      owner.pid < 1 ||
      typeof owner.token !== 'string' ||
      namespace === null ||
      owner.namespace !== namespace
    )
      return false;
    try {
      process.kill(owner.pid, 0);
      return false;
    } catch (error) {
      if ((error as NodeJS.ErrnoException).code !== 'ESRCH') return false;
    }
    const quarantine = `${lockDirectory}.recovered-${crypto.randomUUID()}`;
    await fs.rename(lockDirectory, quarantine);
    moved = true;
    await fs.rm(quarantine, { recursive: true, force: true });
    return true;
  } catch {
    return false;
  } finally {
    if (!moved) await fs.unlink(claim).catch(() => {});
  }
}

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
    if ((error as NodeJS.ErrnoException).code !== 'EEXIST') throw error;
    if (!(await recoverDeadLease(lockDirectory)))
      throw new Error('Shared state is already in use or needs lock recovery');
    await fs.mkdir(lockDirectory, { mode: 0o700 });
  }
  try {
    await fs.writeFile(
      ownerFile,
      JSON.stringify({
        token,
        pid: process.pid,
        namespace: await processNamespace(),
      }),
      {
        flag: 'wx',
        mode: 0o600,
      },
    );
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
