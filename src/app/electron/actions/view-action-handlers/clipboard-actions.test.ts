import assert from 'node:assert/strict';
import test from 'node:test';
import { createClipboardActionHandlers } from './clipboard-actions';

function createHandlers(imageIsEmpty: boolean) {
  const written: unknown[] = [];
  const image = { isEmpty: () => imageIsEmpty };
  const handlers = createClipboardActionHandlers({
    clipboardApi: {
      writeText: () => {},
      writeImage: (value) => written.push(value),
    },
    nativeImage: {
      createFromPath: () => image,
      createFromDataURL: () => image,
    },
    paletteWindow: { win: null, hidePalette: async () => {} },
    clipboardPasteText: async () => {},
    pasteClipboardAction: async () => {},
    removeClipboardHistoryEntries: () => {},
  });
  return { handlers, written, image };
}

const COPY_BRANCHES = [
  ['path', { path: '/tmp/a.png' }],
  ['imagePath', { imagePath: '/tmp/b.png' }],
  ['imageDataUrl', { imageDataUrl: 'data:image/png;base64,AA==' }],
] as const;

for (const [branch, action] of COPY_BRANCHES) {
  test(`copyImage via ${branch} writes a real image`, async () => {
    const { handlers, written, image } = createHandlers(false);
    assert.equal(await handlers.handleCopyImage(action), undefined);
    assert.deepEqual(written, [image]);
  });

  test(`copyImage via ${branch} refuses an empty image`, async () => {
    const { handlers, written } = createHandlers(true);
    assert.deepEqual(await handlers.handleCopyImage(action), {
      toast: { message: 'Image no longer available', tone: 'error' },
    });
    assert.deepEqual(written, []);
  });
}
