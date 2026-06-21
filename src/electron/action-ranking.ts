interface RankedAction {
  id?: unknown;
  kind?: unknown;
  extensionId?: unknown;
  score: number;
  lastUsed: number;
  title?: unknown;
}

const AI_BUILDER_CHAT_TIE_PRIORITY = -1;
const DEFAULT_TIE_PRIORITY = 0;

export function rankedActionTiePriority(action: RankedAction): number {
  if (
    action.kind === 'extension-root-item' &&
    action.extensionId === 'nevermind.ai-builder' &&
    String(action.id || '').startsWith(
      'extension-root:nevermind.ai-builder:ai-chat:',
    )
  ) {
    return AI_BUILDER_CHAT_TIE_PRIORITY;
  }
  return DEFAULT_TIE_PRIORITY;
}

export function compareRankedActions(a: RankedAction, b: RankedAction): number {
  return (
    b.score - a.score ||
    rankedActionTiePriority(b) - rankedActionTiePriority(a) ||
    b.lastUsed - a.lastUsed ||
    String(a.title || '').localeCompare(String(b.title || ''))
  );
}
