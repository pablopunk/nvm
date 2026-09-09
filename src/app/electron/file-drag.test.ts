import assert from 'node:assert/strict';
import test from 'node:test';
import { fileDragIconForPath } from './file-drag';

const PNG_DATA_URL = /^data:image\/png;base64,/;

function fakeImage(empty: boolean, name: string) {
  return {
    isEmpty: () => empty,
    resize: (options: unknown) => ({ name, options }),
  };
}

test('uses the file itself as the drag icon when Electron can decode it', () => {
  const result = fileDragIconForPath(
    {
      createFromPath: () => fakeImage(false, 'file'),
      createFallback: () => fakeImage(false, 'fallback'),
    },
    '/tmp/image.png',
  );

  assert.deepEqual(result, {
    name: 'file',
    options: { width: 64, height: 64, quality: 'good' },
  });
});

test('uses a visible fallback icon for files and directories Electron cannot decode', () => {
  let fallbackDataUrl = '';
  const fallback = fakeImage(false, 'fallback');
  const result = fileDragIconForPath(
    {
      createFromPath: () => fakeImage(true, 'file'),
      createFallback: (dataUrl) => {
        fallbackDataUrl = dataUrl;
        return fallback;
      },
    },
    '/Applications/Example.app',
  );

  assert.equal(result, fallback);
  assert.match(fallbackDataUrl, PNG_DATA_URL);
});
