// biome-ignore-all lint: Form field picker input is a dynamic extension payload moved verbatim from main.ts.
import { expandUserPath } from '../files/file-utils';

export type FormFieldPathPickerDeps = {
  BrowserWindow: { fromWebContents: (webContents: any) => any };
  dialog: {
    showOpenDialog: (
      window: any,
      options: any,
    ) => Promise<{ canceled: boolean; filePaths: string[] }>;
  };
  paletteWindow: { readonly win: any };
};

export function createFormFieldPathPicker(deps: FormFieldPathPickerDeps) {
  const { BrowserWindow, dialog, paletteWindow } = deps;

  async function pickFormFieldPaths(event, input: any = {}) {
    const senderWindow =
      BrowserWindow.fromWebContents(event.sender) ||
      paletteWindow.win ||
      undefined;
    const type =
      input.type === 'folder'
        ? 'folder'
        : input.type === 'files'
          ? 'files'
          : 'file';
    const properties: Array<
      'openFile' | 'openDirectory' | 'multiSelections' | 'createDirectory'
    > = type === 'folder' ? ['openDirectory'] : ['openFile'];
    if (type === 'files') properties.push('multiSelections');
    if (type === 'folder' && input.canCreateDirectories !== false)
      properties.push('createDirectory');
    const filters =
      Array.isArray(input.extensions) && input.extensions.length
        ? [
            {
              name: input.filterName || 'Allowed files',
              extensions: input.extensions
                .map((value) => String(value).replace(/^\./, ''))
                .filter(Boolean),
            },
          ]
        : undefined;
    const result = await dialog.showOpenDialog(senderWindow, {
      title:
        input.title ||
        (type === 'folder'
          ? 'Choose Folder'
          : type === 'files'
            ? 'Choose Files'
            : 'Choose File'),
      buttonLabel: input.buttonLabel || 'Choose',
      properties,
      filters,
      defaultPath:
        typeof input.defaultPath === 'string'
          ? expandUserPath(input.defaultPath)
          : undefined,
    });
    return result.canceled
      ? { canceled: true, paths: [] }
      : { canceled: false, paths: result.filePaths };
  }

  return { pickFormFieldPaths };
}
