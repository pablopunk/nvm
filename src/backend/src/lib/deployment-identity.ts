declare const __NVM_BUILD_COMMIT_SHA__: string | null | undefined;

const FULL_COMMIT_SHA_PATTERN = /^[0-9a-f]{40}$/;
const SHORT_VERSION_LENGTH = 7;
const UNKNOWN_SHORT_VERSION = 'dev';

export function isFullCommitSha(value: unknown): value is string {
  return typeof value === 'string' && FULL_COMMIT_SHA_PATTERN.test(value);
}

export function backendCommitSha(): string | null {
  const bakedIntoBuild = typeof __NVM_BUILD_COMMIT_SHA__ === 'undefined' ? undefined : __NVM_BUILD_COMMIT_SHA__;
  const candidates = [bakedIntoBuild, process.env.VERCEL_GIT_COMMIT_SHA];
  return candidates.find(isFullCommitSha) ?? null;
}

export function backendShortVersion(): string {
  return (backendCommitSha() || process.env.VERCEL_GIT_COMMIT_SHA)?.slice(0, SHORT_VERSION_LENGTH) || UNKNOWN_SHORT_VERSION;
}
