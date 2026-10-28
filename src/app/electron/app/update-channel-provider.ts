import {
  type AppUpdater,
  CancellationToken,
  type UpdateInfo,
} from 'electron-updater';
import { GitHubProvider } from 'electron-updater/out/providers/GitHubProvider.js';
import {
  parseUpdateInfo,
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
};

export type ChannelUpdateInfo = UpdateInfo & {
  tag: string;
  updateChannel: UpdateChannel;
};

// Stock GitHubProvider only understands channel-named prereleases; this provider picks the newest eligible release by semver across stable and nightly tags.
export class NevermindChannelProvider extends GitHubProvider {
  private readonly resolveChannel: () => UpdateChannel;

  constructor(
    options: ChannelFeedOptions,
    updater: AppUpdater,
    runtimeOptions: ProviderRuntimeOptions,
  ) {
    super(options, updater, runtimeOptions);
    this.resolveChannel = options.resolveChannel;
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
      );
      if (info) return { ...info, tag, updateChannel: channel };
    }
    throw Object.assign(
      new Error(`No ${channel} release has published update metadata`),
      { code: 'ERR_UPDATER_NO_PUBLISHED_VERSIONS' },
    );
  }

  private releasesPath() {
    return `/${this.options.owner}/${this.options.repo}/releases`;
  }

  private async listReleaseTags(
    channel: UpdateChannel,
    cancellationToken: CancellationToken,
  ) {
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
  ): Promise<UpdateInfo | null> {
    const channelFile = `${this.getCustomChannelName(channelMetadataNameFor(version))}.yml`;
    const channelFileUrl = new URL(
      `${this.releasesPath()}/download/${tag}/${channelFile}`,
      this.baseUrl,
    );
    try {
      const rawData = await this.executor.request(
        this.createRequestOptions(channelFileUrl),
        cancellationToken,
      );
      return parseUpdateInfo(rawData, channelFile, channelFileUrl);
    } catch (error) {
      if ((error as { statusCode?: number }).statusCode === 404) return null;
      throw error;
    }
  }
}
