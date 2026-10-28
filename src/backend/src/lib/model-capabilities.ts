const REVIEWED_ROUTES: Record<
  string,
  Record<string, readonly ('smart' | 'fast')[]>
> = {
  openai: {
    'gpt-6.1-sol': ['smart'],
  },
  openrouter: {
    'google/gemini-2.5-flash': ['smart', 'fast'],
    'deepseek/deepseek-v4.1-flash-1017': ['smart', 'fast'],
    'openai/gpt-6.1-terra': ['smart', 'fast'],
    'openai/gpt-6-sol': ['fast'],
    'openai/gpt-6.1-sol': ['smart'],
  },
};

export function isReviewedModelRoute(
  provider: string,
  modelId: string,
  kind: 'smart' | 'fast',
): boolean {
  return REVIEWED_ROUTES[provider]?.[modelId]?.includes(kind) ?? false;
}

export function requiresResponsesForTools(modelId: string): boolean {
  return modelId.split('/').at(-1) === 'gpt-6.1-sol';
}

export function supportsModelThinkingLevel(
  modelId: string,
  level: string,
): boolean {
  return (
    !requiresResponsesForTools(modelId) || !['off', 'minimal'].includes(level)
  );
}

export function supportsChatCompletionsTools(
  modelId: string,
  thinkingOff: boolean,
): boolean {
  const name = modelId.split('/').at(-1) ?? '';
  if (name === 'gpt-6-astra' || name === 'gpt-6.1-sol') return false;
  if (
    name === 'gpt-6-sol' ||
    name === 'gpt-6-luna' ||
    name.startsWith('gpt-5.6-')
  )
    return thinkingOff;
  return true;
}
