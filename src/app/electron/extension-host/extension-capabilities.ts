const EXTENSION_TRUST_DISCLOSURE =
  'This extension runs locally with full access to your computer when enabled. Declared capabilities describe intent; they are not security restrictions.';

interface ExtensionLike {
  id?: string;
  capabilities?: readonly string[] | null;
  /** @deprecated Kept for local extensions written before capabilities. */
  permissions?: readonly string[] | null;
}

interface DeclaredExtensionCapabilities {
  capabilities: readonly string[];
  provenance: 'capabilities' | 'legacy-permissions' | 'undeclared';
}

/** Resolves review metadata only. It must never be used as an access check. */
function declaredExtensionCapabilities(
  extension: ExtensionLike | null | undefined,
): DeclaredExtensionCapabilities {
  if (extension && Object.hasOwn(extension, 'capabilities')) {
    return {
      capabilities: Array.isArray(extension.capabilities)
        ? extension.capabilities.map(String)
        : [],
      provenance: 'capabilities',
    };
  }
  if (Array.isArray(extension?.permissions)) {
    return {
      capabilities: extension.permissions.map(String),
      provenance: 'legacy-permissions',
    };
  }
  return { capabilities: [], provenance: 'undeclared' };
}

const KNOWN_WEBVIEW_PERMISSIONS = new Set([
  'autoplay',
  'camera',
  'microphone',
  'display-capture',
  'clipboard-read',
  'clipboard-write',
]);

/** Browser iframe allowlisting is intentionally independent from declarations. */
function filterWebviewPermissionsForExtension(
  _extension: ExtensionLike | null | undefined,
  webviewPermissions: readonly string[] | undefined,
) {
  if (!webviewPermissions || webviewPermissions.length === 0) {
    return webviewPermissions;
  }
  return Array.from(new Set(webviewPermissions)).filter((permission) =>
    KNOWN_WEBVIEW_PERMISSIONS.has(permission),
  );
}

export type { DeclaredExtensionCapabilities, ExtensionLike };
export {
  declaredExtensionCapabilities,
  EXTENSION_TRUST_DISCLOSURE,
  filterWebviewPermissionsForExtension,
};
