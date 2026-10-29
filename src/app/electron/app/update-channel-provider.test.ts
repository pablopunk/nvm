import assert from 'node:assert/strict';
import test from 'node:test';
import { type AppUpdater, CancellationToken } from 'electron-updater';
import type {
  ProviderPlatform,
  ProviderRuntimeOptions,
} from 'electron-updater/out/providers/Provider.js';
import type { UpdateChannel } from './update-channel';
import { NevermindChannelProvider } from './update-channel-provider';

const OWNER = 'pablopunk';
const REPO = 'nvm';
const RELEASES_PATH = `/${OWNER}/${REPO}/releases`;
const LATEST_PATH = `${RELEASES_PATH}/latest`;
const FEED_PATH = `/${OWNER}/${REPO}/releases.atom`;

const STABLE_TAG = 'v0.17.0';
const NIGHTLY_NEWEST_TAG = 'v0.17.1-nightly.20261028.13';
const NIGHTLY_SECOND_TAG = 'v0.17.1-nightly.20261027.12';
const NIGHTLY_THIRD_TAG = 'v0.17.1-nightly.20261026.11';
const NIGHTLY_FOURTH_TAG = 'v0.17.1-nightly.20261025.10';

type FakeRouteResponse =
  | string
  | null
  | Error
  | ((token: CancellationToken) => string);

type FakeRequestOptions = { protocol: string; hostname: string; path: string };

type FakeRequest = { url: string; token: CancellationToken };

type FakeGitHub = ReturnType<typeof createFakeGitHub>;

function httpError(statusCode: number) {
  return Object.assign(new Error(`HTTP error: ${statusCode}`), { statusCode });
}

function cancelledError() {
  return Object.assign(new Error('request cancelled'), {
    name: 'CancellationError',
  });
}

// Mirrors the real executor, which rejects any request whose token is already cancelled.
function createFakeGitHub(routes: Record<string, FakeRouteResponse>) {
  const requests: FakeRequest[] = [];

  async function request(
    options: FakeRequestOptions,
    token: CancellationToken,
  ) {
    const url = new URL(
      `${options.protocol}//${options.hostname}${options.path}`,
    );
    requests.push({ url: url.href, token });
    if (token.cancelled) throw cancelledError();
    const route: FakeRouteResponse | undefined = routes[url.pathname];
    if (route === undefined) throw httpError(404);
    if (route instanceof Error) throw route;
    if (typeof route === 'function') return route(token);
    return route;
  }

  return { executor: { request }, requests };
}

function requestedPaths(github: FakeGitHub) {
  return github.requests.map(({ url }) => new URL(url).pathname);
}

function requestedUrls(github: FakeGitHub) {
  return github.requests.map(({ url }) => url);
}

function metadataPath(tag: string, fileName: string) {
  return `${RELEASES_PATH}/download/${tag}/${fileName}`;
}

function metadataYaml(version: string) {
  return [
    `version: ${version}`,
    'files:',
    `  - url: NVM-${version}.exe`,
    '    sha512: ZmFrZQ==',
    '    size: 1',
    `path: NVM-${version}.exe`,
    'sha512: ZmFrZQ==',
    "releaseDate: '2026-10-28T00:00:00.000Z'",
  ].join('\n');
}

function latestRelease(tag: string | null): Record<string, FakeRouteResponse> {
  return {
    [LATEST_PATH]: tag === null ? null : JSON.stringify({ tag_name: tag }),
  };
}

function feedBody(tags: string[]) {
  const entries = tags.map(
    (tag) =>
      `<entry><link href="https://github.com${RELEASES_PATH}/tag/${tag}"/></entry>`,
  );
  return `<feed>${entries.join('')}</feed>`;
}

function releaseFeed(tags: string[]): Record<string, FakeRouteResponse> {
  return { [FEED_PATH]: feedBody(tags) };
}

function publishedMetadata(
  tag: string,
  fileName: string,
): Record<string, FakeRouteResponse> {
  return { [metadataPath(tag, fileName)]: metadataYaml(tag.slice(1)) };
}

function createProvider(
  channel: UpdateChannel,
  github: FakeGitHub,
  platform: ProviderPlatform = 'win32',
  devRepository = '',
) {
  return new NevermindChannelProvider(
    {
      provider: 'github',
      owner: OWNER,
      repo: REPO,
      resolveChannel: () => channel,
      devRepository,
    },
    {} as AppUpdater,
    {
      executor: github.executor,
      platform,
      isUseMultipleRangeRequest: false,
    } as unknown as ProviderRuntimeOptions,
  );
}

test('dev updates use the isolated prerelease repository for metadata and payloads', async () => {
  const tag = 'v2026.1029.901';
  const path = '/pablopunk/nvm-dev/releases';
  const github = createFakeGitHub({
    '/repos/pablopunk/nvm-dev/releases': JSON.stringify([
      { tag_name: tag, draft: false, prerelease: true },
    ]),
    [`${path}/download/${tag}/nightly.yml`]: metadataYaml(tag.slice(1)),
  });
  const provider = createProvider('dev', github, 'win32', 'pablopunk/nvm-dev');
  const info = await provider.getLatestVersion();
  assert.equal(info.version, tag.slice(1));
  assert.equal(info.updateChannel, 'dev');
  assert.match(
    provider.resolveFiles(info)[0].url.href,
    /\/nvm-dev\/releases\/download\//,
  );
});

async function withUpdaterArch<T>(arch: string, run: () => Promise<T>) {
  const previous = process.env.TEST_UPDATER_ARCH;
  process.env.TEST_UPDATER_ARCH = arch;
  try {
    return await run();
  } finally {
    process.env.TEST_UPDATER_ARCH = previous ?? '';
  }
}

test('stable channel reads only the latest stable release and never the releases feed', async () => {
  const github = createFakeGitHub({
    ...latestRelease(STABLE_TAG),
    ...releaseFeed([NIGHTLY_NEWEST_TAG, STABLE_TAG]),
    ...publishedMetadata(STABLE_TAG, 'latest.yml'),
    ...publishedMetadata(NIGHTLY_NEWEST_TAG, 'nightly.yml'),
  });

  const info = await createProvider('stable', github).getLatestVersion();

  assert.equal(info.tag, STABLE_TAG);
  assert.equal(info.updateChannel, 'stable');
  assert.equal(info.version, '0.17.0');
  assert.deepEqual(requestedUrls(github), [
    'https://github.com/pablopunk/nvm/releases/latest',
    'https://github.com/pablopunk/nvm/releases/download/v0.17.0/latest.yml',
  ]);
});

test('nightly channel picks a newer nightly over the latest stable regardless of feed order', async () => {
  const github = createFakeGitHub({
    ...latestRelease(STABLE_TAG),
    ...releaseFeed([
      NIGHTLY_SECOND_TAG,
      STABLE_TAG,
      NIGHTLY_NEWEST_TAG,
      'v0.16.9',
      'v0.17.1-beta.2',
    ]),
    ...publishedMetadata(NIGHTLY_NEWEST_TAG, 'nightly.yml'),
  });

  const info = await createProvider('nightly', github).getLatestVersion();

  assert.equal(info.tag, NIGHTLY_NEWEST_TAG);
  assert.equal(info.updateChannel, 'nightly');
  assert.equal(info.version, '0.17.1-nightly.20261028.13');
  assert.deepEqual(requestedPaths(github), [
    LATEST_PATH,
    FEED_PATH,
    metadataPath(NIGHTLY_NEWEST_TAG, 'nightly.yml'),
  ]);
});

test('a stable release outranks nightlies of its own base and of older bases', async () => {
  const github = createFakeGitHub({
    ...latestRelease('v0.17.1'),
    ...releaseFeed([
      'v0.17.1-nightly.20261028.13',
      'v0.17.0-nightly.20261028.20',
    ]),
    ...publishedMetadata('v0.17.1', 'latest.yml'),
    ...publishedMetadata('v0.17.1-nightly.20261028.13', 'nightly.yml'),
  });

  const info = await createProvider('nightly', github).getLatestVersion();

  assert.equal(info.tag, 'v0.17.1');
  assert.equal(info.updateChannel, 'nightly');
  assert.equal(info.version, '0.17.1');
  assert.deepEqual(requestedPaths(github), [
    LATEST_PATH,
    FEED_PATH,
    metadataPath('v0.17.1', 'latest.yml'),
  ]);
});

test('nightly lookup falls back to the next newest candidate when the newest lacks nightly metadata', async () => {
  const github = createFakeGitHub({
    ...latestRelease(STABLE_TAG),
    ...releaseFeed([NIGHTLY_NEWEST_TAG, NIGHTLY_SECOND_TAG]),
    ...publishedMetadata(NIGHTLY_SECOND_TAG, 'nightly.yml'),
  });

  const info = await createProvider('nightly', github).getLatestVersion();

  assert.equal(info.tag, NIGHTLY_SECOND_TAG);
  assert.equal(info.updateChannel, 'nightly');
  assert.equal(info.version, '0.17.1-nightly.20261027.12');
  assert.deepEqual(requestedPaths(github), [
    LATEST_PATH,
    FEED_PATH,
    metadataPath(NIGHTLY_NEWEST_TAG, 'nightly.yml'),
    metadataPath(NIGHTLY_SECOND_TAG, 'nightly.yml'),
  ]);
});

test('nightly lookup inspects at most three candidates before reporting no published metadata', async () => {
  const github = createFakeGitHub({
    ...latestRelease(STABLE_TAG),
    ...releaseFeed([
      NIGHTLY_FOURTH_TAG,
      NIGHTLY_SECOND_TAG,
      NIGHTLY_NEWEST_TAG,
      NIGHTLY_THIRD_TAG,
    ]),
    ...publishedMetadata(STABLE_TAG, 'latest.yml'),
  });

  await assert.rejects(createProvider('nightly', github).getLatestVersion(), {
    code: 'ERR_UPDATER_NO_PUBLISHED_VERSIONS',
  });
  assert.deepEqual(requestedPaths(github), [
    LATEST_PATH,
    FEED_PATH,
    metadataPath(NIGHTLY_NEWEST_TAG, 'nightly.yml'),
    metadataPath(NIGHTLY_SECOND_TAG, 'nightly.yml'),
    metadataPath(NIGHTLY_THIRD_TAG, 'nightly.yml'),
  ]);
});

test('stable never rolls back to an older release when the latest stable lacks metadata', async () => {
  const github = createFakeGitHub({
    ...latestRelease('v0.17.1'),
    ...releaseFeed(['v0.17.1', STABLE_TAG]),
    ...publishedMetadata(STABLE_TAG, 'latest.yml'),
  });

  await assert.rejects(createProvider('stable', github).getLatestVersion(), {
    code: 'ERR_UPDATER_NO_PUBLISHED_VERSIONS',
  });
  assert.deepEqual(requestedPaths(github), [
    LATEST_PATH,
    metadataPath('v0.17.1', 'latest.yml'),
  ]);
});

test('releases without a tag publish nothing on either channel', async () => {
  for (const channel of ['stable', 'nightly'] as const) {
    for (const body of [null, '{}']) {
      const github = createFakeGitHub({
        [LATEST_PATH]: body,
        ...releaseFeed([]),
      });

      await assert.rejects(createProvider(channel, github).getLatestVersion(), {
        code: 'ERR_UPDATER_NO_PUBLISHED_VERSIONS',
      });
      assert.equal(
        requestedPaths(github).some((path) => path.includes('/download/')),
        false,
      );
    }
  }
});

test('malformed latest-release JSON surfaces before any feed or metadata request', async () => {
  const github = createFakeGitHub({ [LATEST_PATH]: '{not json' });

  await assert.rejects(createProvider('nightly', github).getLatestVersion(), {
    name: 'SyntaxError',
  });
  assert.deepEqual(requestedPaths(github), [LATEST_PATH]);
});

test('a server error on newest candidate metadata is not treated as missing metadata', async () => {
  const github = createFakeGitHub({
    ...latestRelease(STABLE_TAG),
    ...releaseFeed([NIGHTLY_NEWEST_TAG, NIGHTLY_SECOND_TAG]),
    [metadataPath(NIGHTLY_NEWEST_TAG, 'nightly.yml')]: httpError(500),
    ...publishedMetadata(NIGHTLY_SECOND_TAG, 'nightly.yml'),
  });

  await assert.rejects(createProvider('nightly', github).getLatestVersion(), {
    statusCode: 500,
  });
  assert.deepEqual(requestedPaths(github), [
    LATEST_PATH,
    FEED_PATH,
    metadataPath(NIGHTLY_NEWEST_TAG, 'nightly.yml'),
  ]);
});

test('malformed or empty update metadata fails the lookup instead of skipping to an older release', async () => {
  for (const body of ['version: [unclosed', null]) {
    const github = createFakeGitHub({
      ...latestRelease(STABLE_TAG),
      ...releaseFeed([NIGHTLY_NEWEST_TAG, NIGHTLY_SECOND_TAG]),
      [metadataPath(NIGHTLY_NEWEST_TAG, 'nightly.yml')]: body,
      ...publishedMetadata(NIGHTLY_SECOND_TAG, 'nightly.yml'),
    });

    await assert.rejects(createProvider('nightly', github).getLatestVersion(), {
      code: 'ERR_UPDATER_INVALID_UPDATE_INFO',
    });
    assert.deepEqual(requestedPaths(github), [
      LATEST_PATH,
      FEED_PATH,
      metadataPath(NIGHTLY_NEWEST_TAG, 'nightly.yml'),
    ]);
  }
});

test('rejects update metadata whose version does not match the release tag', async () => {
  const github = createFakeGitHub({
    ...latestRelease(STABLE_TAG),
    ...releaseFeed([NIGHTLY_NEWEST_TAG]),
    [metadataPath(NIGHTLY_NEWEST_TAG, 'nightly.yml')]: metadataYaml('0.16.0'),
  });
  await assert.rejects(createProvider('nightly', github).getLatestVersion(), {
    code: 'ERR_UPDATER_INVALID_UPDATE_INFO',
  });
});

const PLATFORM_METADATA_CASES: Array<{
  platform: ProviderPlatform;
  arch?: string;
  stable: string;
  nightly: string;
}> = [
  { platform: 'win32', stable: 'latest.yml', nightly: 'nightly.yml' },
  { platform: 'darwin', stable: 'latest-mac.yml', nightly: 'nightly-mac.yml' },
  {
    platform: 'linux',
    arch: 'x64',
    stable: 'latest-linux.yml',
    nightly: 'nightly-linux.yml',
  },
  {
    platform: 'linux',
    arch: 'arm64',
    stable: 'latest-linux-arm64.yml',
    nightly: 'nightly-linux-arm64.yml',
  },
];

for (const { platform, arch, stable, nightly } of PLATFORM_METADATA_CASES) {
  test(`${platform}${arch ? ` ${arch}` : ''} reads platform-specific channel metadata files`, async () => {
    await withUpdaterArch(arch ?? '', async () => {
      const stableGitHub = createFakeGitHub({
        ...latestRelease(STABLE_TAG),
        ...publishedMetadata(STABLE_TAG, stable),
      });
      const stableInfo = await createProvider(
        'stable',
        stableGitHub,
        platform,
      ).getLatestVersion();
      assert.equal(stableInfo.tag, STABLE_TAG);
      assert.deepEqual(requestedPaths(stableGitHub), [
        LATEST_PATH,
        metadataPath(STABLE_TAG, stable),
      ]);

      const nightlyGitHub = createFakeGitHub({
        ...latestRelease(STABLE_TAG),
        ...releaseFeed([NIGHTLY_NEWEST_TAG]),
        ...publishedMetadata(NIGHTLY_NEWEST_TAG, nightly),
      });
      const nightlyInfo = await createProvider(
        'nightly',
        nightlyGitHub,
        platform,
      ).getLatestVersion();
      assert.equal(nightlyInfo.tag, NIGHTLY_NEWEST_TAG);
      assert.deepEqual(requestedPaths(nightlyGitHub), [
        LATEST_PATH,
        FEED_PATH,
        metadataPath(NIGHTLY_NEWEST_TAG, nightly),
      ]);
    });
  });
}

test('cancelling the transport while reading the releases feed stops before any metadata request', async () => {
  const cancelled = cancelledError();
  const github = createFakeGitHub({
    ...latestRelease(STABLE_TAG),
    [FEED_PATH]: cancelled,
    ...publishedMetadata(NIGHTLY_NEWEST_TAG, 'nightly.yml'),
  });

  await assert.rejects(
    createProvider('nightly', github).getLatestVersion(),
    (error) => error === cancelled,
  );
  assert.deepEqual(requestedPaths(github), [LATEST_PATH, FEED_PATH]);
});

test('a cancelled metadata request propagates unchanged instead of trying older candidates', async () => {
  const cancelled = cancelledError();
  const github = createFakeGitHub({
    ...latestRelease(STABLE_TAG),
    ...releaseFeed([NIGHTLY_NEWEST_TAG, NIGHTLY_SECOND_TAG]),
    [metadataPath(NIGHTLY_NEWEST_TAG, 'nightly.yml')]: cancelled,
    ...publishedMetadata(NIGHTLY_SECOND_TAG, 'nightly.yml'),
  });

  await assert.rejects(
    createProvider('nightly', github).getLatestVersion(),
    (error) => error === cancelled,
  );
  assert.deepEqual(requestedPaths(github), [
    LATEST_PATH,
    FEED_PATH,
    metadataPath(NIGHTLY_NEWEST_TAG, 'nightly.yml'),
  ]);
});

test('a token cancelled mid-lookup stops the walk at the next request', async () => {
  const github = createFakeGitHub({
    ...latestRelease(STABLE_TAG),
    [FEED_PATH]: (token) => {
      token.cancel();
      return feedBody([NIGHTLY_NEWEST_TAG, NIGHTLY_SECOND_TAG]);
    },
    ...publishedMetadata(NIGHTLY_NEWEST_TAG, 'nightly.yml'),
    ...publishedMetadata(NIGHTLY_SECOND_TAG, 'nightly.yml'),
  });

  await assert.rejects(createProvider('nightly', github).getLatestVersion(), {
    name: 'CancellationError',
  });
  assert.deepEqual(requestedPaths(github), [
    LATEST_PATH,
    FEED_PATH,
    metadataPath(NIGHTLY_NEWEST_TAG, 'nightly.yml'),
  ]);
});

test('every request in one lookup shares a single live cancellation token', async () => {
  const github = createFakeGitHub({
    ...latestRelease(STABLE_TAG),
    ...releaseFeed([NIGHTLY_NEWEST_TAG]),
    ...publishedMetadata(NIGHTLY_NEWEST_TAG, 'nightly.yml'),
  });

  await createProvider('nightly', github).getLatestVersion();

  const tokens = new Set(github.requests.map(({ token }) => token));
  assert.equal(tokens.size, 1);
  assert.ok(
    github.requests.every(
      ({ token }) => token instanceof CancellationToken && !token.cancelled,
    ),
  );
});
