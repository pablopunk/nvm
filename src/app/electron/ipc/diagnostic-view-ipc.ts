import type { IpcMain } from 'electron';
import { isTrustedAppPage } from '../app/window-navigation-policy';
import {
  recordAppView,
  rendererFailureHandle,
} from '../diagnostics/diagnostic-reporting';

export function registerDiagnosticViewIpc(deps: {
  ipcMain: IpcMain;
  isDev: boolean;
  rendererUrl: string | undefined;
  rendererIndexPath: string;
}) {
  function fromTrustedAppPage(
    event: Electron.IpcMainEvent | Electron.IpcMainInvokeEvent,
  ) {
    return (
      event.senderFrame === event.sender.mainFrame &&
      isTrustedAppPage(
        event.sender.getURL(),
        deps.isDev,
        deps.rendererUrl,
        deps.rendererIndexPath,
      )
    );
  }
  deps.ipcMain.handle('diagnostics:renderer-failure', (event) =>
    fromTrustedAppPage(event) ? rendererFailureHandle(event.sender.id) : null,
  );
  deps.ipcMain.on('diagnostics:app-view', (event, view: unknown) => {
    if (fromTrustedAppPage(event)) {
      recordAppView(view);
    }
  });
}
