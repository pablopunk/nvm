const MAX_BUNDLE_ID_BYTES = 255;
const BUNDLE_ID_COMPONENT = /^[A-Za-z0-9-]+$/;
const ASCII_DELETE = 0x7f;
const ASCII_SPACE = 0x20;

export const NEVERMIND_BUNDLE_ID = 'com.pablopunk.nvm';

export function normalizeBundleId(value: string) {
  return value.toLowerCase();
}

function hasUnsafeBundleCharacters(value: string) {
  return Array.from(value).some((character) => {
    const code = character.codePointAt(0) || 0;
    return (
      code <= ASCII_SPACE ||
      code === ASCII_DELETE ||
      character === '/' ||
      character === '\\'
    );
  });
}

export function validateBundleId(value: unknown): string | null {
  if (
    typeof value !== 'string' ||
    Buffer.byteLength(value, 'utf8') > MAX_BUNDLE_ID_BYTES
  ) {
    return null;
  }
  if (!value || hasUnsafeBundleCharacters(value)) {
    return null;
  }
  const components = value.split('.');
  if (
    components.length === 0 ||
    components.some((part) => !BUNDLE_ID_COMPONENT.test(part))
  ) {
    return null;
  }
  return value;
}

export function bundleIdsOverlap(left: string, right: string) {
  const normalizedLeft = normalizeBundleId(left);
  const normalizedRight = normalizeBundleId(right);
  return (
    normalizedLeft === normalizedRight ||
    normalizedLeft.startsWith(`${normalizedRight}.`) ||
    normalizedRight.startsWith(`${normalizedLeft}.`)
  );
}
