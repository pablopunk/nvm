import assert from 'node:assert/strict';
import test from 'node:test';
import {
  TRIAGE_QUESTIONS,
  triageSelectedText,
  triageSummary,
} from './ai-decision-triage';

function contextFor(options: {
  selectedText?: string;
  decide?: () => Promise<unknown>;
}) {
  const decisions: unknown[] = [];
  const indicatorEvents: Array<Record<string, unknown>> = [];
  const logged: unknown[] = [];
  const context = {
    ai: options.decide
      ? {
          decide: async (input: unknown) => {
            decisions.push(input);
            return options.decide?.();
          },
        }
      : undefined,
    desktop: {
      selection: {
        text: async () => options.selectedText ?? 'Can you send the invoice?',
      },
    },
    ui: {
      indicator: {
        show: (event: Record<string, unknown>) => indicatorEvents.push(event),
        update: (event: Record<string, unknown>) => indicatorEvents.push(event),
        hide: () => indicatorEvents.push({ hidden: true }),
      },
    },
    logs: { error: (...args: unknown[]) => logged.push(args) },
  };
  return { context: context as any, decisions, indicatorEvents, logged };
}

test('summarises typed answers without requiring probabilities', () => {
  assert.equal(
    triageSummary({
      needs_reply: { type: 'predicate', value: true, probability: 0.91 },
      category: { type: 'choice', value: 'question' },
      urgency: { type: 'score', value: 4, confidence: 0.6 },
    }),
    'question · urgency 4/5 · needs reply',
  );
  assert.equal(triageSummary({}), 'No triage result');
});

test('asks the batched typed questions once over the selected text', async () => {
  const { context, decisions } = contextFor({
    decide: async () => ({
      id: 'req',
      model: 'm',
      answers: {
        needs_reply: { type: 'predicate', value: true, probability: 0.9 },
        category: { type: 'choice', value: 'question' },
        urgency: { type: 'score', value: 2 },
      },
      usage: { inputTokens: 1, outputTokens: 1 },
    }),
  });
  await triageSelectedText(context);
  assert.equal(decisions.length, 1);
  const call = decisions[0] as {
    input: string;
    questions: unknown[];
    signal: AbortSignal;
  };
  assert.equal(call.input, 'Can you send the invoice?');
  assert.deepEqual(call.questions, TRIAGE_QUESTIONS);
  assert.ok(call.signal instanceof AbortSignal);
});

test('reports a successful triage in the indicator and keeps it visible', async () => {
  const { context, indicatorEvents } = contextFor({
    decide: async () => ({
      id: 'req',
      model: 'm',
      answers: {
        category: { type: 'choice', value: 'bug report' },
      },
      usage: { inputTokens: 1, outputTokens: 1 },
    }),
  });
  await triageSelectedText(context);
  const last = indicatorEvents.at(-1) as Record<string, unknown>;
  assert.equal(last.status, 'success');
  assert.equal(last.subtitle, 'bug report');
  assert.equal(
    indicatorEvents.some((event) => event.hidden === true),
    false,
  );
});

test('surfaces decision failures instead of hiding them', async () => {
  const { context, indicatorEvents, logged } = contextFor({
    decide: async () => {
      throw Object.assign(
        new Error('The decision model is unavailable. Try again shortly.'),
        {
          code: 'decision_provider_unavailable',
        },
      );
    },
  });
  await triageSelectedText(context);
  const last = indicatorEvents.at(-1) as Record<string, unknown>;
  assert.equal(last.status, 'error');
  assert.equal(
    last.subtitle,
    'The decision model is unavailable. Try again shortly.',
  );
  assert.equal(logged.length, 1);
});

test('asks the user to select text instead of calling the model', async () => {
  const { context, decisions, indicatorEvents } = contextFor({
    selectedText: '   ',
    decide: async () => ({}),
  });
  await triageSelectedText(context);
  assert.equal(decisions.length, 0);
  assert.equal(
    (indicatorEvents.at(-1) as Record<string, unknown>).status,
    'error',
  );
});
