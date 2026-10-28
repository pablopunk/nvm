import { realpath } from 'node:fs/promises';
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
};

type CanonicalCredentialPathPolicy = {
  homeDir: string;
  extraDeniedPaths: string[];
  allowedPaths: string[];
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

const ENV_TEMPLATE_FILE_NAMES = new Set([
  '.env.example',
  '.env.sample',
  '.env.template',
  '.env.dist',
  '.env.defaults',
]);

const PATH_GUARDED_TOOL_NAMES = new Set(['read', 'grep', 'find', 'ls']);
const SEARCH_RESULT_TOOL_NAMES = new Set(['grep', 'find', 'ls']);
const PI_NORMALIZED_UNICODE_SPACES = new RegExp(
  '[\\u00A0\\u2000-\\u200A\\u202F\\u205F\\u3000]',
  'g',
);
const GREP_LINE_PATH_BOUNDARY = /(?=[:-]\d+[:-] )/g;

export async function resolveToolPath(
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
  const candidate = foldPathCase(resolvedPath);
  if (pathNamesEnvFile(candidate)) return true;
  if (isInsideAnyRoot(candidate, homeCredentialRoots(policy.homeDir)))
    return true;
  if (isInsideAnyRoot(candidate, policy.allowedPaths ?? [])) return false;
  return isInsideAnyRoot(candidate, policy.extraDeniedPaths ?? []);
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
  const loadCanonicalPolicy = memoizeCanonicalCredentialPathPolicy(policy);
  return {
    name: 'nevermind-credential-path-guard',
    factory: function installCredentialPathGuard(pi: ExtensionAPI) {
      pi.on('tool_call', function blockCredentialPathToolCall(event, ctx) {
        return failOpenOnInternalErrorAsync(async () =>
          credentialPathToolCallResult(
            event,
            ctx.cwd,
            await loadCanonicalPolicy(),
          ),
        );
      });
      pi.on('tool_result', function redactCredentialSearchResults(event, ctx) {
        return failOpenOnInternalErrorAsync(async () =>
          credentialFreeSearchResultPatch(
            event,
            ctx.cwd,
            await loadCanonicalPolicy(),
          ),
        );
      });
    },
  };
}

function memoizeCanonicalCredentialPathPolicy(policy: CredentialPathPolicy) {
  let canonicalPolicy: Promise<CanonicalCredentialPathPolicy> | undefined;
  return function loadCanonicalCredentialPathPolicy() {
    canonicalPolicy ??= canonicalizeCredentialPathPolicy(policy);
    return canonicalPolicy;
  };
}

async function credentialPathToolCallResult(
  event: ToolCallEvent,
  cwd: string,
  policy: CanonicalCredentialPathPolicy,
): Promise<ToolCallEventResult | undefined> {
  if (!PATH_GUARDED_TOOL_NAMES.has(event.toolName)) return undefined;
  const requestedPath = requestedSearchPath(event.input, cwd);
  const target = await resolveToolPath(requestedPath, {
    cwd,
    homeDir: policy.homeDir,
  });
  return pathNamesEnvFile(
    expandToolPathPrefix(requestedPath, policy.homeDir),
  ) || isCredentialPath(target, policy)
    ? { block: true, reason: CREDENTIAL_PATH_BLOCK_REASON }
    : undefined;
}

async function credentialFreeSearchResultPatch(
  event: ToolResultEvent,
  cwd: string,
  policy: CanonicalCredentialPathPolicy,
): Promise<ToolResultEventResult | undefined> {
  if (!SEARCH_RESULT_TOOL_NAMES.has(event.toolName)) return undefined;
  const searchRoot = await resolveToolPath(
    requestedSearchPath(event.input, cwd),
    { cwd, homeDir: policy.homeDir },
  );
  const content = await withoutCredentialOutputLines(event.content, (line) =>
    isCredentialSearchOutputLine(event.toolName, line, searchRoot, policy),
  );
  return content ? { content } : undefined;
}

async function withoutCredentialOutputLines(
  content: ToolResultEvent['content'],
  isCredentialLine: (line: string) => Promise<boolean>,
) {
  let removedAnyLine = false;
  const redactedContent = await Promise.all(
    content.map(async (item) => {
      if (item.type !== 'text') return item;
      const lines = item.text.split('\n');
      const keptLines = await keptNonCredentialLines(lines, isCredentialLine);
      if (keptLines.length === lines.length) return item;
      removedAnyLine = true;
      return { ...item, text: keptLines.join('\n') };
    }),
  );
  return removedAnyLine ? redactedContent : undefined;
}

async function keptNonCredentialLines(
  lines: readonly string[],
  isCredentialLine: (line: string) => Promise<boolean>,
) {
  const credentialFlags = await Promise.all(
    lines.map((line) => isCredentialLine(line)),
  );
  return lines.filter((_line, index) => !credentialFlags[index]);
}

async function isCredentialSearchOutputLine(
  toolName: string,
  line: string,
  searchRoot: string,
  policy: CanonicalCredentialPathPolicy,
) {
  const candidates = searchOutputLinePathCandidates(toolName, line);
  if (candidates.some(pathNamesEnvFile)) return true;
  const resolvedCandidates = await Promise.all(
    candidates.map((candidate) =>
      resolveSearchResultPath(candidate, searchRoot),
    ),
  );
  return resolvedCandidates.some((resolved) =>
    isCredentialPath(resolved, policy),
  );
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

async function canonicalizeExistingAncestor(
  absolutePath: string,
): Promise<string> {
  const missingSegments: string[] = [];
  let current = absolutePath;
  for (;;) {
    try {
      return path.join(await realpath(current), ...missingSegments);
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

async function canonicalizeCredentialPathPolicy(
  policy: CredentialPathPolicy,
): Promise<CanonicalCredentialPathPolicy> {
  const [
    homeDir,
    extraDeniedPaths,
    canonicalHomeCredentialRoots,
    allowedPaths,
  ] = await Promise.all([
    canonicalOrOriginal(policy.homeDir),
    canonicalOrOriginals(policy.extraDeniedPaths ?? []),
    canonicalOrOriginals(homeCredentialRoots(policy.homeDir)),
    canonicalOrOriginals(policy.allowedPaths ?? []),
  ]);
  return {
    homeDir,
    extraDeniedPaths: [...extraDeniedPaths, ...canonicalHomeCredentialRoots],
    allowedPaths,
  };
}

function canonicalOrOriginals(values: readonly string[]) {
  return Promise.all(values.map((value) => canonicalOrOriginal(value)));
}

async function canonicalOrOriginal(value: string) {
  try {
    return await canonicalizeExistingAncestor(path.resolve(value));
  } catch {
    return value;
  }
}

function isInsideAnyRoot(candidate: string, roots: readonly string[]) {
  return roots.some((root) =>
    isInsidePath(candidate, foldPathCase(path.resolve(root))),
  );
}

function foldPathCase(value: string) {
  return value.toLowerCase();
}

function isInsidePath(candidate: string, root: string) {
  return (
    candidate === root ||
    candidate.startsWith(root.endsWith(path.sep) ? root : `${root}${path.sep}`)
  );
}

function pathNamesEnvFile(filePath: string) {
  return isEnvFileName(path.basename(filePath).toLowerCase());
}

function isEnvFileName(basename: string) {
  if (ENV_TEMPLATE_FILE_NAMES.has(basename)) return false;
  return basename === '.env' || basename.startsWith('.env.');
}

async function failOpenOnInternalErrorAsync<T>(
  read: () => Promise<T>,
): Promise<T | undefined> {
  try {
    return await read();
  } catch {
    return undefined;
  }
}
