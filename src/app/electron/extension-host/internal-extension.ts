const INTERNAL_EXTENSION_MARKER = Symbol.for('nevermind.internalExtension');

export interface InternalExtensionLike {
  id?: string;
  [INTERNAL_EXTENSION_MARKER]?: true;
}

export function markInternalExtension<T extends InternalExtensionLike>(
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
  extension: InternalExtensionLike | null | undefined,
) {
  return extension?.[INTERNAL_EXTENSION_MARKER] === true;
}
