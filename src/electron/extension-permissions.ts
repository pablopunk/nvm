export type ExtensionLike = { id?: string; permissions?: readonly string[] | null }

export function isInternalExtension(extension: ExtensionLike | null | undefined) {
  return typeof extension?.id === 'string' && extension.id.startsWith('nevermind.')
}

export function hasExtensionPermission(extension: ExtensionLike | null | undefined, permission: string) {
  const declared = Array.isArray(extension?.permissions) ? extension.permissions : null
  if (declared) return declared.includes(permission)
  return isInternalExtension(extension)
}

export function permissionDeniedError(permission: string) {
  return new Error(`Extension is missing required permission: ${permission}`)
}

export function extensionPermissionCapabilities(extension: ExtensionLike | null | undefined) {
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
    canManageExtensionOwnership: hasExtensionPermission(extension, 'extensions.ownership'),
  }
}
