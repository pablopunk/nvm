import assert from 'node:assert/strict';
import test from 'node:test';
import { copyImageAction, IMAGE_UNAVAILABLE_TOAST } from './copy-image-action';

function createCopy(imageIsEmpty: boolean) {
  const written: unknown[] = [];
  const image = { isEmpty: () => imageIsEmpty };
  const deps = {
    clipboardApi: { writeImage: (value: unknown) => written.push(value) },
    nativeImage: {
      createFromPath: () => image,
      createFromDataURL: () => image,
    },
    expandUserPath: (value: string) => value,
  };
  return {
    copy: (action: Parameters<typeof copyImageAction>[0]) =>
      copyImageAction(action, deps),
    written,
    image,
  };
}

const COPY_BRANCHES = [
  ['path', { path: '/tmp/a.png' }],
  ['imagePath', { imagePath: '/tmp/b.png' }],
  ['imageDataUrl', { imageDataUrl: 'data:image/png;base64,AA==' }],
] as const;

for (const [branch, action] of COPY_BRANCHES) {
  test(`copyImage via ${branch} writes a real image`, async () => {
    const { copy, written, image } = createCopy(false);
    assert.equal(await copy(action), undefined);
    assert.deepEqual(written, [image]);
  });

  test(`copyImage via ${branch} refuses an empty image`, async () => {
    const { copy, written } = createCopy(true);
    assert.deepEqual(await copy(action), IMAGE_UNAVAILABLE_TOAST);
    assert.deepEqual(written, []);
  });
}
