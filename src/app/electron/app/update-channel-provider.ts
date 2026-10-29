import {
  type AppUpdater,
  CancellationToken,
  type UpdateInfo,
} from 'electron-updater';
import { GitHubProvider } from 'electron-updater/out/providers/GitHubProvider.js';
import {
  parseUpdateInfo,
  resolveFiles,
  type ProviderRuntimeOptions,
} from 'electron-updater/out/providers/Provider.js';
import {
  channelMetadataNameFor,
  releaseTagsFromAtomFeed,
  releasesNewestFirst,
  type UpdateChannel,
} from './update-channel';

const MAX_METADATA_CANDIDATES = 3;
const RELEASES_FEED_ACCEPT =
  'application/xml, application/atom+xml, text/xml, */*';

type GitHubFeedOptions = ConstructorParameters<typeof GitHubProvider>[0];

export type ChannelFeedOptions = GitHubFeedOptions & {
  resolveChannel: () => UpdateChannel;
  devRepository?: string;
};

export type ChannelUpdateInfo = UpdateInfo & {
  tag: string;
  updateChannel: UpdateChannel;
};

// Stock GitHubProvider only understands channel-named prereleases; this provider picks the newest eligible release by semver across stable and nightly tags.
export class NevermindChannelProvider extends GitHubProvider {
  private readonly resolveChannel: () => UpdateChannel;
  private readonly devRepository: string;

  constructor(
    options: ChannelFeedOptions,
    updater: AppUpdater,
    runtimeOptions: ProviderRuntimeOptions,
  ) {
    super(options, updater, runtimeOptions);
    this.resolveChannel = options.resolveChannel;
    this.devRepository = options.devRepository ?? '';
  }

  async getLatestVersion(): Promise<ChannelUpdateInfo> {
    const channel = this.resolveChannel();
    const cancellationToken = new CancellationToken();
    const tags = await this.listReleaseTags(channel, cancellationToken);
    const candidates = releasesNewestFirst(tags, channel).slice(
      0,
      MAX_METADATA_CANDIDATES,
    );
    for (const { tag, version } of candidates) {
      const info = await this.fetchChannelMetadata(
        tag,
        version,
        cancellationToken,
        channel,
      );
      if (info) return { ...info, tag, updateChannel: channel };
    }
    throw Object.assign(
      new Error(`No ${channel} release has published update metadata`),
      { code: 'ERR_UPDATER_NO_PUBLISHED_VERSIONS' },
    );
  }

  private releasesPath(channel?: UpdateChannel) {
    if (channel === 'dev') {
      if (
        !/^[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+$/.test(this.devRepository) ||
        this.devRepository.toLowerCase() ===
          `${this.options.owner}/${this.options.repo}`.toLowerCase()
      )
        throw new Error('Dev release repository is not configured');
      return `/${this.devRepository}/releases`;
    }
    return `/${this.options.owner}/${this.options.repo}/releases`;
  }

  private async listReleaseTags(
    channel: UpdateChannel,
    cancellationToken: CancellationToken,
  ) {
    if (channel === 'dev') {
      this.releasesPath(channel);
      const tags: string[] = [];
      for (let page = 1; page <= 10; page += 1) {
        const url = new URL(
          `https://api.github.com/repos/${this.devRepository}/releases`,
        );
        url.searchParams.set('per_page', '100');
        url.searchParams.set('page', String(page));
        const data = await this.httpRequest(
          url,
          { Accept: 'application/vnd.github+json' },
          cancellationToken,
        );
        const releases: unknown = JSON.parse(data ?? 'null');
        if (!Array.isArray(releases))
          throw new Error('Invalid dev release list');
        for (const release of releases) {
          if (
            release?.draft === false &&
            release?.prerelease === true &&
            typeof release?.tag_name === 'string'
          )
            tags.push(release.tag_name);
        }
        if (releases.length < 100) return tags;
      }
      throw new Error('Dev release list exceeds pagination limit');
    }
    const latestStableTag = await this.fetchLatestStableTag(cancellationToken);
    const latestTags = latestStableTag ? [latestStableTag] : [];
    if (channel === 'stable') return latestTags;
    // The feed lists only the ten newest releases, so it cannot find stable tags on its own.
    const feed = await this.httpRequest(
      new URL(`${this.releasesPath()}.atom`, this.baseUrl),
      { accept: RELEASES_FEED_ACCEPT },
      cancellationToken,
    );
    return [...latestTags, ...releaseTagsFromAtomFeed(feed ?? '')];
  }

  private async fetchLatestStableTag(cancellationToken: CancellationToken) {
    const rawData = await this.httpRequest(
      new URL(`${this.releasesPath()}/latest`, this.baseUrl),
      { Accept: 'application/json' },
      cancellationToken,
    );
    return rawData ? ((JSON.parse(rawData).tag_name as string) ?? null) : null;
  }

  private async fetchChannelMetadata(
    tag: string,
    version: string,
    cancellationToken: CancellationToken,
    channel: UpdateChannel,
  ): Promise<UpdateInfo | null> {
    const channelFile = `${this.getCustomChannelName(channel === 'dev' ? 'nightly' : channelMetadataNameFor(version))}.yml`;
    const channelFileUrl = new URL(
      `${this.releasesPath(channel)}/download/${tag}/${channelFile}`,
      this.baseUrl,
    );
    try {
      const rawData = await this.executor.request(
        this.createRequestOptions(channelFileUrl),
        cancellationToken,
      );
      const info = parseUpdateInfo(rawData, channelFile, channelFileUrl);
      if (info.version !== version)
        throw Object.assign(
          new Error(
            `Update metadata for ${tag} reports version ${info.version}`,
          ),
          { code: 'ERR_UPDATER_INVALID_UPDATE_INFO' },
        );
      return info;
    } catch (error) {
      if ((error as { statusCode?: number }).statusCode === 404) return null;
      throw error;
    }
  }

  resolveFiles(updateInfo: ChannelUpdateInfo) {
    return resolveFiles(
      updateInfo,
      this.baseUrl,
      (file) =>
        `${this.releasesPath(updateInfo.updateChannel)}/download/${updateInfo.tag}/${file.replace(/ /g, '-')}`,
    );
  }
}
