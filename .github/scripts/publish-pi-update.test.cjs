const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const os = require('node:os');
const path = require('node:path');
const test = require('node:test');
const {
  candidateBody,
  candidateVersionsFromBody,
  exactCandidateMarker,
  mergeGeneratedFileWithCurrentBase,
  parseCandidateMarker,
  parseStateMarker,
  setGeneratedCommit,
  publishCandidate,
} = require('./publish-pi-update.cjs');
const {
  packageFingerprint,
  packageVersionDelta,
} = require('./pi-update-utils.cjs');

const versions = {
  '@earendil-works/pi-ai': '1.4.0',
  '@earendil-works/pi-coding-agent': '1.4.0',
};

test('clean base changes merge with the bot candidate and conflicting edits require attention', async function semanticMergeCheck() {
  const originalBase = [
    'dependencies:',
    '  pi: 0.80.10',
    '  other: 1.0.0',
    '',
  ].join('\n');
  const generated = originalBase.replace('pi: 0.80.10', 'pi: 1.4.0');
  const currentBase = originalBase.replace('other: 1.0.0', 'other: 2.0.0');
  assert.equal(
    await mergeGeneratedFileWithCurrentBase(
      generated,
      originalBase,
      currentBase,
    ),
    generated.replace('other: 1.0.0', 'other: 2.0.0'),
  );

  const humanEdit = currentBase.replace('pi: 0.80.10', 'pi: 9.9.9');
  assert.equal(
    await mergeGeneratedFileWithCurrentBase(generated, originalBase, humanEdit),
    null,
  );
});

test('PR body records an exact rejected tuple and generated commit fingerprint', function machineReadableMarkers() {
  const fingerprint = 'a'.repeat(64);
  const generatedCommitSha = 'b'.repeat(40);
  const candidate = {
    metadata: {
      candidateVersions: versions,
      baseVersions: {
        '@earendil-works/pi-ai': '0.80.10',
        '@earendil-works/pi-coding-agent': '0.80.10',
      },
      baseSha: 'c'.repeat(40),
      transitiveDelta: [],
    },
    fingerprint,
    transitiveDelta: [],
  };
  const body = setGeneratedCommit(
    candidateBody(candidate, generatedCommitSha),
    generatedCommitSha,
  );
  assert.equal(body.includes(exactCandidateMarker(versions)), true);
  assert.deepEqual(
    parseCandidateMarker(exactCandidateMarker(versions)),
    versions,
  );
  assert.throws(() => parseCandidateMarker('<!-- edited marker -->'));
  const state = parseStateMarker(body);
  assert.deepEqual(state, {
    baseSha: 'c'.repeat(40),
    fingerprint,
    candidateVersions: versions,
    generatedCommitSha,
  });
  assert.match(
    body,
    /Review classification: major; targeted API, tool, provider, auth, and packaging review required/,
  );
  assert.match(body, /Generated commit: b{40}/);
  assert.match(body, /Required CI: pending/);
});

test('same-major Pi candidates retain the routine update review classification', function routineUpdateClassification() {
  const routineVersions = {
    '@earendil-works/pi-ai': '1.4.1',
    '@earendil-works/pi-coding-agent': '1.4.1',
  };
  const body = candidateBody(
    {
      metadata: {
        candidateVersions: routineVersions,
        baseVersions: {
          '@earendil-works/pi-ai': '1.4.0',
          '@earendil-works/pi-coding-agent': '1.4.0',
        },
        baseSha: 'c'.repeat(40),
        transitiveDelta: [],
      },
      fingerprint: 'a'.repeat(64),
      transitiveDelta: [],
    },
    'b'.repeat(40),
  );
  assert.match(
    body,
    /Review classification: routine update; skim the upstream release notes/,
  );
});

test('state marker parser reads nested JSON safely and accepts previously written candidate markers', function legacyStateMarker() {
  const legacyState = {
    baseSha: 'c'.repeat(40),
    fingerprint: 'a'.repeat(64),
    candidateMarker: exactCandidateMarker(versions),
    generatedCommitSha: 'b'.repeat(40),
  };
  const legacyBody = [
    exactCandidateMarker(versions),
    `<!-- pi-update-state:${JSON.stringify(legacyState)} -->`,
  ].join('\n');
  assert.deepEqual(parseStateMarker(legacyBody), legacyState);
  assert.deepEqual(candidateVersionsFromBody(legacyBody), versions);
});

function updaterFixtureFiles(version) {
  const manifest = {
    name: 'fixture',
    dependencies: {
      '@earendil-works/pi-ai': version,
      '@earendil-works/pi-coding-agent': version,
      other: '1.0.0',
    },
  };
  const lockfile = [
    "lockfileVersion: '9.0'",
    '',
    'importers:',
    '  .:',
    '    dependencies:',
    "      '@earendil-works/pi-ai':",
    `        specifier: ${version}`,
    `        version: ${version}`,
    "      '@earendil-works/pi-coding-agent':",
    `        specifier: ${version}`,
    `        version: ${version}`,
    '      other:',
    '        specifier: 1.0.0',
    '        version: 1.0.0',
    '',
    'packages:',
    `  '@earendil-works/pi-ai@${version}':`,
    '    resolution: {integrity: sha512-fixture}',
    `  '@earendil-works/pi-coding-agent@${version}':`,
    '    resolution: {integrity: sha512-fixture}',
    '',
    'snapshots:',
    `  '@earendil-works/pi-ai@${version}': {}`,
    `  '@earendil-works/pi-coding-agent@${version}': {}`,
    '',
  ].join('\n');
  return {
    manifestText: `${JSON.stringify(manifest, null, 2)}\n`,
    lockText: lockfile,
  };
}

test('publisher creates the bot PR and reads the candidate tuple back on its next run', async function githubRequestLifecycle() {
  const directory = await fs.mkdtemp(
    path.join(os.tmpdir(), 'pi-publisher-lifecycle-'),
  );
  const trustedRoot = path.join(directory, 'trusted');
  const artifactDirectory = path.join(directory, 'artifact');
  const baseSha = 'a'.repeat(40);
  const generatedSha = 'b'.repeat(40);
  const treeSha = 'c'.repeat(40);
  const repository = 'owner/repo';
  const base = updaterFixtureFiles('0.80.10');
  const candidate = updaterFixtureFiles('1.4.0');
  const artifactFiles = new Map();
  const blobs = new Map();
  const commits = new Map();
  let branchSha = null;
  let openPullRequest = null;
  let blobCount = 0;
  const login = 'pi-update[bot]';
  const environment = {
    GITHUB_REPOSITORY: repository,
    GITHUB_TOKEN: 'read-token',
    GH_APP_TOKEN: 'write-token',
    GITHUB_SHA: baseSha,
    GITHUB_RUN_ID: '77',
    GITHUB_RUN_ATTEMPT: '1',
    PI_UPDATE_BASE_SHA: baseSha,
    GITHUB_DEFAULT_BRANCH: 'main',
  };

  async function githubFetch(input, options = {}) {
    const url = new URL(input);
    const requestPath = url.pathname;
    const method = options.method || 'GET';
    const body = options.body ? JSON.parse(options.body) : {};
    function response(data, status = 200) {
      return new Response(JSON.stringify(data), {
        status,
        headers: { 'content-type': 'application/json' },
      });
    }
    if (requestPath === `/repos/${repository}/pulls` && method === 'GET') {
      if (url.searchParams.get('state') === 'closed') return response([]);
      return response(openPullRequest ? [openPullRequest] : []);
    }
    if (
      requestPath === `/repos/${repository}/git/ref/heads/main` &&
      method === 'GET'
    )
      return response({ object: { sha: baseSha } });
    if (
      requestPath ===
        `/repos/${repository}/git/ref/heads/automation/pi-update` &&
      method === 'GET'
    )
      return branchSha
        ? response({ object: { sha: branchSha } })
        : response({ message: 'Not Found' }, 404);
    if (
      requestPath === `/repos/${repository}/git/commits/${baseSha}` &&
      method === 'GET'
    )
      return response({ tree: { sha: 'base-tree' } });
    if (requestPath === `/repos/${repository}/git/blobs` && method === 'POST') {
      const sha = (++blobCount).toString(16).padStart(40, '0');
      blobs.set(sha, body.content);
      return response({ sha });
    }
    if (requestPath === `/repos/${repository}/git/trees` && method === 'POST') {
      for (const entry of body.tree)
        artifactFiles.set(entry.path, blobs.get(entry.sha));
      return response({ sha: treeSha });
    }
    if (
      requestPath === `/repos/${repository}/git/commits` &&
      method === 'POST'
    ) {
      const commit = {
        sha: generatedSha,
        tree: { sha: body.tree },
        parents: body.parents.map((sha) => ({ sha })),
        author: { login },
        commit: { message: body.message },
      };
      commits.set(generatedSha, commit);
      return response({ sha: generatedSha });
    }
    if (requestPath === `/repos/${repository}/git/refs` && method === 'POST') {
      branchSha = body.sha;
      return response({ object: { sha: branchSha } });
    }
    if (requestPath === `/repos/${repository}/pulls` && method === 'POST') {
      openPullRequest = {
        number: 12,
        html_url: 'https://github.com/owner/repo/pull/12',
        body: body.body,
        user: { login },
        head: {
          ref: 'automation/pi-update',
          sha: branchSha,
          repo: { full_name: repository },
        },
      };
      return response(openPullRequest);
    }
    if (
      requestPath === `/repos/${repository}/commits/${generatedSha}` &&
      method === 'GET'
    )
      return response(commits.get(generatedSha));
    if (
      requestPath === `/repos/${repository}/pulls/12/files` &&
      method === 'GET'
    )
      return response([
        { filename: 'package.json' },
        { filename: 'pnpm-lock.yaml' },
      ]);
    const contentMatch = new RegExp(
      `^/repos/${repository}/contents/(package\\.json|pnpm-lock\\.yaml)$`,
    ).exec(requestPath);
    if (contentMatch && method === 'GET') {
      const filename = contentMatch[1];
      const ref = url.searchParams.get('ref');
      const content =
        ref === baseSha
          ? filename === 'package.json'
            ? base.manifestText
            : base.lockText
          : filename === 'package.json'
            ? artifactFiles.get('package.json')
            : artifactFiles.get('pnpm-lock.yaml');
      return response({
        type: 'file',
        content: Buffer.from(content).toString('base64'),
      });
    }
    throw new Error(`Unexpected fake GitHub request: ${method} ${url}`);
  }

  try {
    await fs.mkdir(trustedRoot, { recursive: true });
    await fs.mkdir(artifactDirectory, { recursive: true });
    await fs.writeFile(
      path.join(trustedRoot, 'package.json'),
      base.manifestText,
    );
    await fs.writeFile(path.join(trustedRoot, 'pnpm-lock.yaml'), base.lockText);
    const metadata = {
      schemaVersion: 1,
      repository,
      runId: environment.GITHUB_RUN_ID,
      runAttempt: environment.GITHUB_RUN_ATTEMPT,
      baseSha,
      workflowSha: baseSha,
      baseVersions: {
        '@earendil-works/pi-ai': '0.80.10',
        '@earendil-works/pi-coding-agent': '0.80.10',
      },
      candidateVersions: versions,
      transitiveDelta: packageVersionDelta(base.lockText, candidate.lockText),
      fingerprint: packageFingerprint(
        candidate.manifestText,
        candidate.lockText,
      ),
    };
    await fs.writeFile(
      path.join(artifactDirectory, 'package.json'),
      candidate.manifestText,
    );
    await fs.writeFile(
      path.join(artifactDirectory, 'pnpm-lock.yaml'),
      candidate.lockText,
    );
    await fs.writeFile(
      path.join(artifactDirectory, 'metadata.json'),
      JSON.stringify(metadata),
    );

    const dependencies = {
      apiBaseUrl: 'https://api.example.test',
      fetchImplementation: githubFetch,
      trustedRoot,
    };
    const published = await publishCandidate(
      artifactDirectory,
      environment,
      dependencies,
    );
    assert.equal(published.status, 'published');
    assert.equal(published.headSha, generatedSha);
    assert.equal(
      openPullRequest.body.includes(exactCandidateMarker(versions)),
      true,
    );
    assert.deepEqual(
      parseStateMarker(openPullRequest.body).candidateVersions,
      versions,
    );
    assert.equal(
      parseStateMarker(openPullRequest.body).candidateMarker,
      undefined,
    );

    const alreadyCurrent = await publishCandidate(
      artifactDirectory,
      environment,
      dependencies,
    );
    assert.equal(alreadyCurrent.status, 'already-current');
    assert.equal(alreadyCurrent.pullRequest.number, 12);
  } finally {
    await fs.rm(directory, { recursive: true, force: true });
  }
});
