// Parse headers without decoding image pixels in the IPC or billing process.
function dataView(bytes: Uint8Array) {
  return new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
}

function ascii(bytes: Uint8Array, start: number, end: number) {
  return String.fromCharCode(...bytes.subarray(start, end));
}

function matchesBytes(bytes: Uint8Array, expected: number[]) {
  return expected.every((value, index) => bytes[index] === value);
}

function uint16BigEndian(bytes: Uint8Array, offset: number) {
  return dataView(bytes).getUint16(offset, false);
}

function uint16LittleEndian(bytes: Uint8Array, offset: number) {
  return dataView(bytes).getUint16(offset, true);
}

function uint32BigEndian(bytes: Uint8Array, offset: number) {
  return dataView(bytes).getUint32(offset, false);
}

function uint32LittleEndian(bytes: Uint8Array, offset: number) {
  return dataView(bytes).getUint32(offset, true);
}

function dimensionsFromPng(bytes: Uint8Array) {
  const pngSignature = [137, 80, 78, 71, 13, 10, 26, 10];
  if (
    bytes.length < 33 ||
    !matchesBytes(bytes, pngSignature) ||
    uint32BigEndian(bytes, 8) !== 13 ||
    ascii(bytes, 12, 16) !== 'IHDR'
  )
    return null;

  const dimensions = {
    width: uint32BigEndian(bytes, 16),
    height: uint32BigEndian(bytes, 20),
  };
  let offset = 8;
  for (
    let chunkCount = 0;
    chunkCount < 1_000 && offset + 12 <= bytes.length;
    chunkCount += 1
  ) {
    const chunkLength = uint32BigEndian(bytes, offset);
    const chunkType = ascii(bytes, offset + 4, offset + 8);
    const nextOffset = offset + 12 + chunkLength;
    if (nextOffset > bytes.length) return null;
    if (chunkType === 'acTL') return null;
    if (chunkType === 'IEND') return chunkLength === 0 ? dimensions : null;
    offset = nextOffset;
  }
  return null;
}

function dimensionsFromJpeg(bytes: Uint8Array) {
  if (bytes.length < 4 || bytes[0] !== 0xff || bytes[1] !== 0xd8) return null;
  const frameMarkers = new Set([
    0xc0, 0xc1, 0xc2, 0xc3, 0xc5, 0xc6, 0xc7, 0xc9, 0xca, 0xcb, 0xcd, 0xce,
    0xcf,
  ]);
  let offset = 2;
  while (offset + 4 <= bytes.length) {
    if (bytes[offset] !== 0xff) {
      offset += 1;
      continue;
    }
    while (offset < bytes.length && bytes[offset] === 0xff) offset += 1;
    const marker = bytes[offset];
    offset += 1;
    if (marker === 0xd9 || marker === 0xda) return null;
    if (marker === 0x01 || (marker >= 0xd0 && marker <= 0xd7)) continue;
    if (offset + 2 > bytes.length) return null;
    const segmentLength = uint16BigEndian(bytes, offset);
    if (segmentLength < 2 || offset + segmentLength > bytes.length) return null;
    if (frameMarkers.has(marker)) {
      if (segmentLength < 7) return null;
      return {
        height: uint16BigEndian(bytes, offset + 3),
        width: uint16BigEndian(bytes, offset + 5),
      };
    }
    offset += segmentLength;
  }
  return null;
}

function uint24LittleEndian(bytes: Uint8Array, offset: number) {
  return bytes[offset] | (bytes[offset + 1] << 8) | (bytes[offset + 2] << 16);
}

function dimensionsFromVp8Chunk(
  bytes: Uint8Array,
  start: number,
  length: number,
) {
  if (
    length < 10 ||
    bytes[start] & 0x01 ||
    bytes[start + 3] !== 0x9d ||
    bytes[start + 4] !== 0x01 ||
    bytes[start + 5] !== 0x2a
  )
    return null;
  return {
    width: uint16LittleEndian(bytes, start + 6) & 0x3fff,
    height: uint16LittleEndian(bytes, start + 8) & 0x3fff,
  };
}

function dimensionsFromVp8lChunk(
  bytes: Uint8Array,
  start: number,
  length: number,
) {
  if (length < 5 || bytes[start] !== 0x2f) return null;
  return {
    width: 1 + (bytes[start + 1] | ((bytes[start + 2] & 0x3f) << 8)),
    height:
      1 +
      ((bytes[start + 2] >> 6) |
        (bytes[start + 3] << 2) |
        ((bytes[start + 4] & 0x0f) << 10)),
  };
}

function dimensionsFromWebp(bytes: Uint8Array) {
  if (
    bytes.length < 20 ||
    ascii(bytes, 0, 4) !== 'RIFF' ||
    uint32LittleEndian(bytes, 4) !== bytes.length - 8 ||
    ascii(bytes, 8, 12) !== 'WEBP'
  )
    return null;

  let offset = 12;
  let canvas: { width: number; height: number } | null = null;
  let frame: { width: number; height: number } | null = null;
  while (offset + 8 <= bytes.length) {
    const chunkType = ascii(bytes, offset, offset + 4);
    const chunkLength = uint32LittleEndian(bytes, offset + 4);
    const dataStart = offset + 8;
    const dataEnd = dataStart + chunkLength;
    const paddedEnd = dataEnd + (chunkLength % 2);
    if (dataEnd > bytes.length || paddedEnd > bytes.length) return null;
    if (chunkType === 'ANIM' || chunkType === 'ANMF') return null;
    if (chunkType === 'VP8X') {
      if (offset !== 12 || chunkLength !== 10 || bytes[dataStart] & 0x02)
        return null;
      canvas = {
        width: uint24LittleEndian(bytes, dataStart + 4) + 1,
        height: uint24LittleEndian(bytes, dataStart + 7) + 1,
      };
    } else if (chunkType === 'VP8 ') {
      if (frame) return null;
      frame = dimensionsFromVp8Chunk(bytes, dataStart, chunkLength);
      if (!frame) return null;
    } else if (chunkType === 'VP8L') {
      if (frame) return null;
      frame = dimensionsFromVp8lChunk(bytes, dataStart, chunkLength);
      if (!frame) return null;
    }
    offset = paddedEnd;
  }
  if (offset !== bytes.length || !frame) return null;
  return {
    width: Math.max(canvas?.width ?? 0, frame.width),
    height: Math.max(canvas?.height ?? 0, frame.height),
  };
}

function skipGifSubBlocks(bytes: Uint8Array, start: number) {
  let offset = start;
  while (offset < bytes.length) {
    const blockLength = bytes[offset];
    offset += 1;
    if (blockLength === 0) return offset;
    offset += blockLength;
    if (offset > bytes.length) return null;
  }
  return null;
}

function dimensionsFromGif(bytes: Uint8Array) {
  if (bytes.length < 14 || !['GIF87a', 'GIF89a'].includes(ascii(bytes, 0, 6)))
    return null;
  const width = uint16LittleEndian(bytes, 6);
  const height = uint16LittleEndian(bytes, 8);
  const packed = bytes[10];
  let offset = 13;
  if (packed & 0x80) offset += 3 * 2 ** ((packed & 0x07) + 1);
  if (offset > bytes.length) return null;
  let imageCount = 0;
  let hasTrailer = false;
  let maximumFrameWidth = 0;
  let maximumFrameHeight = 0;

  while (offset < bytes.length) {
    const marker = bytes[offset];
    offset += 1;
    if (marker === 0x3b) {
      hasTrailer = true;
      break;
    }
    if (marker === 0x21) {
      if (offset >= bytes.length) return null;
      offset += 1;
      const afterBlocks = skipGifSubBlocks(bytes, offset);
      if (afterBlocks === null) return null;
      offset = afterBlocks;
      continue;
    }
    if (marker !== 0x2c || offset + 9 > bytes.length) return null;
    imageCount += 1;
    if (imageCount > 1) return null;
    const frameWidth = uint16LittleEndian(bytes, offset + 4);
    const frameHeight = uint16LittleEndian(bytes, offset + 6);
    if (frameWidth < 1 || frameHeight < 1) return null;
    maximumFrameWidth = Math.max(maximumFrameWidth, frameWidth);
    maximumFrameHeight = Math.max(maximumFrameHeight, frameHeight);
    const framePacked = bytes[offset + 8];
    offset += 9;
    if (framePacked & 0x80) offset += 3 * 2 ** ((framePacked & 0x07) + 1);
    if (offset >= bytes.length) return null;
    offset += 1;
    const afterBlocks = skipGifSubBlocks(bytes, offset);
    if (afterBlocks === null) return null;
    offset = afterBlocks;
  }

  return hasTrailer && imageCount === 1
    ? {
        width: Math.max(width, maximumFrameWidth),
        height: Math.max(height, maximumFrameHeight),
      }
    : null;
}

export function imageDimensions(bytes: Uint8Array, mimeType: string) {
  if (mimeType === 'image/png') return dimensionsFromPng(bytes);
  if (mimeType === 'image/jpeg') return dimensionsFromJpeg(bytes);
  if (mimeType === 'image/webp') return dimensionsFromWebp(bytes);
  if (mimeType === 'image/gif') return dimensionsFromGif(bytes);
  return null;
}
