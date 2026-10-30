import fs from 'node:fs/promises';
import path from 'node:path';
import { app } from 'electron';
import {
  BaseUpdater,
  type ResolvedUpdateFileInfo,
  type UpdateDownloadedEvent,
} from 'electron-updater';
import type { DownloadUpdateOptions } from 'electron-updater/out/AppUpdater.js';
import {
  type PortableExecutable,
  type PortableUpdateHelperPlan,
  preparePortableUpdateHelper,
} from '../os/windows-portable-update';

type PortableUpdatePackage = Pick<
  PortableUpdateHelperPlan,
  'target' | 'sourcePath' | 'sha512' | 'version'
>;

export function portableUpdateWorkDirectory() {
  return path.join(app.getPath('userData'), 'portable-update');
}

function readPortableUpdateHelperScript() {
  return fs.readFile(
    path.join(
      app.getAppPath(),
      'src',
      'app',
      'resources',
      'windows-portable-update.ps1',
    ),
    'utf8',
  );
}

export function preparePortableUpdate(update: PortableUpdatePackage) {
  return preparePortableUpdateHelper(
    {
      ...update,
      currentVersion: app.getVersion(),
      waitProcessIds: [process.pid, process.ppid],
      workDirectory: portableUpdateWorkDirectory(),
    },
    { helperScript: readPortableUpdateHelperScript },
  );
}

export function selectPortableUpdateFile(
  files: ResolvedUpdateFileInfo[],
  arch: string,
) {
  const suffix = `-win-${arch}-portable.exe`;
  const portableFiles = files.filter((file) =>
    decodeURIComponent(file.url.pathname).toLowerCase().endsWith(suffix),
  );
  if (portableFiles.length !== 1)
    throw Object.assign(
      new Error(
        `Update metadata must list exactly one ${suffix} file; found ${portableFiles.length}.`,
      ),
      { code: 'ERR_UPDATER_INVALID_UPDATE_INFO' },
    );
  return portableFiles[0];
}

/** Updates the electron-builder portable executable in place through a detached helper that runs after Nevermind exits. */
export class WindowsPortableUpdater extends BaseUpdater {
  private readonly target: PortableExecutable;
  private readonly prepare: typeof preparePortableUpdate;
  private launchHelper: (() => void) | null = null;
  private pendingDownloadedEvent: UpdateDownloadedEvent | null = null;

  constructor(
    target: PortableExecutable,
    prepare: typeof preparePortableUpdate = preparePortableUpdate,
    ...updaterArguments: ConstructorParameters<typeof BaseUpdater>
  ) {
    super(...updaterArguments);
    this.target = target;
    this.prepare = prepare;
  }

  protected async doDownloadUpdate(
    downloadUpdateOptions: DownloadUpdateOptions,
  ) {
    const { info, provider } = downloadUpdateOptions.updateInfoAndProvider;
    const fileInfo = selectPortableUpdateFile(
      provider.resolveFiles(info),
      process.arch,
    );
    this.launchHelper = null;
    this.pendingDownloadedEvent = null;
    const result = await this.executeDownload({
      fileExtension: 'exe',
      downloadUpdateOptions,
      fileInfo,
      task: (destinationFile, downloadOptions) =>
        this.httpExecutor.download(
          fileInfo.url,
          destinationFile,
          downloadOptions,
        ),
    });
    const sourcePath = this.installerPath;
    if (!sourcePath) throw new Error('The downloaded update is missing.');
    this.launchHelper = await this.prepare({
      target: this.target,
      sourcePath,
      sha512: fileInfo.info.sha512,
      version: info.version,
    });
    const downloaded = this.pendingDownloadedEvent;
    this.pendingDownloadedEvent = null;
    if (downloaded) super.dispatchUpdateDownloaded(downloaded);
    return result;
  }

  /** Announces the download only after the helper is staged, so Restart to Update cannot race preparation. */
  protected dispatchUpdateDownloaded(event: UpdateDownloadedEvent) {
    this.pendingDownloadedEvent = event;
  }

  protected doInstall() {
    if (!this.launchHelper) {
      this.dispatchError(new Error('No downloaded update is ready to install'));
      return false;
    }
    try {
      this.launchHelper();
      return true;
    } catch (error) {
      this.dispatchError(
        error instanceof Error ? error : new Error(String(error)),
      );
      return false;
    }
  }
}
