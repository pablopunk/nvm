import assert from 'node:assert/strict';
import test from 'node:test';
import { AI_CHAT_IMAGE_LIMIT } from '../shared/ai-chat-images';
import { normalizeAiChatImages } from './ai-chat-images';

const png = {
  data: Buffer.from('image').toString('base64'),
  mimeType: 'image/png',
  name: 'clipboard.png',
};

test('normalizes clone-safe AI chat images for Pi and disk storage', () => {
  assert.deepEqual(normalizeAiChatImages([png]), [
    {
      ...png,
      bytes: Buffer.from('image'),
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
    () => normalizeAiChatImages(Array(AI_CHAT_IMAGE_LIMIT + 1).fill(png)),
    /no more than 8/,
  );
});
