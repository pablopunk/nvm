import { extensionContext } from './_context';

export function createClipboardExtension() {
  return extensionContext.clipboardService!.createClipboardExtension();
}
