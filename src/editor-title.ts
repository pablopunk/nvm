export function titleFromFirstContentLine(
  content: string,
  fallback = 'Untitled note',
) {
  const firstLine = content.split(/\r?\n/).find((line) => line.trim());
  const title = String(firstLine || '')
    .replace(/^(?:#{1,6}|>|[-+*]|\d+\.)\s*/, '')
    .replace(/^\[[ xX]\]\s*/, '')
    .replace(/!\[([^\]]*)\]\([^)]+\)/g, '$1')
    .replace(/\[([^\]]+)\]\([^)]+\)/g, '$1')
    .replace(/[*_~`]+/g, '')
    .trim()
    .replace(/\s+/g, ' ');
  return title.slice(0, 120) || fallback;
}
