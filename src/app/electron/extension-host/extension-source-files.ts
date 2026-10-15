export function isExtensionSourceFile(filename) {
  return (
    typeof filename === 'string' &&
    filename.endsWith('.ts') &&
    !filename.endsWith('.d.ts')
  );
}

export function compareExtensionSourceEntries(left, right) {
  if (left.name < right.name) return -1;
  if (left.name > right.name) return 1;
  return 0;
}
