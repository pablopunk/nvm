import { createHash } from 'node:crypto';

type PiTools = typeof import('@earendil-works/pi-coding-agent');

export type BuilderReadPageOptions = {
  offset?: number;
  limit?: number;
  byteOffset?: number;
};

export type BuilderReadPage = {
  text: string;
  revision: string;
  totalLines: number;
  nextOffset: number | null;
  nextByteOffset: number | null;
  coveredLines?: { start: number; end: number };
  byteRange?: { line: number; start: number; end: number; total: number };
  complete: boolean;
};

export type ExtensionReadCoverage = {
  revision: string;
  totalLines: number;
  completeLines: Set<number>;
  lineByteRanges: Map<number, Array<{ start: number; end: number }>>;
};

export function boundedBuilderReadPage(
  pi: PiTools,
  content: string,
  options: BuilderReadPageOptions = {},
): BuilderReadPage {
  const lines = builderReadLines(content);
  const totalLines = lines.length;
  const revision = createHash('sha256').update(content).digest('hex');
  const offset = Math.min(
    Math.max(Math.floor(Number(options.offset) || 0), 0),
    totalLines,
  );
  const limit = Math.min(
    Math.max(Math.floor(Number(options.limit) || pi.DEFAULT_MAX_LINES), 1),
    pi.DEFAULT_MAX_LINES,
  );
  const byteOffset = Math.max(Math.floor(Number(options.byteOffset) || 0), 0);
  const contentByteLimit = Math.max(pi.DEFAULT_MAX_BYTES - 768, 1);

  function pageHeader(
    startLine: number,
    endLine: number,
    nextOffset: number | null,
    nextByte: number | null,
    byteRange?: BuilderReadPage['byteRange'],
  ) {
    const range = totalLines
      ? `lines ${startLine + 1}-${Math.max(startLine + 1, endLine)} of ${totalLines}`
      : 'empty file';
    const continuation =
      nextOffset === null
        ? 'End of file.'
        : `Continue with offset=${nextOffset}${nextByte ? ` and byteOffset=${nextByte}` : ''}.`;
    const chunk = byteRange
      ? `Line ${byteRange.line + 1} is ${byteRange.total} UTF-8 bytes; this page contains bytes ${byteRange.start}-${byteRange.end}.`
      : '';
    return `[Read page: ${range}; revision ${revision.slice(0, 16)}. ${chunk} ${continuation}]`;
  }

  if (offset >= totalLines) {
    return {
      text: pageHeader(offset, offset, null, null),
      revision,
      totalLines,
      nextOffset: null,
      nextByteOffset: null,
      complete: true,
    };
  }

  const firstLine = lines[offset];
  const firstLineBytes = Buffer.byteLength(firstLine, 'utf8');
  if (firstLineBytes > contentByteLimit) {
    if (byteOffset > 0 && byteOffset >= firstLineBytes)
      throw new Error(
        'byteOffset must point inside the oversized source line.',
      );
    const chunk = sliceUtf8ByBytes(firstLine, byteOffset, contentByteLimit);
    const lineComplete = chunk.end >= firstLineBytes;
    const nextOffset = lineComplete
      ? offset + 1 < totalLines
        ? offset + 1
        : null
      : offset;
    const nextByteOffset = lineComplete ? null : chunk.end;
    const byteRange = {
      line: offset,
      start: chunk.start,
      end: chunk.end,
      total: firstLineBytes,
    };
    const header = pageHeader(
      offset,
      offset + (lineComplete ? 1 : 0),
      nextOffset,
      nextByteOffset,
      byteRange,
    );
    return {
      text: `${header}\n${chunk.text}`,
      revision,
      totalLines,
      nextOffset,
      nextByteOffset,
      byteRange,
      complete: nextOffset === null,
    };
  }

  const selectedLines = lines.slice(offset, offset + limit);
  const truncation = pi.truncateHead(selectedLines.join('\n'), {
    maxBytes: contentByteLimit,
    maxLines: Math.min(limit, pi.DEFAULT_MAX_LINES),
  });
  const outputLines = truncation.outputLines;
  const nextOffset = offset + outputLines;
  const pageEnd = Math.min(offset + selectedLines.length, totalLines);
  const continuationOffset = truncation.truncated
    ? nextOffset
    : pageEnd < totalLines
      ? pageEnd
      : null;
  const header = pageHeader(
    offset,
    truncation.truncated ? nextOffset : pageEnd,
    continuationOffset,
    null,
  );
  return {
    text: truncation.content ? `${header}\n${truncation.content}` : header,
    revision,
    totalLines,
    nextOffset: continuationOffset,
    nextByteOffset: null,
    ...(outputLines > 0
      ? { coveredLines: { start: offset, end: nextOffset } }
      : {}),
    complete: continuationOffset === null,
  };
}

function builderReadLines(content: string) {
  const lines = content.split('\n');
  if (content.endsWith('\n')) lines.pop();
  return lines;
}

export function sliceUtf8ByBytes(
  content: string,
  byteOffset: number,
  maxBytes: number,
) {
  const totalBytes = Buffer.byteLength(content, 'utf8');
  const requestedStart = Math.min(byteOffset, totalBytes);
  const requestedEnd = Math.min(requestedStart + maxBytes, totalBytes);
  let byteCursor = 0;
  let textCursor = 0;
  let start = requestedStart;
  let end = requestedStart;
  let startIndex = content.length;
  let endIndex = content.length;
  for (const character of content) {
    const characterBytes = Buffer.byteLength(character, 'utf8');
    const characterEnd = byteCursor + characterBytes;
    if (characterEnd > requestedStart && byteCursor >= requestedStart) {
      if (characterEnd > requestedEnd) break;
      if (startIndex === content.length) {
        start = byteCursor;
        startIndex = textCursor;
      }
      end = characterEnd;
      endIndex = textCursor + character.length;
    }
    byteCursor = characterEnd;
    textCursor += character.length;
  }
  return {
    text: content.slice(startIndex, endIndex),
    start,
    end,
  };
}

export function recordExtensionReadCoverage(
  coverageByFilename: Map<string, ExtensionReadCoverage>,
  filename: string,
  page: BuilderReadPage,
) {
  let coverage = coverageByFilename.get(filename);
  if (!coverage || coverage.revision !== page.revision) {
    coverage = {
      revision: page.revision,
      totalLines: page.totalLines,
      completeLines: new Set(),
      lineByteRanges: new Map(),
    };
    coverageByFilename.set(filename, coverage);
  }

  if (page.coveredLines) {
    for (
      let line = page.coveredLines.start;
      line < page.coveredLines.end;
      line++
    )
      coverage.completeLines.add(line);
  }
  if (page.byteRange) {
    const ranges = coverage.lineByteRanges.get(page.byteRange.line) || [];
    ranges.push({ start: page.byteRange.start, end: page.byteRange.end });
    const merged = mergeByteRanges(ranges);
    coverage.lineByteRanges.set(page.byteRange.line, merged);
    if (
      merged.length === 1 &&
      merged[0].start === 0 &&
      merged[0].end >= page.byteRange.total
    )
      coverage.completeLines.add(page.byteRange.line);
  }

  if (coverage.completeLines.size >= coverage.totalLines) return true;
  return coverage.totalLines === 0;
}

function mergeByteRanges(ranges: Array<{ start: number; end: number }>) {
  const ordered = [...ranges].sort((left, right) => left.start - right.start);
  const merged: Array<{ start: number; end: number }> = [];
  for (const range of ordered) {
    const previous = merged.at(-1);
    if (previous && range.start <= previous.end)
      previous.end = Math.max(previous.end, range.end);
    else merged.push({ ...range });
  }
  return merged;
}

export function readPageDetails(page: BuilderReadPage) {
  return {
    revision: page.revision,
    totalLines: page.totalLines,
    nextOffset: page.nextOffset,
    nextByteOffset: page.nextByteOffset,
    complete: page.complete,
  };
}

export function formatBuilderValidationError(
  pi: PiTools,
  filename: string,
  error: unknown,
) {
  const message = error instanceof Error ? error.message : String(error);
  const maxBytes = Math.max(pi.DEFAULT_MAX_BYTES - 512, 1);
  const truncated = pi.truncateHead(message, {
    maxBytes,
    maxLines: pi.DEFAULT_MAX_LINES,
  });
  const diagnostic = truncated.firstLineExceedsLimit
    ? sliceUtf8ByBytes(message, 0, maxBytes).text
    : truncated.content;
  const continuation = truncated.truncated
    ? '\n[Validation diagnostics truncated. Re-run validate_extension after addressing the displayed errors.]'
    : '';
  return `${filename}:\n${diagnostic}${continuation}`;
}
