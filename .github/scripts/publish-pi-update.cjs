const fs = require('node:fs/promises');
const os = require('node:os');
const path = require('node:path');
const { execFileSync } = require('node:child_process');
const {
  PACKAGE_FILES,
  PI_PACKAGES,
  compareVersions,
  packageFingerprint,
  packageVersionDelta,
  parseStableVersion,
  validateLockfile,
  validateManifestChanges,
} = require('./pi-update-utils.cjs');

const BOT_BRANCH = 'automation/pi-update';
const STATE_MARKER = '<!-- pi-update-state:';

function exactCandidateMarker(versions) {
  return `<!-- pi-update-candidate:${JSON.stringify(versions)} -->`;
}

function parseStateMarker(body) {
  const start = body.indexOf(STATE_MARKER);
  if (start < 0) return null;

  const jsonStart = start + STATE_MARKER.length;
  let depth = 0;
  let inString = false;
  let escaped = false;
  let jsonEnd = -1;
  for (let index = jsonStart; index < body.length; index += 1) {
    const character = body[index];
    if (inString) {
      if (escaped) escaped = false;
      else if (character === '\\') escaped = true;
      else if (character === '"') inString = false;
      continue;
    }
    if (character === '"') inString = true;
    else if (character === '{') depth += 1;
    else if (character === '}') {
      depth -= 1;
      if (depth === 0) {
        jsonEnd = index + 1;
        break;
      }
      if (depth < 0) return null;
    }
  }
  if (jsonEnd < 0 || !/^\s*-->/.test(body.slice(jsonEnd))) return null;
  try {
    return JSON.parse(body.slice(jsonStart, jsonEnd));
  } catch {
    return null;
  }
}

function sameCandidateVersions(left, right) {
  return PI_PACKAGES.every(
    (packageName) => left?.[packageName] === right?.[packageName],
  );
}

function candidateVersionsFromBody(body) {
  const markers = body
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(
      (line) =>
        line.startsWith('<!-- pi-update-candidate:') && line.endsWith(' -->'),
    );
  if (markers.length !== 1)
    throw new Error('Pi update PR must contain one candidate tuple marker.');
  return parseCandidateMarker(markers[0]);
}

function validateArtifact(snapshot, expected) {
  const { metadata, manifestText, lockText, fileNames } = snapshot;
  if (
    JSON.stringify([...fileNames].sort()) !==
    JSON.stringify(['metadata.json', ...PACKAGE_FILES].sort())
  )
    throw new Error('Update artifact contains unexpected or missing files.');
  if (
    metadata.schemaVersion !== 1 ||
    metadata.repository !== expected.repository ||
    metadata.runId !== expected.runId ||
    String(metadata.runAttempt) !== String(expected.runAttempt) ||
    metadata.baseSha !== expected.baseSha ||
    metadata.workflowSha !== expected.baseSha
  )
    throw new Error(
      'Update artifact run, repository, or base provenance does not match.',
    );
  for (const packageName of PI_PACKAGES) {
    const baseVersion = metadata.baseVersions?.[packageName];
    const candidateVersion = metadata.candidateVersions?.[packageName];
    if (
      !parseStableVersion(baseVersion) ||
      !parseStableVersion(candidateVersion) ||
      baseVersion === candidateVersion ||
      compareVersions(candidateVersion, baseVersion) <= 0 ||
      metadata.candidateVersions[PI_PACKAGES[0]] !== candidateVersion
    )
      throw new Error(
        `Artifact contains an invalid version tuple for ${packageName}.`,
      );
  }
  const baseManifest = expected.baseManifestText;
  validateManifestChanges(
    baseManifest,
    manifestText,
    metadata.baseVersions,
    metadata.candidateVersions,
  );
  validateLockfile(lockText, metadata.candidateVersions, expected.baseLockText);
  const fingerprint = packageFingerprint(manifestText, lockText);
  if (metadata.fingerprint !== fingerprint)
    throw new Error(
      'Update artifact fingerprint does not match its package files.',
    );
  const delta = packageVersionDelta(expected.baseLockText, lockText);
  if (JSON.stringify(delta) !== JSON.stringify(metadata.transitiveDelta))
    throw new Error(
      'Update artifact transitive package delta does not match its lockfile.',
    );
  return {
    metadata,
    manifestText,
    lockText,
    fingerprint,
    transitiveDelta: delta,
  };
}

function safeDeltaText(changes) {
  if (changes.length === 0) return 'No transitive package versions changed.';
  return changes
    .map((change) => {
      const name = safeMarkdownToken(change.name);
      const from = change.from.map(safeMarkdownToken).join(', ') || 'new';
      const to = change.to.map(safeMarkdownToken).join(', ') || 'removed';
      return `- \`${name}\`: ${from} → ${to}`;
    })
    .join('\n');
}

function safeMarkdownToken(value) {
  const stringValue = String(value);
  if (!/^[A-Za-z0-9@/_.+-]+$/.test(stringValue))
    throw new Error(
      'Package delta contains a value that cannot be rendered safely.',
    );
  return stringValue;
}

function candidateTitle(versions, baseVersions) {
  const candidate = versions[PI_PACKAGES[0]];
  const previous = baseVersions[PI_PACKAGES[0]];
  const candidateMajor = Number(candidate.split('.')[0]);
  const previousMajor = Number(previous.split('.')[0]);
  return `chore(deps): update Pi SDK to ${candidate}${candidateMajor > previousMajor ? ' (major)' : ''}`;
}

function candidateBody(validated, generatedCommitSha) {
  const { metadata, transitiveDelta, fingerprint } = validated;
  const versions = metadata.candidateVersions;
  const candidate = versions[PI_PACKAGES[0]];
  const previous = metadata.baseVersions[PI_PACKAGES[0]];
  const major =
    Number(candidate.split('.')[0]) > Number(previous.split('.')[0]);
  const tupleMarker = exactCandidateMarker(versions);
  const state = {
    baseSha: metadata.baseSha,
    fingerprint,
    candidateVersions: versions,
    generatedCommitSha,
  };
  return [
    `Pi SDK update: ${previous} → ${candidate}`,
    '',
    `- \`${PI_PACKAGES[0]}\`: \`${previous}\` → \`${candidate}\``,
    `- \`${PI_PACKAGES[1]}\`: \`${previous}\` → \`${candidate}\``,
    `- [Pi ${candidate} release notes](https://github.com/earendil-works/pi/releases/tag/v${candidate})`,
    `- Review classification: ${major ? 'major; targeted API, tool, provider, auth, and packaging review required' : 'routine update; skim the upstream release notes'}`,
    '- Existing pnpm Pi graph overrides remain unchanged and require human compatibility review.',
    `- Generated commit: pending`,
    `- Required CI: pending on this bot-created PR head`,
    '',
    '### Transitive package version changes',
    '',
    safeDeltaText(transitiveDelta),
    '',
    tupleMarker,
    `${STATE_MARKER}${JSON.stringify(state)} -->`,
  ].join('\n');
}

function setGeneratedCommit(body, commitSha) {
  return body.replace(
    'Generated commit: pending',
    `Generated commit: ${commitSha}`,
  );
}

function readState(body, expectedCandidateVersions) {
  const state = parseStateMarker(body);
  let candidateVersions;
  try {
    candidateVersions = state?.candidateVersions
      ? validateCandidateVersions(state.candidateVersions)
      : parseCandidateMarker(state?.candidateMarker);
  } catch {
    candidateVersions = null;
  }
  if (
    !state ||
    !candidateVersions ||
    !sameCandidateVersions(candidateVersions, expectedCandidateVersions)
  )
    throw new Error(
      'Open Pi update PR has an invalid bot state marker; inspect it manually.',
    );
  if (!/^[a-f0-9]{64}$/.test(state.fingerprint || ''))
    throw new Error('Open Pi update PR has an invalid generated fingerprint.');
  if (!/^[a-f0-9]{40}$/.test(state.baseSha || ''))
    throw new Error('Open Pi update PR has an invalid original base SHA.');
  if (!/^[a-f0-9]{40}$/.test(state.generatedCommitSha || ''))
    throw new Error('Open Pi update PR has an invalid generated commit SHA.');
  return { ...state, candidateVersions };
}

async function readArtifact(directory, expected) {
  const fileNames = await fs.readdir(directory);
  const metadata = JSON.parse(
    await fs.readFile(path.join(directory, 'metadata.json'), 'utf8'),
  );
  const manifestText = await fs.readFile(
    path.join(directory, 'package.json'),
    'utf8',
  );
  const lockText = await fs.readFile(
    path.join(directory, 'pnpm-lock.yaml'),
    'utf8',
  );
  return validateArtifact(
    { metadata, manifestText, lockText, fileNames },
    expected,
  );
}

function githubClient(token, repository, dependencies = {}) {
  const [owner, name] = repository.split('/');
  const fetchImplementation = dependencies.fetchImplementation || fetch;
  const apiBaseUrl = dependencies.apiBaseUrl || 'https://api.github.com';
  const headers = {
    accept: 'application/vnd.github+json',
    authorization: `Bearer ${token}`,
    'x-github-api-version': '2022-11-28',
  };
  async function request(endpoint, options = {}) {
    const response = await fetchImplementation(`${apiBaseUrl}${endpoint}`, {
      ...options,
      headers: {
        ...headers,
        ...(options.body ? { 'content-type': 'application/json' } : {}),
        ...options.headers,
      },
      ...(options.body ? { body: JSON.stringify(options.body) } : {}),
    });
    if (response.status === 204) return { data: null, response };
    const text = await response.text();
    let data;
    try {
      data = text ? JSON.parse(text) : null;
    } catch {
      throw new Error(
        `GitHub API returned an invalid response for ${endpoint}.`,
      );
    }
    if (!response.ok)
      throw new Error(
        `GitHub API ${options.method || 'GET'} ${endpoint} failed (${response.status}): ${data?.message || 'unknown error'}`,
      );
    return { data, response };
  }
  async function paginated(endpoint) {
    const results = [];
    let url = endpoint;
    while (url) {
      const { data, response } = await request(url);
      if (!Array.isArray(data))
        throw new Error(`GitHub API expected a list at ${url}.`);
      results.push(...data);
      const next = /<([^>]+)>;\s*rel="next"/.exec(
        response.headers.get('link') || '',
      );
      url = next ? new URL(next[1]).pathname + new URL(next[1]).search : '';
    }
    return results;
  }
  return { owner, name, paginated, request };
}

async function readGitHubFile(client, ref, filePath) {
  const endpoint = `/repos/${client.owner}/${client.name}/contents/${filePath}?ref=${encodeURIComponent(ref)}`;
  const { data } = await client.request(endpoint);
  if (data?.type !== 'file' || typeof data.content !== 'string')
    throw new Error(`Bot branch does not contain ${filePath} as a data file.`);
  return Buffer.from(data.content.replaceAll('\n', ''), 'base64').toString(
    'utf8',
  );
}

async function verifyGeneratedCommit(client, pullRequest, state) {
  const { data: commit } = await client.request(
    `/repos/${client.owner}/${client.name}/commits/${state.generatedCommitSha}`,
  );
  const botLogin = pullRequest.user?.login;
  const candidateVersions = state.candidateVersions;
  if (
    !botLogin ||
    commit.author?.login !== botLogin ||
    !commit.parents?.some((parent) => parent.sha === state.baseSha) ||
    !String(commit.commit?.message || '').startsWith(
      `chore(deps): update Pi SDK to ${candidateVersions[PI_PACKAGES[0]]}`,
    )
  )
    throw new Error(
      'Pi update PR generated commit is not the recorded bot commit from its original base.',
    );
}

function parseCandidateMarker(marker) {
  const prefix = '<!-- pi-update-candidate:';
  if (
    typeof marker !== 'string' ||
    !marker.startsWith(prefix) ||
    !marker.endsWith(' -->')
  )
    throw new Error('Pi update PR has an invalid candidate tuple marker.');
  let versions;
  try {
    versions = JSON.parse(marker.slice(prefix.length, -4));
  } catch {
    throw new Error('Pi update PR candidate tuple marker is not valid JSON.');
  }
  return validateCandidateVersions(versions);
}

function validateCandidateVersions(versions) {
  const expectedKeys = [...PI_PACKAGES].sort();
  const actualKeys = Object.keys(versions || {}).sort();
  if (
    JSON.stringify(actualKeys) !== JSON.stringify(expectedKeys) ||
    !PI_PACKAGES.every(
      (packageName) =>
        parseStableVersion(versions[packageName]) &&
        versions[packageName] === versions[PI_PACKAGES[0]],
    )
  )
    throw new Error(
      'Pi update PR candidate tuple must use one stable Pi family version.',
    );
  return versions;
}

function mergeGeneratedFileWithCurrentBase(
  generatedText,
  originalBaseText,
  currentBaseText,
) {
  const directory = fs.mkdtemp(path.join(os.tmpdir(), 'pi-update-merge-'));
  return directory.then(async (temporaryDirectory) => {
    const generatedPath = path.join(temporaryDirectory, 'generated');
    const originalBasePath = path.join(temporaryDirectory, 'original-base');
    const currentBasePath = path.join(temporaryDirectory, 'current-base');
    try {
      await Promise.all([
        fs.writeFile(generatedPath, generatedText),
        fs.writeFile(originalBasePath, originalBaseText),
        fs.writeFile(currentBasePath, currentBaseText),
      ]);
      try {
        return execFileSync(
          'git',
          [
            'merge-file',
            '-p',
            '-L',
            'bot-generated',
            '-L',
            'original-base',
            '-L',
            'current-base',
            generatedPath,
            originalBasePath,
            currentBasePath,
          ],
          { encoding: 'utf8' },
        );
      } catch (error) {
        if (error && error.status === 1)
          return mergeDisjointLineEdits(
            generatedPath,
            originalBasePath,
            currentBasePath,
            originalBaseText,
          );
        throw error;
      }
    } finally {
      await fs.rm(temporaryDirectory, { recursive: true, force: true });
    }
  });
}

function mergeDisjointLineEdits(
  generatedPath,
  originalBasePath,
  currentBasePath,
  originalBaseText,
) {
  const generatedEdits = gitLineEdits(originalBasePath, generatedPath);
  const currentBaseEdits = gitLineEdits(originalBasePath, currentBasePath);
  const edits = combineLineEdits(generatedEdits, currentBaseEdits);
  if (!edits) return null;
  return applyLineEdits(originalBaseText, edits);
}

function gitLineEdits(basePath, changedPath) {
  let diff;
  try {
    diff = execFileSync(
      'git',
      [
        'diff',
        '--no-index',
        '--no-ext-diff',
        '--no-color',
        '--unified=0',
        '--',
        basePath,
        changedPath,
      ],
      { encoding: 'utf8' },
    );
  } catch (error) {
    if (error && error.status === 1) diff = String(error.stdout || '');
    else throw error;
  }
  return parseUnifiedDiffEdits(diff);
}

function parseUnifiedDiffEdits(diff) {
  const lines = diff.split('\n');
  const edits = [];
  for (let index = 0; index < lines.length; index += 1) {
    const header = /^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@/.exec(
      lines[index],
    );
    if (!header) continue;
    const oldStart = Number(header[1]);
    const oldCount = Number(header[2] || 1);
    const newCount = Number(header[4] || 1);
    const removed = [];
    const added = [];
    for (index += 1; index < lines.length; index += 1) {
      const line = lines[index];
      if (line.startsWith('@@ ')) {
        index -= 1;
        break;
      }
      if (line.startsWith('\\ No newline at end of file')) continue;
      if (line.startsWith('-')) removed.push(line.slice(1));
      else if (line.startsWith('+')) added.push(line.slice(1));
      else if (line !== '')
        throw new Error('Could not safely parse a zero-context line diff.');
    }
    if (removed.length !== oldCount || added.length !== newCount)
      throw new Error('Git line diff hunk did not match its declared counts.');
    const start = oldCount === 0 ? oldStart : oldStart - 1;
    edits.push({ start, end: start + oldCount, replacement: added });
  }
  return edits;
}

function combineLineEdits(first, second) {
  const combined = [...first];
  for (const candidate of second) {
    let duplicate = false;
    for (const existing of combined) {
      if (sameLineEdit(existing, candidate)) {
        duplicate = true;
        break;
      }
      if (lineEditsConflict(existing, candidate)) return null;
    }
    if (!duplicate) combined.push(candidate);
  }
  return combined.sort(function sortLineEdits(left, right) {
    return left.start - right.start || left.end - right.end;
  });
}

function sameLineEdit(left, right) {
  return (
    left.start === right.start &&
    left.end === right.end &&
    JSON.stringify(left.replacement) === JSON.stringify(right.replacement)
  );
}

function lineEditsConflict(left, right) {
  const leftInsert = left.start === left.end;
  const rightInsert = right.start === right.end;
  if (leftInsert || rightInsert) {
    if (leftInsert && rightInsert) return left.start === right.start;
    const insertion = leftInsert ? left : right;
    const range = leftInsert ? right : left;
    return insertion.start >= range.start && insertion.start <= range.end;
  }
  return Math.max(left.start, right.start) < Math.min(left.end, right.end);
}

function applyLineEdits(originalText, edits) {
  const sourceLines = originalText.split('\n');
  const mergedLines = [];
  let cursor = 0;
  for (const edit of edits) {
    if (edit.start < cursor || edit.start > sourceLines.length) return null;
    mergedLines.push(...sourceLines.slice(cursor, edit.start));
    mergedLines.push(...edit.replacement);
    cursor = edit.end;
  }
  mergedLines.push(...sourceLines.slice(cursor));
  return mergedLines.join('\n');
}

async function verifyOpenPullRequest(client, pullRequest, currentBaseFiles) {
  const currentBranchSha = await botBranchHeadSha(client);
  if (currentBranchSha !== pullRequest.head.sha)
    throw new Error(
      'Pi update PR head differs from the current bot branch; inspect the branch before updating it.',
    );
  const candidateVersions = candidateVersionsFromBody(pullRequest.body || '');
  const state = readState(pullRequest.body || '', candidateVersions);
  await verifyGeneratedCommit(client, pullRequest, state);
  const files = await client.paginated(
    `/repos/${client.owner}/${client.name}/pulls/${pullRequest.number}/files?per_page=100`,
  );
  if (files.some((file) => !PACKAGE_FILES.includes(file.filename)))
    throw new Error(
      'Pi update PR has a changed path outside the bot-owned package manifest and lockfile.',
    );
  const [
    branchManifest,
    branchLock,
    originalManifest,
    originalLock,
    generatedManifest,
    generatedLock,
  ] = await Promise.all([
    readGitHubFile(client, pullRequest.head.sha, 'package.json'),
    readGitHubFile(client, pullRequest.head.sha, 'pnpm-lock.yaml'),
    readGitHubFile(client, state.baseSha, 'package.json'),
    readGitHubFile(client, state.baseSha, 'pnpm-lock.yaml'),
    readGitHubFile(client, state.generatedCommitSha, 'package.json'),
    readGitHubFile(client, state.generatedCommitSha, 'pnpm-lock.yaml'),
  ]);
  if (
    packageFingerprint(generatedManifest, generatedLock) !== state.fingerprint
  )
    throw new Error(
      'Pi update PR generated commit does not match its bot fingerprint.',
    );
  const manifest = JSON.parse(branchManifest);
  for (const packageName of PI_PACKAGES) {
    if (manifest.dependencies?.[packageName] !== candidateVersions[packageName])
      throw new Error(
        `Pi update PR manifest changed its recorded pin for ${packageName}.`,
      );
  }
  validateLockfile(branchLock, candidateVersions, currentBaseFiles.lockText);
  const [expectedManifest, expectedLock] = await Promise.all([
    mergeGeneratedFileWithCurrentBase(
      generatedManifest,
      originalManifest,
      currentBaseFiles.manifestText,
    ),
    mergeGeneratedFileWithCurrentBase(
      generatedLock,
      originalLock,
      currentBaseFiles.lockText,
    ),
  ]);
  const matchesGenerated =
    branchManifest === generatedManifest && branchLock === generatedLock;
  const matchesCleanBaseMerge =
    expectedManifest !== null &&
    expectedLock !== null &&
    branchManifest === expectedManifest &&
    branchLock === expectedLock;
  if (!matchesGenerated && !matchesCleanBaseMerge)
    throw new Error(
      'Pi update PR package changes differ from its recorded bot output or a clean merge of the current base; human edits require review.',
    );
  const fingerprint = packageFingerprint(branchManifest, branchLock);
  return { ...state, branchFingerprint: fingerprint };
}

async function closedUnmergedTupleExists(client, marker) {
  const pulls = await client.paginated(
    `/repos/${client.owner}/${client.name}/pulls?state=closed&per_page=100`,
  );
  return pulls.some(
    (pullRequest) =>
      !pullRequest.merged_at &&
      typeof pullRequest.body === 'string' &&
      pullRequest.body.includes(marker),
  );
}

async function listOpenBotPullRequests(client) {
  return client.paginated(
    `/repos/${client.owner}/${client.name}/pulls?state=open&head=${encodeURIComponent(`${client.owner}:${BOT_BRANCH}`)}&per_page=100`,
  );
}

async function listClosedBotPullRequests(client) {
  return client.paginated(
    `/repos/${client.owner}/${client.name}/pulls?state=closed&head=${encodeURIComponent(`${client.owner}:${BOT_BRANCH}`)}&sort=updated&direction=desc&per_page=100`,
  );
}

async function botBranchHeadSha(client) {
  const { data } = await client.request(
    `/repos/${client.owner}/${client.name}/git/ref/heads/${BOT_BRANCH}`,
  );
  if (!/^[a-f0-9]{40}$/.test(data?.object?.sha || ''))
    throw new Error('Pi update bot branch has an invalid head SHA.');
  return data.object.sha;
}

async function verifyClosedBotBranch(client, repository, currentBaseFiles) {
  const branchSha = await botBranchHeadSha(client);
  const closedPullRequests = await listClosedBotPullRequests(client);
  const previousPullRequest = closedPullRequests.find(
    (pullRequest) =>
      pullRequest.head?.ref === BOT_BRANCH &&
      pullRequest.head?.repo?.full_name === repository,
  );
  if (!previousPullRequest)
    throw new Error(
      'Pi update bot branch exists without a closed bot PR to verify its contents; inspect it before replacement.',
    );
  if (previousPullRequest.head.sha !== branchSha)
    throw new Error(
      'Closed Pi update PR head differs from the current bot branch; inspect the branch before replacement.',
    );
  return verifyOpenPullRequest(client, previousPullRequest, currentBaseFiles);
}

async function writeBranchCommit(client, baseSha, validated) {
  const { data: baseCommit } = await client.request(
    `/repos/${client.owner}/${client.name}/git/commits/${baseSha}`,
  );
  async function createBlob(content) {
    const { data } = await client.request(
      `/repos/${client.owner}/${client.name}/git/blobs`,
      { method: 'POST', body: { content, encoding: 'utf-8' } },
    );
    return data.sha;
  }
  const [manifestSha, lockSha] = await Promise.all([
    createBlob(validated.manifestText),
    createBlob(validated.lockText),
  ]);
  const { data: tree } = await client.request(
    `/repos/${client.owner}/${client.name}/git/trees`,
    {
      method: 'POST',
      body: {
        base_tree: baseCommit.tree.sha,
        tree: [
          {
            path: 'package.json',
            mode: '100644',
            type: 'blob',
            sha: manifestSha,
          },
          {
            path: 'pnpm-lock.yaml',
            mode: '100644',
            type: 'blob',
            sha: lockSha,
          },
        ],
      },
    },
  );
  const version = validated.metadata.candidateVersions[PI_PACKAGES[0]];
  const { data: commit } = await client.request(
    `/repos/${client.owner}/${client.name}/git/commits`,
    {
      method: 'POST',
      body: {
        message: `chore(deps): update Pi SDK to ${version}`,
        tree: tree.sha,
        parents: [baseSha],
      },
    },
  );
  const refPath = `/repos/${client.owner}/${client.name}/git/ref/heads/${BOT_BRANCH}`;
  const branch = await client
    .request(refPath)
    .catch(function branchNotFound(error) {
      if (!String(error.message).includes('(404)')) throw error;
      return null;
    });
  if (branch) {
    await client.request(refPath, {
      method: 'PATCH',
      body: { sha: commit.sha, force: true },
    });
  } else {
    await client.request(`/repos/${client.owner}/${client.name}/git/refs`, {
      method: 'POST',
      body: { ref: `refs/heads/${BOT_BRANCH}`, sha: commit.sha },
    });
  }
  return commit.sha;
}

async function validateCandidateArtifact(
  directory,
  environment = process.env,
  trustedRoot = process.cwd(),
) {
  const repository = environment.GITHUB_REPOSITORY;
  const baseSha = environment.PI_UPDATE_BASE_SHA || environment.GITHUB_SHA;
  if (!repository || !baseSha || !environment.GITHUB_RUN_ID)
    throw new Error(
      'Publisher requires repository, trusted base SHA, and workflow run provenance.',
    );
  const baseManifestText = await fs.readFile(
    path.join(trustedRoot, 'package.json'),
    'utf8',
  );
  const baseLockText = await fs.readFile(
    path.join(trustedRoot, 'pnpm-lock.yaml'),
    'utf8',
  );
  const validated = await readArtifact(directory, {
    repository,
    runId: environment.GITHUB_RUN_ID,
    runAttempt: environment.GITHUB_RUN_ATTEMPT,
    baseSha,
    baseManifestText,
    baseLockText,
  });
  return {
    ...validated,
    currentBaseFiles: {
      manifestText: baseManifestText,
      lockText: baseLockText,
    },
  };
}

async function publishCandidate(
  directory,
  environment = process.env,
  dependencies = {},
) {
  const repository = environment.GITHUB_REPOSITORY;
  const readToken = environment.GITHUB_TOKEN;
  const writeToken = environment.GH_APP_TOKEN;
  const baseSha = environment.PI_UPDATE_BASE_SHA || environment.GITHUB_SHA;
  const defaultBranch = environment.GITHUB_DEFAULT_BRANCH || 'main';
  if (!repository || !readToken || !writeToken || !baseSha)
    throw new Error(
      'Publisher requires repository, trusted base SHA, a read token, and scoped GitHub App token.',
    );
  const validated = await validateCandidateArtifact(
    directory,
    environment,
    dependencies.trustedRoot || process.cwd(),
  );
  const readClient = githubClient(readToken, repository, dependencies);
  const writeClient = githubClient(writeToken, repository, dependencies);
  const marker = exactCandidateMarker(validated.metadata.candidateVersions);
  if (await closedUnmergedTupleExists(readClient, marker)) {
    console.log(
      'This exact Pi version tuple was closed without merging; leaving it closed.',
    );
    return { status: 'rejected-tuple', pullRequest: null };
  }
  const openPullRequests = await listOpenBotPullRequests(readClient);
  if (openPullRequests.length > 1)
    throw new Error('More than one open Pi update PR uses the bot branch.');
  const existingPullRequest = openPullRequests[0];
  const existingState = existingPullRequest
    ? await verifyOpenPullRequest(
        readClient,
        existingPullRequest,
        validated.currentBaseFiles,
      )
    : null;
  if (
    sameCandidateVersions(
      existingState?.candidateVersions,
      validated.metadata.candidateVersions,
    ) &&
    existingState.fingerprint === validated.fingerprint
  ) {
    console.log(
      `Pi update PR ${existingPullRequest.html_url} already contains this verified candidate.`,
    );
    return {
      status: 'already-current',
      pullRequest: existingPullRequest,
      headSha: existingPullRequest.head.sha,
    };
  }

  const { data: baseBranch } = await readClient.request(
    `/repos/${readClient.owner}/${readClient.name}/git/ref/heads/${defaultBranch}`,
  );
  if (baseBranch.object.sha !== baseSha)
    throw new Error(
      'Default branch moved during candidate resolution; run the weekly updater again against its latest base.',
    );
  if (!existingPullRequest) {
    const branch = await readClient
      .request(
        `/repos/${readClient.owner}/${readClient.name}/git/ref/heads/${BOT_BRANCH}`,
      )
      .catch(function branchNotFound(error) {
        if (!String(error.message).includes('(404)')) throw error;
        return null;
      });
    if (branch)
      await verifyClosedBotBranch(
        readClient,
        repository,
        validated.currentBaseFiles,
      );
  }
  const commitSha = await writeBranchCommit(writeClient, baseSha, validated);
  const body = setGeneratedCommit(
    candidateBody(validated, commitSha),
    commitSha,
  );
  const title = candidateTitle(
    validated.metadata.candidateVersions,
    validated.metadata.baseVersions,
  );
  let pullRequest;
  if (existingPullRequest) {
    const updated = await writeClient.request(
      `/repos/${writeClient.owner}/${writeClient.name}/pulls/${existingPullRequest.number}`,
      { method: 'PATCH', body: { title, body, base: defaultBranch } },
    );
    pullRequest = updated.data;
  } else {
    const created = await writeClient.request(
      `/repos/${writeClient.owner}/${writeClient.name}/pulls`,
      {
        method: 'POST',
        body: {
          title,
          body,
          head: BOT_BRANCH,
          base: defaultBranch,
          maintainer_can_modify: true,
        },
      },
    );
    pullRequest = created.data;
  }
  console.log(
    `Pi update PR ${pullRequest.html_url} now points at ${commitSha}; required CI is pending on this head.`,
  );
  return { status: 'published', pullRequest, headSha: commitSha };
}

async function main(args) {
  if (args[0] === '--validate') {
    const validated = await validateCandidateArtifact(args[1]);
    console.log(
      `candidate=${validated.metadata.candidateVersions[PI_PACKAGES[0]]}`,
    );
    return;
  }
  const result = await publishCandidate(args[0]);
  if (process.env.GITHUB_OUTPUT && result.pullRequest)
    await fs.appendFile(
      process.env.GITHUB_OUTPUT,
      `pull_request_url=${result.pullRequest.html_url}\n`,
    );
}

if (require.main === module) {
  main(process.argv.slice(2)).catch(function reportPublishFailure(error) {
    console.error(error instanceof Error ? error.message : String(error));
    process.exitCode = 1;
  });
}

module.exports = {
  BOT_BRANCH,
  candidateBody,
  candidateTitle,
  candidateVersionsFromBody,
  exactCandidateMarker,
  mergeGeneratedFileWithCurrentBase,
  parseStateMarker,
  parseCandidateMarker,
  publishCandidate,
  safeDeltaText,
  setGeneratedCommit,
  validateArtifact,
  validateCandidateArtifact,
  verifyClosedBotBranch,
  verifyOpenPullRequest,
};
