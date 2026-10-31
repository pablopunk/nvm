import assert from 'node:assert/strict';
import test from 'node:test';
import { createFormFieldPathPicker } from './form-field-paths';

function pickerFor(senderWindow: object, paletteWin: object) {
  const held: string[] = [];
  const { pickFormFieldPaths } = createFormFieldPathPicker({
    BrowserWindow: { fromWebContents: () => senderWindow },
    dialog: {
      showOpenDialog: async () => ({
        canceled: false,
        filePaths: ['/home/user/Trips'],
      }),
    },
    paletteWindow: {
      win: paletteWin,
      keepOpenDuringNativeDialog: (showDialog) => {
        held.push('palette');
        return showDialog();
      },
    },
  });
  return { pickFormFieldPaths, held };
}

test('palette form pickers keep the palette open while the dialog is up', async () => {
  const palette = {};
  const { pickFormFieldPaths, held } = pickerFor(palette, palette);

  const result = await pickFormFieldPaths(
    { sender: {} },
    { type: 'folder', title: 'Folder' },
  );

  assert.deepEqual(result, { canceled: false, paths: ['/home/user/Trips'] });
  assert.deepEqual(held, ['palette']);
});

test('independent window pickers do not hold the palette', async () => {
  const { pickFormFieldPaths, held } = pickerFor({}, {});

  await pickFormFieldPaths({ sender: {} }, { type: 'file' });

  assert.deepEqual(held, []);
});
