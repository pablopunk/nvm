import assert from 'node:assert/strict';
import test from 'node:test';
import { normalizeClipboardHistory } from './clipboard-utils';

test('clipboard image migration awaits asynchronous persistence', async () => {
  let persisted = false;
  const history = normalizeClipboardHistory(
    [
      {
        type: 'image',
        imageDataUrl: 'data:image/png;base64,aGVsbG8=',
        createdAt: 1,
      },
    ],
    10,
    async () => {
      await Promise.resolve();
      persisted = true;
      return '/tmp/image.png';
    },
  );

  assert.equal(persisted, false);
  const items = await history;
  assert.equal(persisted, true);
  assert.equal(items[0]?.type, 'image');
  assert.equal(items[0]?.imagePath, '/tmp/image.png');
});
