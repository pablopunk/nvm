/**
 * A native dialog opened from the palette takes focus, which would otherwise dismiss the palette
 * and reset the view that asked for the dialog (for example, a form waiting for a folder).
 */
export function createNativeDialogHold(refocusPalette: () => void) {
  let openDialogs = 0;
  return {
    get active() {
      return openDialogs > 0;
    },
    async during<T>(showDialog: () => Promise<T>): Promise<T> {
      openDialogs += 1;
      try {
        return await showDialog();
      } finally {
        openDialogs -= 1;
        if (openDialogs === 0) refocusPalette();
      }
    },
  };
}
