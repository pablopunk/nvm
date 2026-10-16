import { frontmostApp, hasCapability } from '../os/os';
import type { JobDefinition, JobRegistry } from '../platform/jobs';
import { hasEnabledExtensionEventSubscriber } from './frontmost-app-polling';

export const FRONTMOST_APP_CHANGED_EVENT = 'app.frontmost.changed';
const FRONTMOST_APP_POLL_JOB_ID = 'frontmost-app.poll';
const APP_REINDEX_DEBOUNCE_MS = 1000;

type JobRun = JobDefinition['run'];

export type HostJobsDeps = {
  jobRegistry: JobRegistry;
  saveUserState: JobRun;
  appIndexService: { indexApplications: JobRun };
  appIconCache: { processPending: JobRun };
  indexFiles: JobRun;
  processPendingThumbnails: JobRun;
};

export function createHostJobs(deps: HostJobsDeps) {
  const {
    jobRegistry,
    saveUserState,
    appIndexService,
    appIconCache,
    indexFiles,
    processPendingThumbnails,
  } = deps;
  let frontmostWatcherLastId = '';

  async function pollFrontmostAppChange() {
    if (!hasCapability('frontmost-app')) return;
    const current: any = await frontmostApp();
    const currentId = current?.bundleId || current?.path || current?.name || '';
    if (!currentId || currentId === frontmostWatcherLastId) return;
    frontmostWatcherLastId = currentId;
    jobRegistry.emit(FRONTMOST_APP_CHANGED_EVENT);
  }

  function stopFrontmostAppPolling() {
    jobRegistry.unregister(FRONTMOST_APP_POLL_JOB_ID);
    frontmostWatcherLastId = '';
  }

  function syncFrontmostAppPolling() {
    const hasSubscriber = hasEnabledExtensionEventSubscriber(
      jobRegistry.snapshot(),
      FRONTMOST_APP_CHANGED_EVENT,
    );
    if (!hasSubscriber) return stopFrontmostAppPolling();
    if (jobRegistry.has(FRONTMOST_APP_POLL_JOB_ID)) return;
    jobRegistry.register({
      id: FRONTMOST_APP_POLL_JOB_ID,
      title: 'Frontmost App Poll',
      owner: 'host',
      scope: 'apps',
      triggers: [{ type: 'interval', everyMs: 5000, delayMs: 5000 }],
      timeoutMs: 3000,
      run: pollFrontmostAppChange,
    });
  }

  function registerHostJobs() {
    jobRegistry.register({
      id: 'state.save',
      title: 'Save User State',
      owner: 'host',
      scope: 'state',
      timeoutMs: 5000,
      run: saveUserState,
    });
    jobRegistry.register({
      id: 'apps.index',
      title: 'Application Index',
      owner: 'host',
      scope: 'apps',
      triggers: [
        { type: 'startup', delayMs: 100 },
        {
          type: 'event',
          event: 'apps.changed',
          debounceMs: APP_REINDEX_DEBOUNCE_MS,
        },
      ],
      timeoutMs: 30_000,
      run: appIndexService.indexApplications,
    });
    jobRegistry.register({
      id: 'files.index',
      title: 'File Index',
      owner: 'host',
      scope: 'files',
      triggers: [{ type: 'startup', delayMs: 200 }],
      timeoutMs: 30_000,
      run: indexFiles,
    });
    jobRegistry.register({
      id: 'cache.app-icons',
      title: 'App Icon Cache',
      owner: 'host',
      scope: 'cache',
      timeoutMs: 15_000,
      run: appIconCache.processPending,
    });
    jobRegistry.register({
      id: 'cache.thumbnails',
      title: 'Thumbnail Cache',
      owner: 'host',
      scope: 'cache',
      timeoutMs: 20_000,
      run: processPendingThumbnails,
    });
  }

  return { stopFrontmostAppPolling, syncFrontmostAppPolling, registerHostJobs };
}
