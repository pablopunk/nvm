const DEFAULT_FILE_INDEX_IGNORES = [
  'node_modules',
  '.git',
  'Library',
  'Applications',
];

export function normalizedIgnorePatterns(ignore) {
  const values = Array.isArray(ignore) ? ignore : ignore ? [ignore] : [];
  return [...DEFAULT_FILE_INDEX_IGNORES, ...values]
    .map((value) => String(value).trim())
    .filter(Boolean);
}

function wildcardPatternMatches(pattern, value) {
  if (!pattern.includes('*')) return false;
  const escaped = pattern
    .split('*')
    .map((part) => part.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'))
    .join('.*');
  return new RegExp(`^${escaped}$`, 'i').test(value);
}

export function ignoredByPattern(fullPath, name, patterns) {
  return patterns.some(
    (pattern) =>
      pattern === name ||
      wildcardPatternMatches(pattern, name) ||
      fullPath.includes(pattern),
  );
}
