import { declaredExtensionCapabilities } from './extension-capabilities';

const INTERNAL_EXTENSION_MARKER = Symbol.for('nevermind.internalExtension');

export interface ExtensionLike {
  id?: string;
  permissions?: readonly string[] | null;
  [INTERNAL_EXTENSION_MARKER]?: true;
}

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
  return declaredExtensionCapabilities(extension).capabilities.includes(
    permission,
  );
}

export function permissionDeniedError(permission: string) {
  return new Error(
    `Extension declaration does not include capability: ${permission}`,
  );
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
