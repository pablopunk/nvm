import { FILE_INDEX_CHANGED_EVENT } from '../files/file-index-watcher';
import { frontmostApp, hasCapability } from '../os/os';
import type { JobDefinition, JobRegistry, JobTrigger } from '../platform/jobs';
import {
  CLIPBOARD_IMAGE_GC_INTERVAL_MS,
  CLIPBOARD_IMAGE_GC_JOB_ID,
  CLIPBOARD_IMAGE_GC_STARTUP_DELAY_MS,
  type ClipboardImageGcDeps,
  createClipboardImageGcJob,
} from './clipboard-image-gc-job';
import { hasEnabledExtensionEventSubscriber } from './frontmost-app-polling';

export const FRONTMOST_APP_CHANGED_EVENT = 'app.frontmost.changed';
const FRONTMOST_APP_POLL_JOB_ID = 'frontmost-app.poll';
const APP_REINDEX_DEBOUNCE_MS = 1000;
const FILE_INDEX_FRESHNESS_TRIGGERS: JobTrigger[] = [
  { type: 'startup', delayMs: 200 },
  {
    type: 'event',
    event: FILE_INDEX_CHANGED_EVENT,
    debounceMs: 1500,
    maxWaitMs: 10_000,
  },
  { type: 'event', event: 'wake', debounceMs: 5000 },
  { type: 'interval', everyMs: 15 * 60_000 },
];

type JobRun = JobDefinition['run'];

export type HostJobsDeps = {
  jobRegistry: JobRegistry;
  saveUserState: JobRun;
  appIndexService: { indexApplications: JobRun };
  appIconCache: { processPending: JobRun };
  indexFiles: JobRun;
  processPendingThumbnails: JobRun;
  clipboardImages: ClipboardImageGcDeps;
};

export function createHostJobs(deps: HostJobsDeps) {
  const {
    jobRegistry,
    saveUserState,
    appIndexService,
    appIconCache,
    indexFiles,
    processPendingThumbnails,
    clipboardImages,
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
      triggers: FILE_INDEX_FRESHNESS_TRIGGERS,
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
    jobRegistry.register({
      id: CLIPBOARD_IMAGE_GC_JOB_ID,
      title: 'Clipboard Image Cleanup',
      owner: 'host',
      scope: 'cache',
      timeoutMs: 20_000,
      triggers: [
        {
          type: 'interval',
          everyMs: CLIPBOARD_IMAGE_GC_INTERVAL_MS,
          delayMs: CLIPBOARD_IMAGE_GC_STARTUP_DELAY_MS,
        },
      ],
      run: createClipboardImageGcJob(clipboardImages).run,
    });
  }

  return { stopFrontmostAppPolling, syncFrontmostAppPolling, registerHostJobs };
}
