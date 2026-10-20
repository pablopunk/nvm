import { measureDebugPerformance } from '../../diagnostics/debug-performance';
import { expandUserPath } from '../../files/file-utils';
import { frontmostApp, typeTextIntoFrontmostApp } from '../../os/os';
import { copyImageAction } from './copy-image-action';

export type ClipboardActionHandlerDeps = {
  clipboardApi: {
    writeText: (text: string) => unknown;
    writeImage: (image: any) => unknown;
  };
  nativeImage: {
    createFromPath: (path: string) => any;
    createFromDataURL: (dataUrl: string) => any;
  };
  paletteWindow: {
    readonly win: { isVisible: () => boolean } | null;
    hidePalette: () => Promise<void>;
  };
  clipboardPasteText: (action: any) => Promise<void>;
  pasteClipboardAction: (action: any) => Promise<void>;
  removeClipboardHistoryEntries: (action: any) => any;
};

export function createClipboardActionHandlers(
  deps: ClipboardActionHandlerDeps,
) {
  const {
    clipboardApi,
    nativeImage,
    paletteWindow,
    clipboardPasteText,
    pasteClipboardAction,
    removeClipboardHistoryEntries,
  } = deps;

  function desktopAppIdentity(appInfo: any) {
    return String(
      appInfo?.path || appInfo?.bundleId || appInfo?.id || appInfo?.name || '',
    );
  }

  async function pasteTextAction(action: any) {
    if (action?.title === 'Paste Dictation') {
      await measureDebugPerformance(
        'dictation.host-paste',
        {
          textLength: String(action.text || '').length,
          restoreClipboard: Boolean(action.restoreClipboard),
          alwaysLog: true,
        },
        () => clipboardPasteText(action),
      );
      return;
    }
    await clipboardPasteText(action);
  }

  async function handleCopyText(action) {
    await clipboardApi.writeText(action.text || '');
  }

  async function handlePasteText(action) {
    if (
      action.expectedFrontmostAppId &&
      String(action.expectedFrontmostAppId) !==
        desktopAppIdentity(await frontmostApp())
    )
      return {
        toast: {
          message: 'Frontmost app changed. Select the text and try again',
          tone: 'error',
        },
      };
    if (paletteWindow.win?.isVisible()) await paletteWindow.hidePalette();
    await pasteTextAction(action);
    return { toast: { message: 'Pasted' } };
  }

  async function handleInsertCharacter(action) {
    if (action.mode !== 'paste') {
      await clipboardApi.writeText(action.text || '');
      return;
    }
    if (paletteWindow.win?.isVisible()) await paletteWindow.hidePalette();
    await pasteTextAction({
      ...action,
      type: 'pasteText',
      concealed: true,
      restoreClipboard: true,
    });
    return { toast: { message: 'Pasted' } };
  }

  async function handlePasteClipboard(action) {
    if (paletteWindow.win?.isVisible()) await paletteWindow.hidePalette();
    await pasteClipboardAction(action);
    return { toast: { message: 'Pasted' } };
  }

  async function handleTypeText(action) {
    const result = await typeTextIntoFrontmostApp(action.text || '', {
      delayMs: action.delayMs,
    });
    return result?.ok
      ? { toast: { message: 'Typed' } }
      : {
          toast: {
            message: result?.error || 'Unable to type text',
            tone: 'error',
          },
        };
  }

  function handleCopyImage(action) {
    return copyImageAction(action, {
      clipboardApi,
      nativeImage,
      expandUserPath,
    });
  }

  function handleRemoveClipboardHistory(action) {
    return removeClipboardHistoryEntries(action);
  }

  return {
    handleCopyText,
    handlePasteText,
    handleInsertCharacter,
    handlePasteClipboard,
    handleTypeText,
    handleCopyImage,
    handleRemoveClipboardHistory,
  };
}
