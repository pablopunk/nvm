import path from 'node:path';
import { nativeImage } from 'electron';
import { fileDragIconForPath } from './file-drag';
import { expandUserPath } from './file-utils';

export function startFileDrag(event, filePath) {
  const resolvedPath = expandUserPath(filePath);
  if (!(resolvedPath && path.isAbsolute(resolvedPath))) return;
  event.sender.startDrag({
    file: resolvedPath,
    icon: fileDragIconForPath(
      {
        createFromPath: (candidate) => nativeImage.createFromPath(candidate),
        createFallback: (dataUrl) => nativeImage.createFromDataURL(dataUrl),
      },
      resolvedPath,
    ),
  });
}
