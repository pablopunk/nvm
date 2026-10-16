import type { JobDefinition } from '../platform/jobs';

export interface ClipboardWatcherDeps {
  createWatcherJob: () => Promise<JobDefinition>;
  registerJob: (job: JobDefinition) => void;
}

export function createClipboardWatcher(deps: ClipboardWatcherDeps) {
  async function startClipboardWatcher() {
    const watcherJob = await deps.createWatcherJob();
    deps.registerJob(watcherJob);
  }

  return { startClipboardWatcher };
}
