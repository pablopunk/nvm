import assert from 'node:assert/strict';
import test from 'node:test';
import { AI_CHAT_IMAGE_LIMIT } from '../../shared/ai-chat-images';
import { normalizeAiChatImages } from './ai-chat-images';

function pngBytes(width = 1, height = 1) {
  const bytes = Buffer.alloc(57);
  bytes.set([137, 80, 78, 71, 13, 10, 26, 10]);
  bytes.writeUInt32BE(13, 8);
  bytes.write('IHDR', 12, 'ascii');
  bytes.writeUInt32BE(width, 16);
  bytes.writeUInt32BE(height, 20);
  bytes.write('IDAT', 37, 'ascii');
  bytes.write('IEND', 49, 'ascii');
  return bytes;
}

const png = {
  data: pngBytes().toString('base64'),
  mimeType: 'image/png',
  name: 'clipboard.png',
};

test('normalizes clone-safe AI chat images for Pi and disk storage', () => {
  assert.deepEqual(normalizeAiChatImages([png]), [
    {
      ...png,
      bytes: pngBytes(),
      extension: 'png',
    },
  ]);
});

test('rejects unsupported, malformed, and excessive AI chat images', () => {
  assert.throws(
    () => normalizeAiChatImages([{ ...png, mimeType: 'image/svg+xml' }]),
    /PNG, JPEG, WebP, or GIF/,
  );
  assert.throws(
    () => normalizeAiChatImages([{ ...png, data: 'not base64' }]),
    /data is invalid/,
  );
  assert.throws(
    () =>
      normalizeAiChatImages([
        { ...png, data: Buffer.from('image').toString('base64') },
      ]),
    /valid and 2048 pixels/,
  );
  assert.throws(
    () =>
      normalizeAiChatImages([
        { ...png, data: pngBytes(2049, 1).toString('base64') },
      ]),
    /valid and 2048 pixels/,
  );
  assert.throws(
    () => normalizeAiChatImages(Array(AI_CHAT_IMAGE_LIMIT + 1).fill(png)),
    /no more than 8/,
  );
});
