import {
  compareSemverVersions,
  parseSemverVersion,
} from '../platform/version-utils';

export const UPDATE_CHANNEL_OPTIONS = [
  { value: 'stable', label: 'Stable' },
  { value: 'nightly', label: 'Nightly' },
] as const;

export type UpdateChannel = (typeof UPDATE_CHANNEL_OPTIONS)[number]['value'];

export const DEFAULT_UPDATE_CHANNEL: UpdateChannel = 'stable';
export const NIGHTLY_PRERELEASE_IDENTIFIER = 'nightly';

const RELEASE_TAG_HREF_PATTERN = /\/releases\/tag\/([^"'<>\s?#]+)/g;

export type ChannelRelease = { tag: string; version: string };

export function parseUpdateChannel(value: unknown): UpdateChannel {
  return (
    UPDATE_CHANNEL_OPTIONS.find((option) => option.value === value)?.value ??
    DEFAULT_UPDATE_CHANNEL
  );
}

export function updateChannelLabel(channel: UpdateChannel): string {
  return (
    UPDATE_CHANNEL_OPTIONS.find((option) => option.value === channel)?.label ??
    channel
  );
}

export function releaseVersionFromTag(tag: string): string | null {
  const version = tag.startsWith('v') ? tag.slice(1) : tag;
  return parseSemverVersion(version) ? version : null;
}

export function isStableReleaseVersion(version: string) {
  return parseSemverVersion(version)?.prerelease.length === 0;
}

export function isNightlyReleaseVersion(version: string) {
  return (
    parseSemverVersion(version)?.prerelease[0] === NIGHTLY_PRERELEASE_IDENTIFIER
  );
}

export function isReleaseVersionOnChannel(
  version: string,
  channel: UpdateChannel,
) {
  if (isStableReleaseVersion(version)) return true;
  return channel === 'nightly' && isNightlyReleaseVersion(version);
}

export function channelMetadataNameFor(version: string) {
  return isNightlyReleaseVersion(version)
    ? NIGHTLY_PRERELEASE_IDENTIFIER
    : 'latest';
}

export function releasesNewestFirst(
  tags: readonly string[],
  channel: UpdateChannel,
): ChannelRelease[] {
  const seenVersions = new Set<string>();
  const releases: ChannelRelease[] = [];
  for (const tag of tags) {
    const version = releaseVersionFromTag(tag);
    if (
      !version ||
      seenVersions.has(version) ||
      !isReleaseVersionOnChannel(version, channel)
    )
      continue;
    seenVersions.add(version);
    releases.push({ tag, version });
  }
  return releases.sort(
    (left, right) => compareSemverVersions(right.version, left.version) ?? 0,
  );
}

function decodeReleaseTag(encoded: string) {
  try {
    return decodeURIComponent(encoded);
  } catch {
    return encoded;
  }
}

export function releaseTagsFromAtomFeed(feed: string): string[] {
  const tags = new Set<string>();
  for (const match of feed.matchAll(RELEASE_TAG_HREF_PATTERN))
    tags.add(decodeReleaseTag(match[1]));
  return [...tags];
}

export function isUpdateInfoForChannel(info: unknown, channel: UpdateChannel) {
  return (
    typeof info === 'object' &&
    info !== null &&
    (info as { updateChannel?: unknown }).updateChannel === channel
  );
}
