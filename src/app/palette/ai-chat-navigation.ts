import type { CommandView } from './model';

export function restoredAiChatView(
  backStack: CommandView[],
  siblingViews: CommandView[],
) {
  const restoredView =
    backStack.length > 0 ? backStack.at(-1) : siblingViews.at(-1);
  return restoredView?.aiChat ? restoredView : null;
}
