import type { JobSnapshot } from './jobs';

type EventSubscriber = Pick<JobSnapshot, 'enabled' | 'owner' | 'triggers'>;

export function hasEnabledExtensionEventSubscriber(
  jobs: EventSubscriber[],
  event: string,
) {
  return jobs.some(
    (job) =>
      job.enabled &&
      job.owner === 'extension' &&
      job.triggers.some(
        (trigger) => trigger.type === 'event' && trigger.event === event,
      ),
  );
}
