const INTERNAL_EXTENSION_MARKER = Symbol.for('nevermind.internalExtension');

export type ExtensionLike = {
  id?: string;
  permissions?: readonly string[] | null;
  [INTERNAL_EXTENSION_MARKER]?: true;
};

export function markInternalExtension<T extends ExtensionLike>(
  extension: T,
): T {
  Object.defineProperty(extension, INTERNAL_EXTENSION_MARKER, {
    value: true,
    enumerable: false,
    configurable: false,
  });
  return extension;
}

export function isInternalExtension(
  extension: ExtensionLike | null | undefined,
) {
  return extension?.[INTERNAL_EXTENSION_MARKER] === true;
}

export function hasExtensionPermission(
  extension: ExtensionLike | null | undefined,
  permission: string,
) {
  const declared = Array.isArray(extension?.permissions)
    ? extension.permissions
    : null;
  if (declared) return declared.includes(permission);
  return isInternalExtension(extension);
}

export function permissionDeniedError(permission: string) {
  return new Error(`Extension is missing required permission: ${permission}`);
}

const WEBVIEW_PERMISSION_HOST_PERMISSIONS: Record<string, string[]> = {
  autoplay: [],
  camera: ['camera'],
  microphone: ['camera'],
  'display-capture': ['desktop.files'],
  'clipboard-read': ['clipboard.history'],
  'clipboard-write': ['clipboard.history'],
};

export function filterWebviewPermissionsForExtension(
  extension: ExtensionLike | null | undefined,
  webviewPermissions: readonly string[] | undefined,
) {
  if (!webviewPermissions?.length) return webviewPermissions;
  return Array.from(new Set(webviewPermissions)).filter((permission) => {
    const requiredPermissions = WEBVIEW_PERMISSION_HOST_PERMISSIONS[permission];
    if (!requiredPermissions) return false;
    return requiredPermissions.every((requiredPermission) =>
      hasExtensionPermission(extension, requiredPermission),
    );
  });
}

export function extensionPermissionCapabilities(
  extension: ExtensionLike | null | undefined,
) {
  return {
    canUseDesktopApps: hasExtensionPermission(extension, 'desktop.apps'),
    canUseDesktopFiles: hasExtensionPermission(extension, 'desktop.files'),
    canUseClipboard: hasExtensionPermission(extension, 'clipboard.history'),
    canUseSystem: hasExtensionPermission(extension, 'system'),
    canUseOcr: hasExtensionPermission(extension, 'ocr'),
    canUseUpdates: hasExtensionPermission(extension, 'updates'),
    canUseShortcuts: hasExtensionPermission(extension, 'shortcuts'),
    canUseAi: hasExtensionPermission(extension, 'ai'),
    canWriteSettings: hasExtensionPermission(extension, 'settings.write'),
    canManageExtensionOwnership: hasExtensionPermission(
      extension,
      'extensions.ownership',
    ),
  };
}
