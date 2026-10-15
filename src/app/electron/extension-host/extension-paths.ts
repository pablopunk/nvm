import path from 'node:path';

interface ExtensionPathOptions {
  homeDirectory: string;
  userDataDirectory: string;
  useUserDataForExtensions?: boolean;
  joinPath?: (...paths: string[]) => string;
}

function resolveExtensionPaths(options: ExtensionPathOptions) {
  const joinPath = options.joinPath ?? path.join;
  return {
    extensionsDir: options.useUserDataForExtensions
      ? joinPath(options.userDataDirectory, 'extensions')
      : joinPath(options.homeDirectory, '.config', 'nevermind', 'extensions'),
    extensionDraftsDir: joinPath(options.userDataDirectory, 'extension-drafts'),
  };
}

export { resolveExtensionPaths };
export type { ExtensionPathOptions };
