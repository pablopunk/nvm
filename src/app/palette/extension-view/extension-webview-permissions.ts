export const EXTENSION_WEBVIEW_SANDBOX = 'allow-scripts allow-forms';
export const DEFAULT_EXTENSION_WEBVIEW_PERMISSIONS = ['autoplay'] as const;
export const EXTENSION_WEBVIEW_PERMISSION_ALLOW_VALUES = {
  autoplay: 'autoplay',
  camera: 'camera',
  microphone: 'microphone',
  'display-capture': 'display-capture',
  'clipboard-read': 'clipboard-read',
  'clipboard-write': 'clipboard-write',
} as const;
export const EXTENSION_WEBVIEW_ALLOW =
  DEFAULT_EXTENSION_WEBVIEW_PERMISSIONS.map(
    (permission) => EXTENSION_WEBVIEW_PERMISSION_ALLOW_VALUES[permission],
  ).join('; ');

export function extensionWebviewAllow(webviewPermissions?: readonly string[]) {
  const permissions = webviewPermissions?.length
    ? webviewPermissions
    : DEFAULT_EXTENSION_WEBVIEW_PERMISSIONS;
  return Array.from(new Set(permissions))
    .map(
      (permission) =>
        EXTENSION_WEBVIEW_PERMISSION_ALLOW_VALUES[
          permission as keyof typeof EXTENSION_WEBVIEW_PERMISSION_ALLOW_VALUES
        ],
    )
    .filter(Boolean)
    .join('; ');
}
