import path from 'node:path';

function sameExtensionSourceFile(
  currentFile: string | undefined,
  candidateFile: string,
) {
  return (
    Boolean(currentFile) &&
    path.resolve(currentFile) === path.resolve(candidateFile)
  );
}

export { sameExtensionSourceFile };
