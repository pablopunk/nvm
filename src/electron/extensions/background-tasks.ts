import type { JobSnapshot } from '../jobs';
import { extensionContext } from './_context';

function jobTime(value?: number) {
  return value ? new Date(value).toLocaleTimeString() : 'Never';
}

function jobTriggerSummary(job: JobSnapshot) {
  if (!job.triggers.length) return 'Manual';
  return job.triggers
    .map((trigger: any) => {
      if (trigger.type === 'startup') return 'Startup';
      if (trigger.type === 'interval')
        return `Every ${Math.round(trigger.everyMs / 1000)}s`;
      if (trigger.type === 'event') return `On ${trigger.event}`;
      return 'Manual';
    })
    .join(' · ');
}

function jobSubtitle(job: JobSnapshot) {
  const status =
    job.status === 'running'
      ? 'Running'
      : job.status === 'failed'
        ? `Failed: ${job.lastError || 'unknown error'}`
        : job.status === 'backing-off'
          ? `Backing off until ${jobTime(job.backoffUntil)}`
          : job.status;
  return `${status} · ${jobTriggerSummary(job)} · Last: ${jobTime(job.lastFinishedAt)}`;
}

function jobHistoryMarkdown(job: JobSnapshot) {
  if (!job.history.length) return '_No runs recorded yet._';
  return job.history
    .map(
      (entry) =>
        `- ${new Date(entry.finishedAt).toLocaleTimeString()} · ${entry.status} · ${entry.reason} · ${entry.durationMs}ms${entry.error ? ` · ${entry.error}` : ''}`,
    )
    .join('\n');
}

function jobDetailsMarkdown(job: JobSnapshot) {
  return [
    `# ${job.title}`,
    '',
    `- ID: ${job.id}`,
    `- Owner: ${job.owner}`,
    job.scope ? `- Scope: ${job.scope}` : '',
    `- Status: ${job.status}`,
    `- Enabled: ${job.enabled ? 'yes' : 'no'}`,
    `- Running: ${job.running ? 'yes' : 'no'}`,
    `- Triggers: ${jobTriggerSummary(job)}`,
    `- Runs: ${job.runCount}`,
    `- Failures: ${job.failureCount}`,
    `- Last reason: ${job.lastReason || '-'}`,
    `- Last started: ${jobTime(job.lastStartedAt)}`,
    `- Last finished: ${jobTime(job.lastFinishedAt)}`,
    job.lastDurationMs == null
      ? ''
      : `- Last duration: ${job.lastDurationMs}ms`,
    job.nextRunAt ? `- Next run: ${jobTime(job.nextRunAt)}` : '',
    job.backoffUntil ? `- Backoff until: ${jobTime(job.backoffUntil)}` : '',
    `- Consecutive failures: ${job.consecutiveFailures}`,
    job.lastError ? `\n## Last error\n\n${job.lastError}` : '',
    `\n## Recent runs\n\n${jobHistoryMarkdown(job)}`,
  ]
    .filter(Boolean)
    .join('\n');
}

function backgroundTasksView(ctx) {
  const jobs = extensionContext.jobRegistry.snapshot();
  return ctx.ui.list({
    id: 'background-tasks',
    title: 'Background Tasks',
    subtitle: `${jobs.length} host-managed jobs`,
    presentation: 'root',
    searchBarPlaceholder: 'Search background tasks',
    emptyView: { title: 'No background tasks registered.' },
    items: jobs.map((job) => jobItem(ctx, job)),
  });
}

function jobItem(ctx, job: JobSnapshot) {
  const runNow = ctx.actions.run('Run Now', async () => {
    try {
      await extensionContext.jobRegistry.run(job.id, 'manual');
      return {
        toast: { message: `Ran ${job.title}` },
        view: backgroundTasksView(ctx),
        navigation: 'replace',
      };
    } catch (error) {
      return {
        view: {
          type: 'preview',
          title: `${job.title} Failed`,
          content: `# ${job.title} Failed\n\n${error instanceof Error ? error.message : String(error)}`,
        },
      };
    }
  });
  const toggle = ctx.actions.run(
    job.enabled ? 'Disable Job' : 'Enable Job',
    async () => {
      extensionContext.jobRegistry.setEnabled(job.id, !job.enabled);
      await extensionContext.saveUserState();
      return {
        toast: {
          message: `${job.enabled ? 'Disabled' : 'Enabled'} ${job.title}`,
        },
        view: backgroundTasksView(ctx),
        navigation: 'replace',
      };
    },
  );
  const clearError = ctx.actions.run('Clear Error', () => {
    extensionContext.jobRegistry.clearError(job.id);
    return {
      toast: { message: `Cleared ${job.title}` },
      view: backgroundTasksView(ctx),
      navigation: 'replace',
    };
  });
  const showDetails = ctx.actions.push('Show Details', {
    type: 'preview',
    title: job.title,
    content: jobDetailsMarkdown(job),
  });
  return {
    id: `job:${job.id}`,
    title: job.title,
    subtitle: jobSubtitle(job),
    icon:
      job.status === 'failed' || job.status === 'backing-off'
        ? 'circle-alert'
        : job.running
          ? 'loader'
          : job.enabled
            ? 'activity'
            : 'circle-pause',
    accessories: [{ text: job.owner }, { text: job.status }],
    primaryAction: showDetails,
    actionPanel: {
      sections: [
        {
          actions: [
            showDetails,
            runNow,
            toggle,
            job.lastError ? clearError : null,
          ].filter(Boolean),
        },
      ],
    },
  };
}

export function createBackgroundTasksExtension() {
  return {
    id: 'nevermind.background-tasks',
    title: 'Background Tasks',
    capabilities: [] as const,
    commands: [
      {
        id: 'background-tasks',
        actionId: 'background-tasks',
        title: 'Background Tasks',
        subtitle: 'Inspect and run host-managed background jobs',
        icon: 'activity',
        score: 16,
        run: (ctx) => backgroundTasksView(ctx),
      },
    ],
  };
}
