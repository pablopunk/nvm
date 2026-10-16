import { spawn } from 'node:child_process';
import path from 'node:path';
import {
  openFileWithApp as openFileWithOsApp,
  hasCapability,
  quickLookTitle,
} from '../os/os';
import { warn as logWarn } from '../platform/logger';
import { execFileText } from './exec-file-text';
import { expandUserPath } from './file-utils';
import { compatibleOpenWithApps, type OpenWithApp } from './open-with-apps';

export interface OpenWithAppsDeps {
  appIndexService: { get(): OpenWithApp[] };
}

export function quickLookPath(filePath) {
  if (!hasCapability('quick-look'))
    return {
      toast: {
        message: `${quickLookTitle()} is not available on this OS`,
        tone: 'error',
      },
    };
  const resolvedPath = expandUserPath(filePath);
  if (!(resolvedPath && path.isAbsolute(resolvedPath)))
    return {
      toast: { message: `Cannot ${quickLookTitle()} this item`, tone: 'error' },
    };
  const child = spawn('qlmanage', ['-p', resolvedPath], {
    detached: true,
    stdio: 'ignore',
  });
  child.on('error', (err) =>
    logWarn(
      'quickLook.failed',
      { path: resolvedPath, error: err?.message },
      { source: 'host', scope: 'action' },
    ),
  );
  child.unref();
}

async function contentTypesForPath(filePath) {
  if (!hasCapability('quick-look')) return [];
  try {
    const stdout = (await execFileText('mdls', [
      '-raw',
      '-name',
      'kMDItemContentTypeTree',
      filePath,
    ])) as string;
    return stdout.match(/"([^"]+)"/g)?.map((item) => item.slice(1, -1)) || [];
  } catch {
    return [];
  }
}

async function documentTypesForApp(appPath) {
  try {
    const stdout = (await execFileText('/usr/bin/plutil', [
      '-convert',
      'json',
      '-o',
      '-',
      path.join(appPath, 'Contents', 'Info.plist'),
    ])) as string;
    return JSON.parse(stdout).CFBundleDocumentTypes || [];
  } catch {
    return [];
  }
}

export function createOpenWithApps(deps: OpenWithAppsDeps) {
  const { appIndexService } = deps;

  async function openWithApps(filePath) {
    const resolvedPath = expandUserPath(filePath);
    if (!(resolvedPath && path.isAbsolute(resolvedPath))) return [];
    if (!hasCapability('open-with-app-filtering')) return appIndexService.get();
    const extension = path
      .extname(resolvedPath)
      .replace(/^\./, '')
      .toLowerCase();
    const contentTypes = new Set(await contentTypesForPath(resolvedPath));
    return compatibleOpenWithApps(
      appIndexService.get(),
      extension,
      contentTypes,
      documentTypesForApp,
    );
  }

  return openWithApps;
}

export async function openPathWithApp(filePath, appPath) {
  const resolvedPath = expandUserPath(filePath);
  const resolvedAppPath = expandUserPath(appPath);
  if (
    !(
      resolvedPath &&
      resolvedAppPath &&
      path.isAbsolute(resolvedPath) &&
      path.isAbsolute(resolvedAppPath)
    )
  )
    return {
      toast: { message: 'Cannot open this file with that app', tone: 'error' },
    };
  await openFileWithOsApp(resolvedPath, resolvedAppPath);
}
