import { session } from 'electron';
import { canRequestMediaPermission } from '../os/os';
import { isTrustedAppPage } from './window-navigation-policy';

export function installPermissionHandlers(
  isDev: boolean,
  rendererUrl = '',
  rendererIndexPath?: string,
) {
  const allowedPermissions = [
    'media',
    'display-capture',
    'clipboard-read',
    'clipboard-sanitized-write',
  ];
  session.defaultSession.setPermissionCheckHandler(
    (webContents, permission) => {
      const url = webContents?.getURL() || '';
      return (
        isTrustedAppPage(url, isDev, rendererUrl, rendererIndexPath) &&
        allowedPermissions.includes(permission) &&
        canRequestMediaPermission(permission)
      );
    },
  );
  session.defaultSession.setPermissionRequestHandler(
    (webContents, permission, callback) => {
      const url = webContents.getURL();
      if (
        isTrustedAppPage(url, isDev, rendererUrl, rendererIndexPath) &&
        allowedPermissions.includes(permission) &&
        canRequestMediaPermission(permission)
      ) {
        return callback(true);
      }
      callback(false);
    },
  );
}
