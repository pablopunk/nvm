import fs from 'node:fs/promises';
import path from 'node:path';
import compatibility from '../../resources/state-compatibility.json';
import { atomicallyReplaceFile } from '../platform/atomic-file';

// Increment before a host-owned persisted format becomes unreadable by earlier builds.
export const STATE_COMPATIBILITY_EPOCH = compatibility.epoch;
const MARKER_NAME = 'state-compatibility.json';

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
  if (!Number.isSafeInteger(epoch) || epoch < 1)
    throw new Error('Invalid state compatibility epoch');
  const directories = [
    ...new Set([directory, sharedDirectory].map((item) => path.resolve(item))),
  ];
  const markers = await Promise.all(
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
  let trusted = markers.every((marker) => marker?.trusted === true);
  if (markers.every((marker) => marker === null)) {
    const entries = await Promise.all(
      directories.map(async (root) => {
        try {
          return await fs.readdir(root);
        } catch (error) {
          if ((error as NodeJS.ErrnoException).code === 'ENOENT') return [];
          throw error;
        }
      }),
    );
    trusted = entries.every((items) => items.length === 0);
  }
  const next: StateCompatibilityMarker = { version: 1, highestEpoch, trusted };
  for (const root of directories)
    await atomicallyReplaceFile(
      path.join(root, MARKER_NAME),
      `${JSON.stringify(next)}\n`,
    );
  return next;
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
