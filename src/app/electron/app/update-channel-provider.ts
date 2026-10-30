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
  releasesNewestFirst,
  type UpdateChannel,
} from './update-channel';

const MAX_METADATA_CANDIDATES = 3;

type GitHubFeedOptions = ConstructorParameters<typeof GitHubProvider>[0];

export type ChannelFeedOptions = GitHubFeedOptions & {
  resolveChannel: () => UpdateChannel;
};

export type ChannelUpdateInfo = UpdateInfo & {
  tag: string;
  updateChannel: UpdateChannel;
};

export function hasCompatibleMacUpdate(
  info: UpdateInfo,
  arch: string,
): boolean {
  if (arch !== 'arm64' && arch !== 'x64') return false;
  return (info.files ?? []).some((file) =>
    file.url.endsWith(`-mac-${arch}.zip`),
  );
}

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
        channel,
      );
      if (info) return { ...info, tag, updateChannel: channel };
    }
    throw Object.assign(
      new Error(`No ${channel} release has published update metadata`),
      { code: 'ERR_UPDATER_NO_PUBLISHED_VERSIONS' },
    );
  }

  private releasesPath(_channel?: UpdateChannel) {
    return `/${this.options.owner}/${this.options.repo}/releases`;
  }

  private async listReleaseTags(
    channel: UpdateChannel,
    cancellationToken: CancellationToken,
  ) {
    if (channel === 'dev' || channel === 'nightly') {
      const stable =
        channel === 'nightly'
          ? await this.fetchLatestStableTag(cancellationToken)
          : null;
      const url = new URL(
        `https://raw.githubusercontent.com/${this.options.owner}/${this.options.repo}/release-pointers/.channels/${channel}.json`,
      );
      url.searchParams.set('check', String(Date.now()));
      const data = await this.httpRequest(
        url,
        { 'Cache-Control': 'no-cache' },
        cancellationToken,
      );
      const pointer = JSON.parse(data ?? 'null');
      if (
        pointer?.kind !== 'nevermind-channel-pointer' ||
        pointer.channel !== channel ||
        !Array.isArray(pointer.tags) ||
        pointer.tags.length > 20 ||
        pointer.tags.some((tag: unknown) => typeof tag !== 'string')
      )
        throw new Error('Invalid channel pointer');
      return stable ? [...pointer.tags, stable] : pointer.tags;
    }
    const latestStableTag = await this.fetchLatestStableTag(cancellationToken);
    const latestTags = latestStableTag ? [latestStableTag] : [];
    return latestTags;
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
    const channelFile = `${this.getCustomChannelName(channel === 'dev' || tag.endsWith('-nightly') ? 'nightly' : channelMetadataNameFor(version))}.yml`;
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
      if (
        process.platform === 'darwin' &&
        !hasCompatibleMacUpdate(info, process.arch)
      ) {
        throw new Error(`No compatible macOS update for ${process.arch}`);
      }
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
