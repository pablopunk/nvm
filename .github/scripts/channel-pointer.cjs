'use strict';

const { execFileSync } = require('node:child_process');
const { compareVersions } = require('./nightly-versioning.cjs');

function channelVersion(channel, tag) {
  if (!['dev', 'nightly'].includes(channel)) throw new Error('Invalid channel');
  const match = new RegExp(`^v(\\d+\\.\\d+\\.\\d+)-${channel}$`).exec(
    tag ?? '',
  );
  if (!match) throw new Error(`Invalid ${channel} tag`);
  return match[1];
}

function choosePointer(channel, tag, existing) {
  if (!['dev', 'nightly'].includes(channel)) throw new Error('Invalid channel');
  const nextVersion = channelVersion(channel, tag);
  if (existing !== null) {
    if (
      existing?.kind !== 'nevermind-channel-pointer' ||
      existing.channel !== channel ||
      !Array.isArray(existing.tags) ||
      existing.tags.length < 1 ||
      existing.tags.length > 2
    )
      throw new Error('Invalid existing channel pointer');
    const previousVersion = channelVersion(channel, existing.tags[0]);
    for (const previousTag of existing.tags.slice(1)) {
      if (
        compareVersions(
          channelVersion(channel, previousTag),
          previousVersion,
        ) >= 0
      )
        throw new Error('Invalid channel pointer order');
    }
    if (compareVersions(previousVersion, nextVersion) >= 0) return existing;
    return {
      kind: 'nevermind-channel-pointer',
      channel,
      tags: [tag, existing.tags[0]],
    };
  }
  return { kind: 'nevermind-channel-pointer', channel, tags: [tag] };
}

function gh(args, allowMissing = false) {
  try {
    return execFileSync('gh', args, {
      encoding: 'utf8',
      stdio: ['ignore', 'pipe', 'pipe'],
    });
  } catch (error) {
    if (allowMissing && String(error.stderr ?? '').includes('(HTTP 404)'))
      return null;
    throw error;
  }
}

function publishPointer({ repository, channel, tag, sourceSha, run = gh }) {
  if (
    !/^[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+$/.test(repository) ||
    !/^[0-9a-f]{40}$/.test(sourceSha)
  )
    throw new Error('Invalid repository or source commit');
  channelVersion(channel, tag);
  const release = JSON.parse(
    run(['api', `repos/${repository}/releases/tags/${tag}`]),
  );
  if (
    release.draft !== false ||
    release.prerelease !== true ||
    release.tag_name !== tag
  )
    throw new Error('Pointer target is not a published prerelease');
  const resolved = JSON.parse(
    run(['api', `repos/${repository}/commits/${tag}`]),
  );
  if (resolved.sha !== sourceSha)
    throw new Error('Pointer target commit mismatch');
  const branch = 'release-pointers';
  const ref = run(['api', `repos/${repository}/git/ref/heads/${branch}`], true);
  if (ref === null)
    run([
      'api',
      '-X',
      'POST',
      `repos/${repository}/git/refs`,
      '-f',
      `ref=refs/heads/${branch}`,
      '-f',
      `sha=${sourceSha}`,
    ]);
  const endpoint = `repos/${repository}/contents/.channels/${channel}.json`;
  const previousRaw = run(['api', `${endpoint}?ref=${branch}`], true);
  const previous = previousRaw ? JSON.parse(previousRaw) : null;
  const oldPointer = previous
    ? JSON.parse(
        Buffer.from(previous.content.replace(/\s/g, ''), 'base64').toString(
          'utf8',
        ),
      )
    : null;
  const pointer = choosePointer(channel, tag, oldPointer);
  if (oldPointer && JSON.stringify(pointer) === JSON.stringify(oldPointer))
    return pointer;
  const content = Buffer.from(`${JSON.stringify(pointer)}\n`).toString(
    'base64',
  );
  const args = [
    'api',
    '-X',
    'PUT',
    endpoint,
    '-f',
    `message=Update ${channel} release pointer`,
    '-f',
    `branch=${branch}`,
    '-f',
    `content=${content}`,
  ];
  if (previous) args.push('-f', `sha=${previous.sha}`);
  run(args);
  return pointer;
}

if (require.main === module) {
  try {
    const [channel, tag, sourceSha] = process.argv.slice(2);
    console.log(
      JSON.stringify(
        publishPointer({
          repository: process.env.GITHUB_REPOSITORY,
          channel,
          tag,
          sourceSha,
        }),
      ),
    );
  } catch (error) {
    console.error(error instanceof Error ? error.message : String(error));
    process.exitCode = 1;
  }
}
module.exports = { choosePointer, publishPointer };
