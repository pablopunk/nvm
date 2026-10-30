import type { BrowserWindow } from 'electron';

const SPACE_CONFINEMENT_DELAY_MS = 80;

export function createPaletteSpaces() {
  let timer: ReturnType<typeof setTimeout> | undefined;

  function setVisibleOnAllSpaces(win: BrowserWindow, visible: boolean) {
    if (process.platform !== 'darwin') {
      return;
    }
    win.setVisibleOnAllWorkspaces(visible, {
      visibleOnFullScreen: true,
      skipTransformProcessType: true,
    });
  }

  function prepare(win: BrowserWindow) {
    clearTimeout(timer);
    setVisibleOnAllSpaces(win, true);
  }

  function confine(win: BrowserWindow) {
    timer = setTimeout(() => {
      if (!win.isDestroyed() && win.isVisible()) {
        setVisibleOnAllSpaces(win, false);
      }
    }, SPACE_CONFINEMENT_DELAY_MS);
  }

  function hide(win: BrowserWindow) {
    clearTimeout(timer);
    setVisibleOnAllSpaces(win, false);
  }

  return { prepare, confine, hide };
}
