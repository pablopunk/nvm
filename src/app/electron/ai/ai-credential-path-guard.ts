import { realpathSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import type {
  ExtensionAPI,
  ExtensionFactory,
  ToolCallEvent,
  ToolCallEventResult,
  ToolResultEvent,
  ToolResultEventResult,
} from '@earendil-works/pi-coding-agent';

export type CredentialPathPolicy = {
  homeDir: string;
  extraDeniedPaths?: readonly string[];
  allowedPaths?: readonly string[];
  platform?: NodeJS.Platform;
};

type CanonicalCredentialPathPolicy = {
  homeDir: string;
  extraDeniedPaths: string[];
  allowedPaths: string[];
  platform: NodeJS.Platform;
};

export const CREDENTIAL_PATH_BLOCK_REASON =
  'Access to credential locations is blocked.';

const HOME_RELATIVE_CREDENTIAL_LOCATIONS = [
  '.ssh',
  '.aws',
  '.gnupg',
  '.config/gh',
  '.config/gcloud',
  '.azure',
  '.netrc',
  '.docker/config.json',
  '.kube',
  '.npmrc',
  '.pypirc',
  '.git-credentials',
  '.pi',
  'Library/Keychains',
  'Library/Cookies',
  'Library/Safari',
  'Library/Application Support/Google/Chrome',
  'Library/Application Support/BraveSoftware',
  'Library/Application Support/Firefox',
  'Library/Application Support/Microsoft Edge',
  '.config/google-chrome',
  '.mozilla',
] as const;

const NEVERMIND_PRIVATE_USER_DATA_ENTRIES = [
  'nevermind-auth.json',
  'nevermind-auth-by-origin.json',
  'byo-key.json',
  'extension-secrets',
] as const;

const PATH_GUARDED_TOOL_NAMES = new Set(['read', 'grep', 'find', 'ls']);
const SEARCH_RESULT_TOOL_NAMES = new Set(['grep', 'find', 'ls']);
const PI_NORMALIZED_UNICODE_SPACES = new RegExp(
  '[\\u00A0\\u2000-\\u200A\\u202F\\u205F\\u3000]',
  'g',
);
const GREP_LINE_PATH_BOUNDARY = /(?=[:-]\d+[:-] )/g;

export function resolveToolPath(
  rawPath: string,
  { cwd, homeDir }: { cwd: string; homeDir: string },
) {
  const expandedPath = expandToolPathPrefix(rawPath, homeDir);
  return canonicalizeExistingAncestor(path.resolve(cwd, expandedPath));
}

export function isCredentialPath(
  resolvedPath: string,
  policy: CredentialPathPolicy,
) {
  const platform = policy.platform ?? process.platform;
  const candidate = foldForPlatform(resolvedPath, platform);
  if (isEnvFileName(path.basename(candidate))) return true;
  if (isInsideAnyRoot(candidate, homeCredentialRoots(policy.homeDir), platform))
    return true;
  if (isInsideAnyRoot(candidate, policy.allowedPaths ?? [], platform))
    return false;
  return isInsideAnyRoot(candidate, policy.extraDeniedPaths ?? [], platform);
}

export function nevermindPrivatePaths(userDataDir: string, agentDir: string) {
  return [
    agentDir,
    ...NEVERMIND_PRIVATE_USER_DATA_ENTRIES.map((entry) =>
      path.join(userDataDir, entry),
    ),
  ];
}

export function createCredentialPathGuardExtension(
  policy: CredentialPathPolicy,
): { name: string; factory: ExtensionFactory } {
  const canonicalPolicy = canonicalizeCredentialPathPolicy(policy);
  return {
    name: 'nevermind-credential-path-guard',
    factory: function installCredentialPathGuard(pi: ExtensionAPI) {
      pi.on('tool_call', function blockCredentialPathToolCall(event, ctx) {
        return failOpenOnInternalError(() =>
          credentialPathToolCallResult(event, ctx.cwd, canonicalPolicy),
        );
      });
      pi.on('tool_result', function redactCredentialSearchResults(event, ctx) {
        return failOpenOnInternalError(() =>
          credentialFreeSearchResultPatch(event, ctx.cwd, canonicalPolicy),
        );
      });
    },
  };
}

function credentialPathToolCallResult(
  event: ToolCallEvent,
  cwd: string,
  policy: CanonicalCredentialPathPolicy,
): ToolCallEventResult | undefined {
  if (!PATH_GUARDED_TOOL_NAMES.has(event.toolName)) return undefined;
  const target = resolveToolPath(requestedSearchPath(event.input, cwd), {
    cwd,
    homeDir: policy.homeDir,
  });
  return isCredentialPath(target, policy)
    ? { block: true, reason: CREDENTIAL_PATH_BLOCK_REASON }
    : undefined;
}

function credentialFreeSearchResultPatch(
  event: ToolResultEvent,
  cwd: string,
  policy: CanonicalCredentialPathPolicy,
): ToolResultEventResult | undefined {
  if (!SEARCH_RESULT_TOOL_NAMES.has(event.toolName)) return undefined;
  const searchRoot = resolveToolPath(requestedSearchPath(event.input, cwd), {
    cwd,
    homeDir: policy.homeDir,
  });
  const content = withoutCredentialOutputLines(event.content, (line) =>
    searchOutputLinePathCandidates(event.toolName, line).some((candidate) =>
      isCredentialPath(resolveSearchResultPath(candidate, searchRoot), policy),
    ),
  );
  return content ? { content } : undefined;
}

function withoutCredentialOutputLines(
  content: ToolResultEvent['content'],
  isCredentialLine: (line: string) => boolean,
) {
  let removedAnyLine = false;
  const redactedContent = content.map((item) => {
    if (item.type !== 'text') return item;
    const lines = item.text.split('\n');
    const keptLines = lines.filter((line) => !isCredentialLine(line));
    if (keptLines.length === lines.length) return item;
    removedAnyLine = true;
    return { ...item, text: keptLines.join('\n') };
  });
  return removedAnyLine ? redactedContent : undefined;
}

function searchOutputLinePathCandidates(toolName: string, line: string) {
  if (toolName === 'grep') return grepOutputLinePathCandidates(line);
  const pathWithoutDirectorySuffix = line.replace(/\/$/, '');
  return pathWithoutDirectorySuffix ? [pathWithoutDirectorySuffix] : [];
}

/**
 * A grep line is `path:N: text` for matches and `path-N- text` for context. The
 * path may itself contain `:N:`, so every boundary yields a candidate prefix.
 */
function grepOutputLinePathCandidates(line: string) {
  return [...line.matchAll(GREP_LINE_PATH_BOUNDARY)]
    .map((match) => line.slice(0, match.index))
    .filter((candidate) => candidate.length > 0);
}

function resolveSearchResultPath(candidate: string, searchRoot: string) {
  return canonicalizeExistingAncestor(path.resolve(searchRoot, candidate));
}

function requestedSearchPath(input: unknown, cwd: string) {
  const requestedPath = (input as { path?: unknown }).path;
  return typeof requestedPath === 'string' ? requestedPath : cwd;
}

function expandToolPathPrefix(rawPath: string, homeDir: string) {
  const normalized = rawPath
    .replace(PI_NORMALIZED_UNICODE_SPACES, ' ')
    .replace(/^@/, '');
  if (normalized === '~') return homeDir;
  if (normalized.startsWith('~/'))
    return path.join(homeDir, normalized.slice(2));
  if (normalized.startsWith('file://')) return fileURLToPath(normalized);
  return normalized;
}

function canonicalizeExistingAncestor(absolutePath: string): string {
  const missingSegments: string[] = [];
  let current = absolutePath;
  for (;;) {
    try {
      return path.join(realpathSync(current), ...missingSegments);
    } catch {
      const parent = path.dirname(current);
      if (parent === current) return absolutePath;
      missingSegments.unshift(path.basename(current));
      current = parent;
    }
  }
}

function homeCredentialRoots(homeDir: string) {
  return HOME_RELATIVE_CREDENTIAL_LOCATIONS.map((location) =>
    path.resolve(homeDir, location),
  );
}

function canonicalizeCredentialPathPolicy(
  policy: CredentialPathPolicy,
): CanonicalCredentialPathPolicy {
  return {
    homeDir: canonicalOrOriginal(policy.homeDir),
    extraDeniedPaths: (policy.extraDeniedPaths ?? []).map(canonicalOrOriginal),
    allowedPaths: (policy.allowedPaths ?? []).map(canonicalOrOriginal),
    platform: policy.platform ?? process.platform,
  };
}

function canonicalOrOriginal(value: string) {
  try {
    return canonicalizeExistingAncestor(path.resolve(value));
  } catch {
    return value;
  }
}

function isInsideAnyRoot(
  candidate: string,
  roots: readonly string[],
  platform: NodeJS.Platform,
) {
  return roots.some((root) =>
    isInsidePath(candidate, foldForPlatform(path.resolve(root), platform)),
  );
}

function foldForPlatform(value: string, platform: NodeJS.Platform) {
  return platform === 'darwin' || platform === 'win32'
    ? value.toLowerCase()
    : value;
}

function isInsidePath(candidate: string, root: string) {
  return (
    candidate === root ||
    candidate.startsWith(root.endsWith(path.sep) ? root : `${root}${path.sep}`)
  );
}

function isEnvFileName(basename: string) {
  return basename === '.env' || basename.startsWith('.env.');
}

function failOpenOnInternalError<T>(read: () => T): T | undefined {
  try {
    return read();
  } catch {
    return undefined;
  }
}
