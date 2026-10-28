import {
  type DecisionAnswer,
  DecisionProviderResponseError,
  isNonNegativeInteger,
  isProbability,
  type ParsedDecisionResponse,
} from './decision-contract';
import type { DecisionQuestion, DecisionRequest } from './decision-request';

type OpenRouterQuestion =
  | { type: 'noul'; instructions: string }
  | { type: 'choice'; instructions: string; criteria: Record<string, string> }
  | { type: 'score'; instructions: string; criteria: string[] };

type OpenRouterAnswerRecord = Record<string, unknown>;

function scoreLevels(question: Extract<DecisionQuestion, { type: 'score' }>) {
  return Array.from(
    { length: question.max - question.min + 1 },
    (_, index) => question.min + index,
  );
}

function scoreLevelCriterion(
  question: Extract<DecisionQuestion, { type: 'score' }>,
  level: number,
): string {
  const label = question.labels?.[String(level)];
  return label ? `${level} = ${label}` : String(level);
}

export function openRouterQuestion(
  question: DecisionQuestion,
): OpenRouterQuestion {
  if (question.type === 'predicate')
    return { type: 'noul', instructions: question.question };
  if (question.type === 'choice')
    return {
      type: 'choice',
      instructions: question.question,
      criteria: Object.fromEntries(
        question.options.map((option) => [option, option]),
      ),
    };
  return {
    type: 'score',
    instructions: question.question,
    criteria: scoreLevels(question).map((level) =>
      scoreLevelCriterion(question, level),
    ),
  };
}

function imageStatePart(image: DecisionRequest['images'][number]) {
  return {
    type: 'image_url',
    image_url: {
      url: `data:${image.mediaType};base64,${image.data}`,
      detail: 'auto',
    },
  };
}

export function openRouterState(request: DecisionRequest) {
  if (request.images.length === 0) return request.input;
  return [
    ...(request.input ? [request.input] : []),
    ...request.images.map(imageStatePart),
  ];
}

export function buildOpenRouterDecisionBody(
  request: DecisionRequest,
  modelId: string,
) {
  return {
    model: modelId,
    state: openRouterState(request),
    questions: Object.fromEntries(
      request.questions.map((question) => [
        question.id,
        openRouterQuestion(question),
      ]),
    ),
    provider: { data_collection: 'deny' },
  };
}

export function openRouterDecisionsUrl(baseUrl: string): string {
  return `${new URL(baseUrl).origin}/api/alpha/decisions`;
}

function requireRecord(value: unknown, label: string): OpenRouterAnswerRecord {
  if (value && typeof value === 'object' && !Array.isArray(value))
    return value as OpenRouterAnswerRecord;
  throw new DecisionProviderResponseError(`${label} is not an object`);
}

function probabilityMap(
  value: unknown,
  allowedKeys: readonly string[],
  label: string,
): Record<string, number> | undefined {
  if (value === undefined) return;
  const record = requireRecord(value, label);
  for (const [key, probability] of Object.entries(record)) {
    if (!(allowedKeys.includes(key) && isProbability(probability)))
      throw new DecisionProviderResponseError(
        `${label} has an invalid entry ${key}`,
      );
  }
  return record as Record<string, number>;
}

function optionalConfidence(value: unknown, label: string): number | undefined {
  if (value === undefined) return;
  if (!isProbability(value))
    throw new DecisionProviderResponseError(`${label} confidence is invalid`);
  return value;
}

function predicateAnswer(
  raw: OpenRouterAnswerRecord,
  label: string,
): DecisionAnswer {
  if (!isProbability(raw.noul))
    throw new DecisionProviderResponseError(`${label} predicate is invalid`);
  return { type: 'predicate', value: raw.noul >= 0.5, probability: raw.noul };
}

function choiceAnswer(
  raw: OpenRouterAnswerRecord,
  question: Extract<DecisionQuestion, { type: 'choice' }>,
  label: string,
): DecisionAnswer {
  if (typeof raw.choice !== 'string' || !question.options.includes(raw.choice))
    throw new DecisionProviderResponseError(`${label} choice is not an option`);
  return {
    type: 'choice',
    value: raw.choice,
    probabilities: probabilityMap(raw.probabilities, question.options, label),
    confidence: optionalConfidence(raw.confidence, label),
  };
}

function scoreAnswer(
  raw: OpenRouterAnswerRecord,
  question: Extract<DecisionQuestion, { type: 'score' }>,
  label: string,
): DecisionAnswer {
  const levels = scoreLevels(question);
  if (
    typeof raw.score !== 'number' ||
    !Number.isFinite(raw.score) ||
    raw.score < 0 ||
    raw.score > levels.length - 1
  )
    throw new DecisionProviderResponseError(
      `${label} score is outside its levels`,
    );
  return {
    type: 'score',
    value: raw.score + question.min,
    probabilities:
      raw.probabilities === undefined
        ? undefined
        : Object.fromEntries(
            Object.entries(
              probabilityMap(
                raw.probabilities,
                levels.map((_, index) => String(index)),
                label,
              ) ?? {},
            ).map(([index, probability]) => [
              String(Number(index) + question.min),
              probability,
            ]),
          ),
    confidence: optionalConfidence(raw.confidence, label),
  };
}

function answerFor(
  question: DecisionQuestion,
  raw: OpenRouterAnswerRecord,
): DecisionAnswer {
  const label = `answer ${question.id}`;
  const expectedType = question.type === 'predicate' ? 'noul' : question.type;
  if (raw.type !== expectedType)
    throw new DecisionProviderResponseError(`${label} has an unexpected type`);
  if (question.type === 'predicate') return predicateAnswer(raw, label);
  if (question.type === 'choice') return choiceAnswer(raw, question, label);
  return scoreAnswer(raw, question, label);
}

function parseUsage(value: unknown) {
  const usage = requireRecord(value, 'usage');
  if (
    !(
      isNonNegativeInteger(usage.input_tokens) &&
      isNonNegativeInteger(usage.output_tokens)
    )
  )
    throw new DecisionProviderResponseError('usage token counts are invalid');
  const cost = usage.cost;
  if (
    cost !== undefined &&
    (typeof cost !== 'number' || !Number.isFinite(cost) || cost < 0)
  )
    throw new DecisionProviderResponseError('usage cost is invalid');
  return {
    usage: {
      inputTokens: usage.input_tokens,
      outputTokens: usage.output_tokens,
    },
    providerCostUsd: cost as number | undefined,
  };
}

export function parseOpenRouterDecisionResponse(
  body: unknown,
  request: DecisionRequest,
): ParsedDecisionResponse {
  const response = requireRecord(body, 'response');
  const rawAnswers = requireRecord(response.answers, 'answers');
  const expectedIds = request.questions.map((question) => question.id);
  const unexpected = Object.keys(rawAnswers).filter(
    (id) => !expectedIds.includes(id),
  );
  if (unexpected.length > 0)
    throw new DecisionProviderResponseError(
      `answers contain unrequested ids: ${unexpected.join(', ')}`,
    );
  const answers: Record<string, DecisionAnswer> = {};
  for (const question of request.questions) {
    if (!(question.id in rawAnswers))
      throw new DecisionProviderResponseError(
        `answer ${question.id} is missing`,
      );
    answers[question.id] = answerFor(
      question,
      requireRecord(rawAnswers[question.id], `answer ${question.id}`),
    );
  }
  const { usage, providerCostUsd } = parseUsage(response.usage);
  return {
    answers,
    usage,
    providerCostUsd,
    providerModel:
      typeof response.model === 'string' ? response.model : undefined,
    providerResponseId:
      typeof response.id === 'string' ? response.id : undefined,
  };
}
