import fs from 'node:fs/promises';
import { expandUserPath, extensionForPath } from './file-utils';

const BYTES_PER_KILOBYTE = 1024;
const MAX_PREVIEW_KILOBYTES = 64;
const BINARY_SNIFF_KILOBYTES = 8;
const TEXT_FILE_PREVIEW_MAX_BYTES = MAX_PREVIEW_KILOBYTES * BYTES_PER_KILOBYTE;
const BINARY_SNIFF_BYTES = BINARY_SNIFF_KILOBYTES * BYTES_PER_KILOBYTE;
const SHORTEST_MARKDOWN_FENCE = 3;
const UNICODE_REPLACEMENT = '�';
const BACKTICK_RUN_PATTERN = /`+/g;

type TextFilePreview =
  | { kind: 'text'; text: string; truncated: boolean; sizeBytes: number }
  | { kind: 'binary'; sizeBytes: number }
  | { kind: 'unreadable'; message: string };

function looksBinary(bytes: Buffer) {
  return bytes.subarray(0, BINARY_SNIFF_BYTES).includes(0);
}

function decodeWithoutTrailingPartialCharacter(
  bytes: Buffer,
  truncated: boolean,
) {
  const text = new TextDecoder('utf-8').decode(bytes);
  return truncated && text.endsWith(UNICODE_REPLACEMENT)
    ? text.slice(0, -1)
    : text;
}

async function readLeadingBytes(filePath: string, maxBytes: number) {
  const handle = await fs.open(filePath, 'r');
  try {
    const { size } = await handle.stat();
    const buffer = Buffer.alloc(Math.min(size, maxBytes));
    const { bytesRead } = await handle.read(buffer, 0, buffer.length, 0);
    return { bytes: buffer.subarray(0, bytesRead), sizeBytes: size };
  } finally {
    await handle.close();
  }
}

async function readTextFilePreview(
  filePath: string,
  maxBytes = TEXT_FILE_PREVIEW_MAX_BYTES,
): Promise<TextFilePreview> {
  try {
    const { bytes, sizeBytes } = await readLeadingBytes(
      expandUserPath(filePath),
      maxBytes,
    );
    if (looksBinary(bytes)) {
      return { kind: 'binary', sizeBytes };
    }
    const truncated = sizeBytes > bytes.length;
    return {
      kind: 'text',
      text: decodeWithoutTrailingPartialCharacter(bytes, truncated),
      truncated,
      sizeBytes,
    };
  } catch (error) {
    return {
      kind: 'unreadable',
      message: error instanceof Error ? error.message : String(error),
    };
  }
}

function fenceLongerThanAnyBacktickRun(text: string) {
  const longestRun = Math.max(
    SHORTEST_MARKDOWN_FENCE - 1,
    ...(text.match(BACKTICK_RUN_PATTERN) || []).map((run) => run.length),
  );
  return '`'.repeat(longestRun + 1);
}

function formatKilobytes(bytes: number) {
  return `${Math.round(bytes / BYTES_PER_KILOBYTE)} KB`;
}

function textFilePreviewMarkdown(filePath: string, preview: TextFilePreview) {
  if (preview.kind === 'binary') {
    return `_Binary file (${formatKilobytes(preview.sizeBytes)}) — no text preview._`;
  }
  if (preview.kind === 'unreadable') {
    return `_Could not read this file: ${preview.message}_`;
  }
  const fence = fenceLongerThanAnyBacktickRun(preview.text);
  const language = extensionForPath(filePath) || '';
  const block = `${fence}${language}\n${preview.text}\n${fence}`;
  return preview.truncated
    ? `${block}\n\n_Showing the first ${formatKilobytes(Buffer.byteLength(preview.text))} of ${formatKilobytes(preview.sizeBytes)}._`
    : block;
}

export type { TextFilePreview };
export {
  readTextFilePreview,
  TEXT_FILE_PREVIEW_MAX_BYTES,
  textFilePreviewMarkdown,
};
