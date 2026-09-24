import fs from 'node:fs/promises';
import path from 'node:path';

export type SavedDictation = {
  id: string;
  createdAt: number;
  segmentCount: number;
};

export function createDictationRecordings(directory: string) {
  function recordingPath(id: string) {
    if (!/^dictation-[a-f0-9-]{36}$/.test(id))
      throw new Error('Invalid dictation recording ID');
    return path.join(directory, id);
  }

  async function save(id: string, segments: Uint8Array[]) {
    const location = recordingPath(id);
    await fs.mkdir(location, { recursive: true, mode: 0o700 });
    for (const [index, audio] of segments.entries())
      await fs.writeFile(path.join(location, `${index}.webm`), audio, {
        mode: 0o600,
      });
    await fs.writeFile(
      path.join(location, 'recording.json'),
      JSON.stringify({
        id,
        createdAt: Date.now(),
        segmentCount: segments.length,
      }),
      { mode: 0o600 },
    );
  }

  async function list(): Promise<SavedDictation[]> {
    const entries = await fs.readdir(directory).catch(() => []);
    const recordings = await Promise.all(
      entries.map(async (id) => {
        try {
          const metadata = JSON.parse(
            await fs.readFile(
              path.join(recordingPath(id), 'recording.json'),
              'utf8',
            ),
          ) as SavedDictation;
          return metadata.id === id &&
            Number.isInteger(metadata.segmentCount) &&
            metadata.segmentCount > 0 &&
            metadata.segmentCount <= 120
            ? metadata
            : null;
        } catch {
          return null;
        }
      }),
    );
    return recordings
      .filter((entry): entry is SavedDictation => entry !== null)
      .sort((a, b) => b.createdAt - a.createdAt);
  }

  async function load(id: string) {
    const metadata = (await list()).find((entry) => entry.id === id);
    if (!metadata) throw new Error('Dictation recording not found');
    return Promise.all(
      Array.from({ length: metadata.segmentCount }, (_, index) =>
        fs.readFile(path.join(recordingPath(id), `${index}.webm`)),
      ),
    );
  }

  async function remove(id: string) {
    await fs.rm(recordingPath(id), { recursive: true, force: true });
  }

  return { save, list, load, remove };
}
