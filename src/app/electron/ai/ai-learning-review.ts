// biome-ignore-all lint: AI learning reviews parse dynamic model output and chat metadata.
import { warn as logWarn } from '../platform/logger';
import type { JobRegistry } from '../platform/jobs';
import type { LearningKind, LocalLearningStore } from '../state/learning-store';

export function normalizedLearningReview(response: string) {
  const raw = String(response || '').trim();
  if (!raw) return [];
  const fencedMatch = raw.match(/```(?:json)?\s*([\s\S]*?)```/i);
  const candidate = (fencedMatch?.[1] || raw).trim();
  const parsed = JSON.parse(candidate) as {
    learnings?: Array<{
      kind?: string;
      summary?: string;
      appliesWhen?: string;
      keywords?: string[];
      confidence?: string;
      evidence?: string;
    }>;
  };
  return (parsed.learnings || [])
    .map((learning) => ({
      kind:
        learning.kind === 'workflow' || learning.kind === 'preference'
          ? learning.kind
          : 'environment',
      summary: String(learning.summary || '').trim(),
      appliesWhen: learning.appliesWhen
        ? String(learning.appliesWhen).trim()
        : undefined,
      keywords: Array.isArray(learning.keywords)
        ? learning.keywords.map(String)
        : [],
      confidence:
        learning.confidence === 'low' || learning.confidence === 'high'
          ? learning.confidence
          : 'medium',
      evidence: learning.evidence
        ? String(learning.evidence).trim()
        : undefined,
    }))
    .filter((learning) => learning.summary);
}

export function learningReviewPrompt(snapshot: any) {
  return `You are curating the canonical user learning rules for Nevermind's extension-building AI on this machine.

Analyze the conversation and trace for friction patterns, including misunderstandings, too many iterations, repeated back-and-forth, retries, environment quirks, tool misuse, or anything else that would help future extension-building chats go better.

Your job is to update the current learning set, not append blindly. Merge overlapping rules, rewrite weak rules into stronger generic ones, remove stale or redundant rules, and keep the final set small. Prefer a tiny set of generic durable rules over many specific ones.

Generic means durable principles for future extension builds, not exact API/class workarounds, one extension's domain model, one bug's workaround, or transient host/client behavior. If a finding only applies to the current extension or technology, lift it to a broader principle or omit it.

Do not include product feedback about improving the extension API or builder prompt. Do not include volatile facts like absolute file paths, specific chat titles, or one-off task details unless they are stable machine conventions.

Return the full resulting rule set as strict JSON only with this shape:
{"learnings":[{"kind":"environment|workflow|preference","summary":"short generic rule","appliesWhen":"optional short trigger","keywords":["keyword"],"confidence":"low|medium|high","evidence":"brief rationale"}]}

Keep the final list as small as possible. Returning {"learnings":[]} is valid when no durable user learnings should exist.

Trace:
${JSON.stringify(snapshot, null, 2)}`;
}

export type AiLearningReviewDeps = {
  getLearningStore: () => LocalLearningStore | null;
  aiLearningMetadata: (chatId: string) => any;
  jobRegistry: Pick<JobRegistry, 'snapshot' | 'register' | 'run'>;
  getNevermindAi: () => any;
};

export function createAiLearningReview(deps: AiLearningReviewDeps) {
  const { getLearningStore, aiLearningMetadata, jobRegistry, getNevermindAi } =
    deps;

  const learningReviewJobs = new Map<string, Promise<void>>();

  function relevantLearningContext(message: string, chatId: string) {
    const learningStore = getLearningStore();
    if (!learningStore) return '';
    const metadata = aiLearningMetadata(chatId);
    const learnings = learningStore.relevantLearnings({
      message,
      query: metadata.query,
      contextExtensionFile: metadata.contextExtensionFile,
      limit: 4,
    });
    if (!learnings.length) return '';
    const lines = learnings.map(
      (learning) =>
        `- ${learning.summary}${learning.appliesWhen ? ` When relevant: ${learning.appliesWhen}.` : ''}`,
    );
    return `\n\nLocal machine learnings for future extension builds:\n${lines.join('\n')}`;
  }

  function recordLearningReview(chatId: string) {
    const jobId = `ai.learning.review.${chatId}`;
    const learningStore = getLearningStore();
    if (
      !learningStore?.shouldReview(chatId) ||
      learningReviewJobs.has(chatId) ||
      jobRegistry.snapshot().some((job) => job.id === jobId && job.running) ||
      !getNevermindAi()?.ask
    )
      return;
    const snapshot = learningStore.reviewSnapshot(chatId);
    if (!snapshot) return;
    jobRegistry.register({
      id: jobId,
      title: `AI Learning Review: ${chatId}`,
      owner: 'host',
      scope: 'ai',
      timeoutMs: 60_000,
      run: async () => {
        try {
          const response = await getNevermindAi().ask(
            learningReviewPrompt(snapshot),
            {
              system:
                'You maintain a tiny canonical set of generic user learnings for future Nevermind extension-building chats. Merge and rewrite rules instead of appending; omit one-off implementation details. Return strict JSON and keep the final set minimal.',
            },
          );
          const learnings = normalizedLearningReview(response);
          getLearningStore()?.replaceLearningsFromReview(
            chatId,
            learnings as Array<{
              kind: LearningKind;
              summary: string;
              appliesWhen?: string;
              keywords?: string[];
              confidence?: 'low' | 'medium' | 'high';
              evidence?: string;
            }>,
          );
        } catch (error) {
          logWarn(
            'ai.learning.review.failed',
            {
              chatId,
              error: error instanceof Error ? error.message : String(error),
            },
            { source: 'host', scope: 'ai' },
          );
          throw error;
        } finally {
          learningReviewJobs.delete(chatId);
        }
      },
    });
    const job = jobRegistry
      .run(jobId, 'ai-chat-exited')
      .then(() => undefined)
      .catch(() => undefined);
    learningReviewJobs.set(chatId, job);
  }

  return { relevantLearningContext, recordLearningReview };
}
