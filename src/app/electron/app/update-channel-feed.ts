import type { UpdateChannel } from './update-channel';
import { NevermindChannelProvider } from './update-channel-provider';

declare const __NVM_DEV_RELEASE_REPOSITORY__: string;

type UpdateFeedHost = {
  configOnDisk: { value: Promise<Record<string, unknown>> };
  setFeedURL: (options: Record<string, unknown>) => void;
};

export async function installNevermindChannelFeed(
  host: UpdateFeedHost,
  resolveChannel: () => UpdateChannel,
) {
  const publishConfig = await host.configOnDisk.value;
  host.setFeedURL({
    ...publishConfig,
    provider: 'custom',
    updateProvider: NevermindChannelProvider,
    resolveChannel,
    devRepository:
      typeof __NVM_DEV_RELEASE_REPOSITORY__ === 'string'
        ? __NVM_DEV_RELEASE_REPOSITORY__
        : '',
  });
}
