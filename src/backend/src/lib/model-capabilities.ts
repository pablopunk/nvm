export function requiresResponsesForTools(modelId: string): boolean {
  return modelId.split('/').at(-1) === 'gpt-6.1-sol';
}

export function supportsModelThinkingLevel(modelId: string, level: string): boolean {
  return !requiresResponsesForTools(modelId) || !['off', 'minimal'].includes(level);
}
