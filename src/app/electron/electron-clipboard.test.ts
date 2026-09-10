import assert from 'node:assert/strict';
import test from 'node:test';
import { createElectronClipboardApi } from './electron-clipboard';

function createFakes(
  clipboardItems: Array<{ types: string[]; value: unknown }>,
) {
  const calls = { readImage: 0, createFromBuffer: 0 };
  const clipboard = {
    read: async () =>
      clipboardItems.map((entry) => ({
        types: entry.types,
        getType: async () => entry.value,
      })),
    readImage: () => {
      calls.readImage += 1;
      return { isEmpty: () => false, toPNG: () => Buffer.from('native-image') };
    },
    readText: () => '',
    clear: () => {},
  };
  const nativeImage = {
    createFromBuffer: (buffer: Buffer) => {
      calls.createFromBuffer += 1;
      return {
        isEmpty: () => buffer.length === 0,
        toPNG: () => buffer,
      };
    },
    createEmpty: () => ({ isEmpty: () => true, toPNG: () => Buffer.from('') }),
  };
  const api = createElectronClipboardApi({
    clipboard: clipboard as any,
    nativeImage: nativeImage as any,
    ClipboardItem: class {},
  });
  return { api, calls };
}

test('readImage decodes explicit png clipboard items', async () => {
  const { api, calls } = createFakes([
    { types: ['image/png'], value: new Blob([Buffer.from('png-bytes')]) },
  ]);

  const image = await api.readImage();

  assert.equal(image.isEmpty(), false);
  assert.equal(calls.createFromBuffer, 1);
  assert.equal(calls.readImage, 0, 'native fallback should not be used');
});

test('readImage falls back to native readImage for non-png formats', async () => {
  const { api, calls } = createFakes([
    { types: ['image/tiff'], value: new Blob([Buffer.from('tiff-bytes')]) },
  ]);

  const image = await api.readImage();

  assert.equal(image.isEmpty(), false);
  assert.equal(calls.createFromBuffer, 0);
  assert.equal(calls.readImage, 1, 'native readImage should be used');
});

test('readImage falls back to native readImage for empty clipboards', async () => {
  const { api, calls } = createFakes([]);

  const image = await api.readImage();

  assert.equal(image.isEmpty(), false);
  assert.equal(calls.readImage, 1);
});
