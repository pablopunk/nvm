import type { BrowserWindow } from 'electron';

export function installDevToolsShortcut(target: BrowserWindow) {
  target.webContents.on('before-input-event', (_event, input) => {
    if (
      !((input.meta || input.control) && input.alt) ||
      input.key.toLowerCase() !== 'i'
    )
      return;
    if (target.webContents.isDevToolsOpened())
      target.webContents.closeDevTools();
    else target.webContents.openDevTools({ mode: 'detach' });
  });
}
