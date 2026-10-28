import type { UpdateChannel } from './update-channel';
import { NevermindChannelProvider } from './update-channel-provider';

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
  });
}
