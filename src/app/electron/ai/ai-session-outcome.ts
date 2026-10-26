import type { AgentSessionEvent } from '@earendil-works/pi-coding-agent';

type AssistantMessageOutcome = {
  stopReason?: string;
  errorMessage?: string;
};

export type AiSessionOutcomeTracker = {
  settled: boolean;
  piAborted: boolean;
  lastAssistantMessage?: AssistantMessageOutcome;
  recoveryError?: string;
  recoveryAborted: boolean;
  recoveryReason?: string;
  retryAttempts: number;
  summaryRetryAttempts: number;
};

export type AiSessionOutcome =
  | { kind: 'done' }
  | { kind: 'error'; message: string }
  | { kind: 'aborted' };

export function createAiSessionOutcomeTracker(): AiSessionOutcomeTracker {
  return {
    settled: false,
    piAborted: false,
    recoveryAborted: false,
    retryAttempts: 0,
    summaryRetryAttempts: 0,
  };
}

export function observeAiSessionOutcome(
  tracker: AiSessionOutcomeTracker,
  event: AgentSessionEvent,
) {
  if (event.type === 'agent_settled') {
    tracker.settled = true;
    tracker.piAborted = 'aborted' in event && event.aborted === true;
    return;
  }

  if (event.type === 'message_end' && event.message.role === 'assistant') {
    tracker.lastAssistantMessage = event.message;
    if (
      event.message.stopReason !== 'error' &&
      event.message.stopReason !== 'aborted'
    ) {
      tracker.recoveryError = undefined;
      tracker.recoveryAborted = false;
    }
    return;
  }

  if (event.type === 'compaction_end' && event.reason === 'overflow') {
    tracker.recoveryReason = event.aborted
      ? 'compaction-aborted'
      : event.errorMessage
        ? 'summary-failed'
        : event.result
          ? 'overflow-compacted'
          : 'overflow-unrecovered';
    tracker.recoveryAborted ||= event.aborted;
    if (event.errorMessage) tracker.recoveryError = event.errorMessage;
    else if (!event.result && !event.willRetry)
      tracker.recoveryError =
        'Pi could not compact the conversation after the provider rejected its context.';
    else if (event.result && event.willRetry) tracker.recoveryError = undefined;
    return;
  }

  if (event.type === 'auto_retry_end') {
    tracker.retryAttempts = Math.max(tracker.retryAttempts, event.attempt);
    if (!event.success && event.finalError) {
      tracker.recoveryReason = 'transient-retry-failed';
      tracker.recoveryError = event.finalError;
    }
    return;
  }

  const eventType: string = event.type;
  if (
    eventType === 'summarization_retry_scheduled' &&
    'attempt' in event &&
    typeof event.attempt === 'number'
  ) {
    tracker.summaryRetryAttempts = Math.max(
      tracker.summaryRetryAttempts,
      event.attempt,
    );
    tracker.recoveryReason = 'summary-retry-scheduled';
  }
}

export function resolveAiSessionOutcome(
  tracker: AiSessionOutcomeTracker,
  options: { abortRequested?: boolean; thrownError?: unknown } = {},
): AiSessionOutcome {
  if (options.abortRequested || tracker.piAborted || tracker.recoveryAborted)
    return { kind: 'aborted' };

  const assistantMessage = tracker.lastAssistantMessage;
  if (assistantMessage?.stopReason === 'aborted') return { kind: 'aborted' };

  if (options.thrownError !== undefined) {
    return {
      kind: 'error',
      message: errorMessage(options.thrownError),
    };
  }

  if (tracker.recoveryError) {
    return { kind: 'error', message: tracker.recoveryError };
  }

  if (!tracker.settled)
    return {
      kind: 'error',
      message: 'Pi ended the request before its recovery cycle settled.',
    };

  if (!assistantMessage) {
    return {
      kind: 'error',
      message: 'Pi ended without a final assistant response.',
    };
  }

  if (assistantMessage.stopReason === 'error') {
    return {
      kind: 'error',
      message: assistantMessage.errorMessage || 'AI request failed',
    };
  }

  if (assistantMessage.stopReason === 'toolUse') {
    return {
      kind: 'error',
      message: 'Pi ended while a tool call was still pending.',
    };
  }

  return { kind: 'done' };
}

function errorMessage(error: unknown) {
  return error instanceof Error ? error.message : String(error);
}
