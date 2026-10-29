import fs from 'node:fs/promises';
import path from 'node:path';
import compatibility from '../../resources/state-compatibility.json';
import { atomicallyReplaceFile } from '../platform/atomic-file';
import {
  acquireSharedStateLease,
  type SharedStateLease,
} from '../platform/shared-state-lease';
import { createVerifiedStateBackup } from './state-migration-backup';

// Increment before a host-owned persisted format becomes unreadable by earlier builds.
export const STATE_COMPATIBILITY_EPOCH = compatibility.epoch;
const MARKER_NAME = 'state-compatibility.json';
const PENDING_NAME = 'state-compatibility.pending.json';

export type StateCompatibilityMarker = {
  version: 1;
  highestEpoch: number;
  trusted: boolean;
};

function parseMarker(value: unknown): StateCompatibilityMarker | null {
  if (!value || typeof value !== 'object') return null;
  const marker = value as Record<string, unknown>;
  if (
    marker.version !== 1 ||
    !Number.isSafeInteger(marker.highestEpoch) ||
    (marker.highestEpoch as number) < 1 ||
    typeof marker.trusted !== 'boolean'
  )
    return null;
  return marker as StateCompatibilityMarker;
}

export async function advanceStateCompatibility(
  directory: string,
  epoch: number,
  sharedDirectory = directory,
): Promise<StateCompatibilityMarker> {
  const lease = await acquireSharedStateLease(sharedDirectory);
  try {
    return await advanceMarkers(
      directory,
      epoch,
      sharedDirectory,
      lease.assertHeld,
    );
  } finally {
    await lease.release();
  }
}

async function advanceMarkers(
  directory: string,
  epoch: number,
  sharedDirectory: string,
  assertHeld: () => Promise<void>,
  establishTrust = false,
): Promise<StateCompatibilityMarker> {
  if (!Number.isSafeInteger(epoch) || epoch < 1)
    throw new Error('Invalid state compatibility epoch');
  const directories = [
    ...new Set([directory, sharedDirectory].map((item) => path.resolve(item))),
  ];
  let markers = await Promise.all(
    directories.map(async (root) => {
      try {
        const marker = parseMarker(
          JSON.parse(await fs.readFile(path.join(root, MARKER_NAME), 'utf8')),
        );
        if (!marker) throw new Error('Invalid state compatibility marker');
        return marker;
      } catch (error) {
        if ((error as NodeJS.ErrnoException).code !== 'ENOENT') throw error;
        return null;
      }
    }),
  );
  const pendingPath = path.join(path.resolve(sharedDirectory), PENDING_NAME);
  let pending: { roots: string[]; marker: StateCompatibilityMarker } | null =
    null;
  try {
    const raw = JSON.parse(await fs.readFile(pendingPath, 'utf8'));
    const marker = parseMarker(raw?.marker);
    if (!marker || JSON.stringify(raw?.roots) !== JSON.stringify(directories))
      throw new Error('Invalid pending state compatibility update');
    pending = { roots: directories, marker };
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code !== 'ENOENT') throw error;
  }
  if (pending) {
    if (
      pending.marker.highestEpoch > epoch ||
      markers.some(
        (marker) => (marker?.highestEpoch ?? 0) > pending!.marker.highestEpoch,
      )
    )
      throw new Error('Pending update conflicts with newer persisted state');
    const recovered = { ...pending.marker, trusted: false };
    for (const root of directories) {
      await assertHeld();
      await atomicallyReplaceFile(
        path.join(root, MARKER_NAME),
        `${JSON.stringify(recovered)}\n`,
      );
    }
    await fs.unlink(pendingPath);
    markers = directories.map(() => recovered);
  }
  const highestEpoch = Math.max(
    epoch,
    ...markers.map((marker) => marker?.highestEpoch ?? epoch),
  );
  if (highestEpoch > epoch)
    throw new Error('This build cannot safely open newer persisted state');
  if (
    markers.some((marker) => marker === null) &&
    markers.some((marker) => marker !== null)
  )
    throw new Error('A state compatibility marker is missing');
  const trusted =
    establishTrust || markers.every((marker) => marker?.trusted === true);
  const next: StateCompatibilityMarker = { version: 1, highestEpoch, trusted };
  await assertHeld();
  await atomicallyReplaceFile(
    pendingPath,
    `${JSON.stringify({ roots: directories, marker: next })}\n`,
  );
  for (const root of directories) {
    await assertHeld();
    await atomicallyReplaceFile(
      path.join(root, MARKER_NAME),
      `${JSON.stringify(next)}\n`,
    );
  }
  await assertHeld();
  await fs.unlink(pendingPath);
  return next;
}

export async function backupAndTrustState(input: {
  privateRoot: string;
  sharedRoot: string;
  backupParent: string;
  epoch: number;
  lease: SharedStateLease;
  legacyInstancesClosed: boolean;
  verifyNoUncoordinatedWriters: () => Promise<boolean>;
}) {
  if (!input.legacyInstancesClosed)
    throw new Error('Close older Nevermind instances before migrating state');
  const backupPath = await createVerifiedStateBackup({
    roots: [
      { name: 'private', path: input.privateRoot },
      { name: 'shared', path: input.sharedRoot },
    ],
    backupParent: input.backupParent,
    lease: input.lease,
    verifyNoUncoordinatedWriters: input.verifyNoUncoordinatedWriters,
  });
  await input.lease.assertHeld();
  if (!(await input.verifyNoUncoordinatedWriters()))
    throw new Error('Cannot rule out other state writers');
  const marker = await advanceMarkers(
    input.privateRoot,
    input.epoch,
    input.sharedRoot,
    input.lease.assertHeld,
    true,
  );
  return { backupPath, marker };
}

export function supportsStateDowngrade(
  marker: StateCompatibilityMarker | null,
  targetEpoch: unknown,
) {
  return (
    marker?.trusted === true &&
    Number.isSafeInteger(targetEpoch) &&
    targetEpoch === marker.highestEpoch
  );
}

export async function stateRootsSupportDowngrade(
  directories: readonly string[],
  targetEpoch: unknown,
): Promise<boolean> {
  const roots = [
    ...new Set(directories.map((directory) => path.resolve(directory))),
  ];
  if (roots.length !== 2) return false;
  let lease: Awaited<ReturnType<typeof acquireSharedStateLease>> | undefined;
  let supported = false;
  try {
    lease = await acquireSharedStateLease(roots[1]);
    await Promise.all(
      roots.map(async (root) => {
        try {
          await fs.access(path.join(root, PENDING_NAME));
          throw new Error('State compatibility update is pending');
        } catch (error) {
          if ((error as NodeJS.ErrnoException).code !== 'ENOENT') throw error;
        }
      }),
    );
    const markers = await Promise.all(
      roots.map(async (root) =>
        parseMarker(
          JSON.parse(await fs.readFile(path.join(root, MARKER_NAME), 'utf8')),
        ),
      ),
    );
    await lease.assertHeld();
    supported = markers.every((marker) =>
      supportsStateDowngrade(marker, targetEpoch),
    );
  } catch {
    supported = false;
  } finally {
    if (lease) {
      try {
        await lease.release();
      } catch {
        supported = false;
      }
    }
  }
  return supported;
}
