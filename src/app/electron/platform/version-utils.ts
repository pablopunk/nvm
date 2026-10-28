const SEMVER_PATTERN =
  /^v?(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+[0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*)?$/;
const NUMERIC_IDENTIFIER_PATTERN = /^\d+$/;

export type SemverVersion = {
  core: [number, number, number];
  prerelease: string[];
};

export function parseSemverVersion(version: unknown): SemverVersion | null {
  const match = SEMVER_PATTERN.exec(String(version ?? ''));
  if (!match) return null;
  return {
    core: [Number(match[1]), Number(match[2]), Number(match[3])],
    prerelease: match[4] ? match[4].split('.') : [],
  };
}

function compareNumericIdentifiers(left: string, right: string) {
  if (left.length !== right.length)
    return Math.sign(left.length - right.length);
  return left === right ? 0 : left < right ? -1 : 1;
}

function comparePrereleaseIdentifier(left: string, right: string) {
  const leftNumeric = NUMERIC_IDENTIFIER_PATTERN.test(left);
  const rightNumeric = NUMERIC_IDENTIFIER_PATTERN.test(right);
  if (leftNumeric && rightNumeric)
    return compareNumericIdentifiers(left, right);
  if (leftNumeric !== rightNumeric) return leftNumeric ? -1 : 1;
  return left === right ? 0 : left < right ? -1 : 1;
}

function comparePrereleaseIdentifiers(left: string[], right: string[]) {
  if (!left.length || !right.length)
    return Math.sign(right.length - left.length);
  const length = Math.max(left.length, right.length);
  for (let index = 0; index < length; index += 1) {
    if (left[index] === undefined) return -1;
    if (right[index] === undefined) return 1;
    const difference = comparePrereleaseIdentifier(left[index], right[index]);
    if (difference !== 0) return difference;
  }
  return 0;
}

export function compareSemverVersions(left: unknown, right: unknown) {
  const leftVersion = parseSemverVersion(left);
  const rightVersion = parseSemverVersion(right);
  if (!leftVersion || !rightVersion) return null;
  for (let index = 0; index < 3; index += 1) {
    const difference = leftVersion.core[index] - rightVersion.core[index];
    if (difference !== 0) return Math.sign(difference);
  }
  return comparePrereleaseIdentifiers(
    leftVersion.prerelease,
    rightVersion.prerelease,
  );
}

export function versionParts(version: unknown) {
  return String(version || '')
    .split(/[.-]/)
    .map((part) => Number.parseInt(part, 10))
    .map((part) => (Number.isFinite(part) ? part : 0));
}

function legacyVersionIsNewer(version: unknown, current: unknown) {
  const nextParts = versionParts(version);
  const currentParts = versionParts(current);
  const length = Math.max(nextParts.length, currentParts.length);
  for (let index = 0; index < length; index += 1) {
    const next = nextParts[index] || 0;
    const existing = currentParts[index] || 0;
    if (next > existing) return true;
    if (next < existing) return false;
  }
  return false;
}

export function isNewerVersion(version: unknown, current: unknown) {
  const comparison = compareSemverVersions(version, current);
  if (comparison !== null) return comparison > 0;
  return legacyVersionIsNewer(version, current);
}
