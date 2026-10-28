export type PredicateAnswer = {
  type: 'predicate';
  value: boolean;
  probability: number;
};

export type ChoiceAnswer = {
  type: 'choice';
  value: string;
  probabilities?: Record<string, number>;
  confidence?: number;
};

export type ScoreAnswer = {
  type: 'score';
  value: number;
  probabilities?: Record<string, number>;
  confidence?: number;
};

export type DecisionAnswer = PredicateAnswer | ChoiceAnswer | ScoreAnswer;

export type DecisionUsage = {
  inputTokens: number;
  outputTokens: number;
};

export type ParsedDecisionResponse = {
  answers: Record<string, DecisionAnswer>;
  usage: DecisionUsage;
  providerCostUsd?: number;
  providerModel?: string;
  providerResponseId?: string;
};

export class DecisionProviderResponseError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'DecisionProviderResponseError';
  }
}

export function isProbability(value: unknown): value is number {
  return typeof value === 'number' && value >= 0 && value <= 1;
}

export function isNonNegativeInteger(value: unknown): value is number {
  return Number.isInteger(value) && (value as number) >= 0;
}
