type CopyImageAction = {
  path?: string;
  imagePath?: string;
  imageDataUrl?: string;
};

type CopyImageDeps = {
  clipboardApi: { writeImage: (image: any) => unknown };
  nativeImage: {
    createFromPath: (path: string) => any;
    createFromDataURL: (dataUrl: string) => any;
  };
  expandUserPath: (value: string) => string;
};

export const IMAGE_UNAVAILABLE_TOAST = {
  toast: { message: 'Image no longer available', tone: 'error' },
} as const;

/** Copies the action's image, refusing missing or empty images instead of clearing the clipboard. */
export async function copyImageAction(
  action: CopyImageAction,
  deps: CopyImageDeps,
) {
  const { nativeImage } = deps;
  const image = action.path
    ? nativeImage.createFromPath(deps.expandUserPath(action.path))
    : action.imagePath
      ? nativeImage.createFromPath(action.imagePath)
      : nativeImage.createFromDataURL(action.imageDataUrl as string);
  if (!image || image.isEmpty()) return IMAGE_UNAVAILABLE_TOAST;
  await deps.clipboardApi.writeImage(image);
}
