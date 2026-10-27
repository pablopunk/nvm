import assert from 'node:assert/strict';
import test from 'node:test';
import type { AgentSessionEvent } from '@earendil-works/pi-coding-agent';
import {
  createAiSessionOutcomeTracker,
  observeAiSessionOutcome,
  resolveAiSessionOutcome,
} from './ai-session-outcome';

function observeEvent(
  tracker: ReturnType<typeof createAiSessionOutcomeTracker>,
  event: object,
) {
  observeAiSessionOutcome(tracker, event as AgentSessionEvent);
}

function overflowError(
  tracker: ReturnType<typeof createAiSessionOutcomeTracker>,
) {
  observeEvent(tracker, {
    type: 'message_end',
    message: {
      role: 'assistant',
      stopReason: 'error',
      errorMessage:
        'Prompt exceeds 100000 input tokens; context_length_exceeded',
    },
  });
}

test('missing summary credentials keep the overflow terminal', () => {
  const tracker = createAiSessionOutcomeTracker();
  overflowError(tracker);
  observeEvent(tracker, {
    type: 'compaction_end',
    reason: 'overflow',
    result: undefined,
    aborted: false,
    willRetry: false,
    errorMessage: 'No API key is configured for the summary provider.',
  });
  observeEvent(tracker, { type: 'agent_settled' });

  assert.deepEqual(resolveAiSessionOutcome(tracker), {
    kind: 'error',
    message: 'No API key is configured for the summary provider.',
  });
});

test('null summary preparation cannot turn the overflow into success', () => {
  const tracker = createAiSessionOutcomeTracker();
  overflowError(tracker);
  observeEvent(tracker, {
    type: 'compaction_end',
    reason: 'overflow',
    result: undefined,
    aborted: false,
    willRetry: false,
  });
  observeEvent(tracker, { type: 'agent_settled' });

  assert.deepEqual(resolveAiSessionOutcome(tracker), {
    kind: 'error',
    message:
      'Pi could not compact the conversation after the provider rejected its context.',
  });
});

test('a recovered assistant attempt supersedes the overflow after settlement', () => {
  const tracker = createAiSessionOutcomeTracker();
  overflowError(tracker);
  observeEvent(tracker, {
    type: 'compaction_end',
    reason: 'overflow',
    result: { summary: 'Condensed history' },
    aborted: false,
    willRetry: true,
  });
  observeEvent(tracker, {
    type: 'message_end',
    message: { role: 'assistant', stopReason: 'stop' },
  });
  observeEvent(tracker, { type: 'agent_settled' });

  assert.deepEqual(resolveAiSessionOutcome(tracker), { kind: 'done' });
});

test('summary failures and exhausted retries cannot become false success', () => {
  const compactionFailure = createAiSessionOutcomeTracker();
  overflowError(compactionFailure);
  observeEvent(compactionFailure, {
    type: 'compaction_end',
    reason: 'overflow',
    result: undefined,
    aborted: false,
    willRetry: false,
    errorMessage: 'summary credits exhausted',
  });
  observeEvent(compactionFailure, { type: 'agent_settled' });
  assert.deepEqual(resolveAiSessionOutcome(compactionFailure), {
    kind: 'error',
    message: 'summary credits exhausted',
  });

  const retryFailure = createAiSessionOutcomeTracker();
  overflowError(retryFailure);
  observeEvent(retryFailure, {
    type: 'auto_retry_end',
    success: false,
    attempt: 2,
    finalError: 'provider credits exhausted',
  });
  observeEvent(retryFailure, { type: 'agent_settled' });
  assert.deepEqual(resolveAiSessionOutcome(retryFailure), {
    kind: 'error',
    message: 'provider credits exhausted',
  });
  assert.equal(retryFailure.retryAttempts, 2);

  assert.deepEqual(
    resolveAiSessionOutcome(createAiSessionOutcomeTracker(), {
      thrownError: new Error('summary request failed'),
    }),
    { kind: 'error', message: 'summary request failed' },
  );
});

test('host cancellation wins when it lands during compaction or before an assistant message', () => {
  const duringCompaction = createAiSessionOutcomeTracker();
  overflowError(duringCompaction);
  observeEvent(duringCompaction, {
    type: 'compaction_end',
    reason: 'overflow',
    result: undefined,
    aborted: true,
    willRetry: false,
  });
  observeEvent(duringCompaction, { type: 'agent_settled' });
  assert.deepEqual(resolveAiSessionOutcome(duringCompaction), {
    kind: 'aborted',
  });

  const beforeFirstAssistant = createAiSessionOutcomeTracker();
  observeEvent(beforeFirstAssistant, { type: 'agent_settled' });
  assert.deepEqual(
    resolveAiSessionOutcome(beforeFirstAssistant, { abortRequested: true }),
    { kind: 'aborted' },
  );
});

test('a prompt that has not emitted its settlement event cannot report success', () => {
  const tracker = createAiSessionOutcomeTracker();
  observeEvent(tracker, {
    type: 'message_end',
    message: { role: 'assistant', stopReason: 'stop' },
  });

  assert.deepEqual(resolveAiSessionOutcome(tracker), {
    kind: 'error',
    message: 'Pi ended the request before its recovery cycle settled.',
  });
});
