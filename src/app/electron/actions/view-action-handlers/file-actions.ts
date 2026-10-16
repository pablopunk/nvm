import { expandUserPath } from '../../files/file-utils';
import {
  openPathWithApp,
  quickLookPath,
} from '../../files/quick-look-and-open-with';
import { warn as logWarn } from '../../platform/logger';
import { openExternalUrl } from '../../platform/url-utils';
import { runInBackground } from '../run-in-background';

export type FileActionHandlerDeps = {
  shell: {
    openPath: (path: string) => Promise<string>;
    showItemInFolder: (fullPath: string) => void;
    trashItem: (path: string) => Promise<void>;
  };
};

export function createFileActionHandlers(deps: FileActionHandlerDeps) {
  const { shell } = deps;

  function handleOpenPath(action) {
    runInBackground(() => shell.openPath(action.path), action.traceId);
  }

  function handleRevealPath(action) {
    runInBackground(() => shell.showItemInFolder(action.path), action.traceId);
  }

  function handleQuickLook(action) {
    return quickLookPath(action.path);
  }

  function handleOpenWith(action) {
    runInBackground(
      () => openPathWithApp(action.path, action.appPath || action.app?.path),
      action.traceId,
    );
  }

  function handleOpenUrl(action) {
    runInBackground(async () => {
      if (!(await openExternalUrl(action.url)))
        logWarn(
          'openUrl.rejected',
          { url: action.url },
          { source: 'host', scope: 'action' },
        );
    }, action.traceId);
  }

  async function handleTrash(action) {
    const results: Array<{ path: string; ok: boolean }> = [];
    for (const itemPath of action.paths || [action.path]) {
      if (!itemPath) continue;
      const fullPath = expandUserPath(itemPath);
      try {
        let timer: NodeJS.Timeout | undefined;
        const timedOut = new Promise<'timedOut'>((resolve) => {
          timer = setTimeout(() => resolve('timedOut'), 10_000);
        });
        const outcome = await Promise.race([
          shell.trashItem(fullPath).then(() => 'ok' as const),
          timedOut,
        ]);
        clearTimeout(timer);
        if (outcome === 'timedOut') {
          logWarn(
            'extension.trash.timedOut',
            { path: fullPath },
            { source: 'host', scope: 'extension' },
          );
          results.push({ path: fullPath, ok: false });
        } else {
          results.push({ path: fullPath, ok: true });
        }
      } catch (err: any) {
        logWarn(
          'extension.trash.failed',
          { path: fullPath, error: err?.message },
          { source: 'host', scope: 'extension' },
        );
        results.push({ path: fullPath, ok: false });
      }
    }
    const successCount = results.filter((r) => r.ok).length;
    const total = results.length;
    return {
      toast: {
        message:
          total === 0
            ? 'Nothing to trash'
            : successCount === total
              ? `Moved ${total} item${total > 1 ? 's' : ''} to Trash`
              : `Moved ${successCount}/${total} items to Trash`,
        tone: successCount === total ? 'default' : 'error',
      },
    };
  }

  return {
    handleOpenPath,
    handleRevealPath,
    handleQuickLook,
    handleOpenWith,
    handleOpenUrl,
    handleTrash,
  };
}
