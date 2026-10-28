import { execFile } from 'node:child_process';

const SAFE_BUNDLE_ID = /^[A-Za-z0-9.-]+$/;

export function findMacAppsWithBundleId(bundleId: string) {
  return new Promise<string[]>((resolve, reject) => {
    if (!SAFE_BUNDLE_ID.test(bundleId)) {
      reject(new Error('Invalid application identifier'));
      return;
    }
    const query = `(kMDItemCFBundleIdentifier == '${bundleId}'c || kMDItemCFBundleIdentifier == '${bundleId}.*'c) && kMDItemContentTypeTree == 'com.apple.application-bundle'`;
    execFile(
      '/usr/bin/mdfind',
      [query],
      { timeout: 2000, maxBuffer: 1024 * 1024 },
      (error, stdout) => {
        if (error) {
          reject(error);
          return;
        }
        resolve(applicationPathsFromSpotlight(stdout));
      },
    );
  });
}

export function applicationPathsFromSpotlight(output: string) {
  return output.split(/\r?\n/).filter((item) => {
    const components = item.split('/');
    return (
      item.endsWith('.app') &&
      !item.includes('.app/') &&
      !components.some(
        (component) => component === '.Trash' || component === '.Trashes',
      )
    );
  });
}
