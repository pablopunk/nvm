interface DragImage {
  isEmpty(): boolean;
  resize(options: {
    width: number;
    height: number;
    quality: 'good';
  }): DragImage;
}

interface DragImageFactory {
  createFromPath(filePath: string): DragImage;
  createFallback(dataUrl: string): DragImage;
}

const GENERIC_FILE_DRAG_ICON =
  'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAADAAAAAwCAIAAADYYG7QAAAABnRSTlMAAAAAAABupgeRAAAA9klEQVRYw2NgGAX4ASNBFdlFDdS1cmofPgPxOYjqTiHGWSzEaBYREVKUl6XcEfcfPn7z5h1+NThDCB48gX7umuoq1AqY6zfvrN+0E08gMREMGyq6hoGBQVNdRURECI8CAg6iSkyRZCYT0ebQCYw6aNRBow6iNSCq6oCD7XuOkGeNp4sNkSoHXQiNOogQIC0NEZ8UyAaDLoRGHUQIjJZDow6iFIyWQ6MOohTQqhwiO7UNuhAadRAhMFoOjTpo1EGjDkIF9x8+prqV+M0k4KA3b95dv3mHiq65fvMO/pFhogbOaTFOTdHA+Zs37wgOeFMLDLq5jkEHAG0sRE33acZ3AAAAAElFTkSuQmCC';

export function fileDragIconForPath(
  imageFactory: DragImageFactory,
  filePath: string,
) {
  const fileImage = imageFactory.createFromPath(filePath);
  if (!fileImage.isEmpty()) {
    return fileImage.resize({ width: 64, height: 64, quality: 'good' });
  }
  return imageFactory.createFallback(GENERIC_FILE_DRAG_ICON);
}
