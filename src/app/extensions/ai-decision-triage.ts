import type {
  ExtensionContext,
  ExtensionDecisionAnswer,
  ExtensionDecisionQuestion,
  ExtensionIndicatorInput,
} from '../resources/nevermind-extension-api';

const TRIAGE_INDICATOR_ID = 'triage-selected-text-with-ai';
const TRIAGE_TIMEOUT_MS = 30_000;

export const TRIAGE_QUESTIONS: ExtensionDecisionQuestion[] = [
  {
    id: 'needs_reply',
    type: 'predicate',
    question: 'Does the text ask the reader for a reply or an action?',
  },
  {
    id: 'category',
    type: 'choice',
    question: 'Which category best fits the text?',
    options: ['bug report', 'feature request', 'question', 'other'],
  },
  {
    id: 'urgency',
    type: 'score',
    question: 'How urgent is the text?',
    min: 1,
    max: 5,
    labels: { '1': 'no rush', '5': 'drop everything' },
  },
];

function indicator(subtitle: string): ExtensionIndicatorInput {
  return {
    id: TRIAGE_INDICATOR_ID,
    title: 'Triage Selected Text with AI',
    subtitle,
    status: 'loading',
  };
}

function triageParts(answers: Record<string, ExtensionDecisionAnswer>) {
  const parts: string[] = [];
  const category = answers.category;
  const urgency = answers.urgency;
  const needsReply = answers.needs_reply;
  if (category?.type === 'choice') parts.push(category.value);
  if (urgency?.type === 'score') parts.push(`urgency ${urgency.value}/5`);
  if (needsReply?.type === 'predicate')
    parts.push(needsReply.value ? 'needs reply' : 'no reply needed');
  return parts;
}

export function triageSummary(
  answers: Record<string, ExtensionDecisionAnswer>,
): string {
  return triageParts(answers).join(' · ') || 'No triage result';
}

export async function triageSelectedText(ctx: ExtensionContext) {
  ctx.ui.indicator.show(indicator('Reading Selection'));
  let keepFinalIndicatorVisible = false;
  try {
    if (!ctx.ai) throw new Error('AI is unavailable');
    const ai = ctx.ai;
    const selectedText = String((await ctx.desktop.selection.text()) ?? '');
    if (!selectedText.trim()) {
      keepFinalIndicatorVisible = true;
      ctx.ui.indicator.update({
        ...indicator('Could not read selected text. Select it and try again'),
        status: 'error',
        durationMs: 4000,
      });
      return;
    }
    ctx.ui.indicator.update(indicator('Deciding'));
    const result = await ai.decide({
      input: selectedText,
      questions: TRIAGE_QUESTIONS,
      signal: AbortSignal.timeout(TRIAGE_TIMEOUT_MS),
    });
    keepFinalIndicatorVisible = true;
    ctx.ui.indicator.update({
      ...indicator(triageSummary(result.answers)),
      status: 'success',
      durationMs: 6000,
    });
  } catch (error) {
    ctx.logs.error('Triage selected text failed', error);
    keepFinalIndicatorVisible = true;
    ctx.ui.indicator.update({
      ...indicator(
        error instanceof Error ? error.message : 'Could not triage text',
      ),
      status: 'error',
      durationMs: 4000,
    });
  } finally {
    if (!keepFinalIndicatorVisible) ctx.ui.indicator.hide(TRIAGE_INDICATOR_ID);
  }
}
