import type { UpdateChannel } from './update-channel';

type UpdateInfo = {
  version?: string;
  updateChannel?: UpdateChannel;
  nevermindStateEpoch?: unknown;
};
type UpdateCheckResult = {
  isUpdateAvailable: boolean;
  updateInfo?: UpdateInfo;
};
type CancellationTokenLike = {
  readonly cancelled: boolean;
  cancel: () => void;
};
type AutoUpdaterLike = {
  autoDownload: boolean;
  autoInstallOnAppQuit: boolean;
  allowDowngrade: boolean;
  checkForUpdates: () => Promise<UpdateCheckResult | null | undefined>;
  downloadUpdate: (
    cancellationToken?: CancellationTokenLike,
  ) => Promise<unknown>;
  quitAndInstall: () => void;
  on: (event: string, listener: (...args: any[]) => void) => void;
};

type UpdateStatus =
  | 'idle'
  | 'unsupported'
  | 'checking'
  | 'available'
  | 'downloading'
  | 'downloaded'
  | 'installing'
  | 'error';

type UpdateState = {
  checkInFlight: boolean;
  downloadInFlight: boolean;
  installInFlight: boolean;
  status: UpdateStatus;
  availableInfo: UpdateInfo | null;
  downloadedInfo: UpdateInfo | null;
  latestVersion: string | null;
  errorMessage: string;
};

type UpdateManagerOptions = {
  getUpdateChannel?: () => UpdateChannel;
  prepareUpdateFeed?: () => Promise<unknown>;
  createCancellationToken?: () => CancellationTokenLike;
  canInstallUpdate?: (info: UpdateInfo) => boolean | Promise<boolean>;
};

function createLocalCancellationToken(): CancellationTokenLike {
  let cancelled = false;
  return {
    get cancelled() {
      return cancelled;
    },
    cancel() {
      cancelled = true;
    },
  };
}

export { createLocalCancellationToken };
export type {
  AutoUpdaterLike,
  CancellationTokenLike,
  UpdateInfo,
  UpdateManagerOptions,
  UpdateState,
};
