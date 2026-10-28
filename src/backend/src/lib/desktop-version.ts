type StrictDesktopVersion = {
  core: [string, string, string];
  prerelease: string[];
};

const STRICT_VERSION_PATTERN = /^v?(\d+)(?:\.(\d+))?(?:\.(\d+))?(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$/i;
const NUMERIC_IDENTIFIER_PATTERN = /^\d+$/;

export function compareVersions(left: string, right: string) {
  const leftVersion = parseStrictDesktopVersion(left);
  const rightVersion = parseStrictDesktopVersion(right);
  if (leftVersion && rightVersion) return compareStrictDesktopVersions(leftVersion, rightVersion);
  return compareLooseVersionParts(left, right);
}

function parseStrictDesktopVersion(version: string): StrictDesktopVersion | null {
  const match = STRICT_VERSION_PATTERN.exec(version);
  if (!match) return null;
  const [, major, minor, patch, prerelease] = match;
  return {
    core: [major, minor ?? '0', patch ?? '0'],
    prerelease: prerelease ? prerelease.split('.') : [],
  };
}

function compareStrictDesktopVersions(left: StrictDesktopVersion, right: StrictDesktopVersion) {
  const coreDiff = compareCoreVersions(left.core, right.core);
  if (coreDiff !== 0) return coreDiff;
  return comparePrereleaseIdentifiers(left.prerelease, right.prerelease);
}

function compareCoreVersions(left: [string, string, string], right: [string, string, string]) {
  for (let index = 0; index < left.length; index += 1) {
    const diff = compareNumericIdentifiers(left[index], right[index]);
    if (diff !== 0) return diff;
  }
  return 0;
}

// Semver precedence: a release outranks its prereleases, and identifiers compare left to right.
function comparePrereleaseIdentifiers(left: string[], right: string[]) {
  if (left.length === 0 && right.length === 0) return 0;
  if (left.length === 0) return 1;
  if (right.length === 0) return -1;
  for (let index = 0; index < Math.max(left.length, right.length); index += 1) {
    if (index >= left.length) return -1;
    if (index >= right.length) return 1;
    const diff = comparePrereleaseIdentifier(left[index], right[index]);
    if (diff !== 0) return diff;
  }
  return 0;
}

function comparePrereleaseIdentifier(left: string, right: string) {
  const leftIsNumeric = NUMERIC_IDENTIFIER_PATTERN.test(left);
  const rightIsNumeric = NUMERIC_IDENTIFIER_PATTERN.test(right);
  if (leftIsNumeric && rightIsNumeric) {
    return compareNumericIdentifiers(left, right);
  }
  if (leftIsNumeric !== rightIsNumeric) return leftIsNumeric ? -1 : 1;
  if (left === right) return 0;
  return left > right ? 1 : -1;
}

function compareNumericIdentifiers(left: string, right: string) {
  const normalizedLeft = left.replace(/^0+/, '') || '0';
  const normalizedRight = right.replace(/^0+/, '') || '0';
  if (normalizedLeft.length !== normalizedRight.length)
    return normalizedLeft.length > normalizedRight.length ? 1 : -1;
  if (normalizedLeft === normalizedRight) return 0;
  return normalizedLeft > normalizedRight ? 1 : -1;
}

// Legacy comparison kept for inputs that are not strict semver, so their results do not change.
function compareLooseVersionParts(left: string, right: string) {
  const leftParts = looseVersionParts(left);
  const rightParts = looseVersionParts(right);
  for (let index = 0; index < Math.max(leftParts.length, rightParts.length); index += 1) {
    const diff = (leftParts[index] || 0) - (rightParts[index] || 0);
    if (diff !== 0) return diff > 0 ? 1 : -1;
  }
  return 0;
}

function looseVersionParts(version: string) {
  return String(version || '')
    .replace(/^v/i, '')
    .split(/[.-]/)
    .map((part) => Number.parseInt(part, 10))
    .filter((part) => Number.isFinite(part));
}
