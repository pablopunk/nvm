const LINE_BREAK_PATTERN = /\r?\n/;
const MARKDOWN_PREFIX_PATTERN = /^(?:#{1,6}|>|[-+*]|\d+\.)\s*/;
const CHECKBOX_PREFIX_PATTERN = /^\[[ xX]\]\s*/;
const IMAGE_PATTERN = /!\[([^\]]*)\]\([^)]+\)/g;
const LINK_PATTERN = /\[([^\]]+)\]\([^)]+\)/g;
const MARKDOWN_DECORATION_PATTERN = /[*_~`]+/g;
const WHITESPACE_PATTERN = /\s+/g;
const MAX_TITLE_LENGTH = 120;

export function titleFromFirstContentLine(
  content: string,
  fallback = 'Untitled note',
) {
  const firstLine = content
    .split(LINE_BREAK_PATTERN)
    .find((line) => line.trim());
  const title = String(firstLine || '')
    .replace(MARKDOWN_PREFIX_PATTERN, '')
    .replace(CHECKBOX_PREFIX_PATTERN, '')
    .replace(IMAGE_PATTERN, '$1')
    .replace(LINK_PATTERN, '$1')
    .replace(MARKDOWN_DECORATION_PATTERN, '')
    .trim()
    .replace(WHITESPACE_PATTERN, ' ');
  return title.slice(0, MAX_TITLE_LENGTH) || fallback;
}
