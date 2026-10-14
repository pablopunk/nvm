import assert from 'node:assert/strict';
import { test } from 'node:test';
import {
  AI_CHAT_IMAGE_LIMIT,
  AI_CHAT_IMAGE_MAX_BYTES,
  AI_CHAT_IMAGE_TOTAL_MAX_BYTES,
} from '../../../app/shared/ai-chat-images';
import { analyzeAiImageInputs, imagePayloadIntent } from './ai-image-input';

function png(width = 1, height = 1, payloadBytes = 0) {
  const bytes = Buffer.alloc(57 + payloadBytes);
  bytes.set([137, 80, 78, 71, 13, 10, 26, 10]);
  bytes.writeUInt32BE(13, 8);
  bytes.write('IHDR', 12, 'ascii');
  bytes.writeUInt32BE(width, 16);
  bytes.writeUInt32BE(height, 20);
  bytes.writeUInt32BE(payloadBytes, 33);
  bytes.write('IDAT', 37, 'ascii');
  bytes.write('IEND', 49 + payloadBytes, 'ascii');
  return bytes;
}

function jpeg(width = 1, height = 1) {
  const bytes = Buffer.alloc(21);
  bytes.set([0xff, 0xd8, 0xff, 0xc0, 0, 17, 8]);
  bytes.writeUInt16BE(height, 7);
  bytes.writeUInt16BE(width, 9);
  bytes[11] = 3;
  return bytes;
}

function webp(
  width = 1,
  height = 1,
  canvasWidth = width,
  canvasHeight = height,
) {
  const extended = canvasWidth !== width || canvasHeight !== height;
  const bytes = Buffer.alloc(extended ? 48 : 30);
  bytes.write('RIFF', 0, 'ascii');
  bytes.writeUInt32LE(bytes.length - 8, 4);
  bytes.write('WEBP', 8, 'ascii');
  let chunkStart = 12;
  if (extended) {
    bytes.write('VP8X', chunkStart, 'ascii');
    bytes.writeUInt32LE(10, chunkStart + 4);
    bytes[chunkStart + 8 + 4] = (canvasWidth - 1) & 0xff;
    bytes[chunkStart + 8 + 5] = ((canvasWidth - 1) >> 8) & 0xff;
    bytes[chunkStart + 8 + 6] = ((canvasWidth - 1) >> 16) & 0xff;
    bytes[chunkStart + 8 + 7] = (canvasHeight - 1) & 0xff;
    bytes[chunkStart + 8 + 8] = ((canvasHeight - 1) >> 8) & 0xff;
    bytes[chunkStart + 8 + 9] = ((canvasHeight - 1) >> 16) & 0xff;
    chunkStart += 18;
  }
  bytes.write('VP8 ', chunkStart, 'ascii');
  bytes.writeUInt32LE(10, chunkStart + 4);
  const frameStart = chunkStart + 8;
  bytes[frameStart + 3] = 0x9d;
  bytes[frameStart + 4] = 0x01;
  bytes[frameStart + 5] = 0x2a;
  bytes.writeUInt16LE(width, frameStart + 6);
  bytes.writeUInt16LE(height, frameStart + 8);
  return bytes;
}

function gif(width = 1, height = 1, frameWidth = width, frameHeight = height) {
  const bytes = Buffer.alloc(28);
  bytes.write('GIF89a', 0, 'ascii');
  bytes.writeUInt16LE(width, 6);
  bytes.writeUInt16LE(height, 8);
  bytes[13] = 0x2c;
  bytes.writeUInt16LE(frameWidth, 18);
  bytes.writeUInt16LE(frameHeight, 20);
  bytes[23] = 2;
  bytes[24] = 1;
  bytes[26] = 0;
  bytes[27] = 0x3b;
  return bytes;
}

function imageUrl(data: Buffer, mimeType = 'image/png') {
  return {
    type: 'image_url',
    image_url: `data:${mimeType};base64,${data.toString('base64')}`,
  };
}

test('accepts bounded inline PNG image shapes used by supported chat APIs', () => {
  const data = png();
  const payloads = [
    { messages: [{ content: [imageUrl(data)] }] },
    {
      messages: [
        {
          content: [
            {
              type: 'image',
              source: {
                type: 'base64',
                media_type: 'image/png',
                data: data.toString('base64'),
              },
            },
          ],
        },
      ],
    },
    {
      contents: [
        {
          parts: [
            {
              inlineData: {
                mimeType: 'image/png',
                data: data.toString('base64'),
              },
            },
          ],
        },
      ],
    },
  ];

  for (const payload of payloads)
    assert.deepEqual(analyzeAiImageInputs(payload), {
      imageCount: 1,
      imageBytes: data.length,
      invalid: false,
    });
});

test('accepts the maximum supported image dimensions', () => {
  assert.deepEqual(analyzeAiImageInputs(imageUrl(png(2048, 2048))), {
    imageCount: 1,
    imageBytes: 57,
    invalid: false,
  });
});

test('accepts valid JPEG, WebP, and single-frame GIF headers', () => {
  for (const [mimeType, data] of [
    ['image/jpeg', jpeg(64, 32)],
    ['image/webp', webp(64, 32)],
    ['image/gif', gif(64, 32)],
  ] as const) {
    assert.equal(analyzeAiImageInputs(imageUrl(data, mimeType)).invalid, false);
  }
});

test('rejects remote, malformed, oversized, and over-dimensioned image shapes', () => {
  const malformedInputs = [
    { type: 'image_url', image_url: 'https://example.test/image.png' },
    { type: 'image_url', image_url: 'data:image/png;base64,not-base64' },
    imageUrl(png(2049, 1)),
    imageUrl(png(1, 2049)),
    imageUrl(gif(1, 1, 2049, 1), 'image/gif'),
    imageUrl(webp(2049, 1, 1, 1), 'image/webp'),
    imageUrl(png(1, 1, AI_CHAT_IMAGE_MAX_BYTES)),
  ];

  for (const payload of malformedInputs)
    assert.equal(analyzeAiImageInputs(payload).invalid, true);
});

test('rejects more images or aggregate bytes than the shared upload policy', () => {
  const tiny = imageUrl(png());
  assert.equal(
    analyzeAiImageInputs(
      Array.from({ length: AI_CHAT_IMAGE_LIMIT + 1 }, () => tiny),
    ).invalid,
    true,
  );

  const imageBytes = Math.floor(AI_CHAT_IMAGE_TOTAL_MAX_BYTES / 2) + 1;
  const data = imageUrl(png(1, 1, imageBytes - 57));
  assert.equal(analyzeAiImageInputs([data, data]).invalid, true);
});

test('audio inlineData is not misreported as an invalid image', () => {
  const audio = { inlineData: { mimeType: 'audio/wav', data: 'AAAA' } };
  assert.equal(imagePayloadIntent(audio), false);
  assert.deepEqual(analyzeAiImageInputs(audio), {
    imageCount: 0,
    imageBytes: 0,
    invalid: false,
  });
});

test('does not accept image bytes under a conflicting non-image content type', () => {
  const payload = {
    type: 'audio',
    inlineData: {
      mimeType: 'image/png',
      data: png().toString('base64'),
    },
  };

  assert.equal(analyzeAiImageInputs(payload).invalid, true);
});
