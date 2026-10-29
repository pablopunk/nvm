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
): Promise<StateCompatibilityMarker> {
  if (!Number.isSafeInteger(epoch) || epoch < 1)
    throw new Error('Invalid state compatibility epoch');
  const file = path.join(directory, MARKER_NAME);
  let marker: StateCompatibilityMarker;
  try {
    marker = parseMarker(
      JSON.parse(await fs.readFile(file, 'utf8')),
    ) as StateCompatibilityMarker;
    if (!marker) throw new Error('Invalid state compatibility marker');
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code !== 'ENOENT') throw error;
    // A missing marker cannot distinguish a fresh profile from a deleted marker.
    marker = { version: 1, highestEpoch: epoch, trusted: false };
  }
  if (marker.highestEpoch > epoch)
    throw new Error('This build cannot safely open newer persisted state');
  const next = {
    ...marker,
    highestEpoch: Math.max(marker.highestEpoch, epoch),
  };
  await atomicallyReplaceFile(file, `${JSON.stringify(next)}\n`);
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
