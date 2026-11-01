import { type RefObject, useEffect, useRef } from 'react';
import type { CommandView } from './model';
import type { PaletteMode } from './preload-api';

interface ViewModeInput {
  view: CommandView | null;
  siblings: CommandView[];
  hasPreview: boolean;
  previewCount: number;
  chatOpen: RefObject<boolean>;
  chatId: RefObject<string | undefined>;
}

function paletteMode({
  view,
  hasPreview,
  previewCount,
  hasChat,
  hasSiblings,
}: {
  view: CommandView | null;
  hasPreview: boolean;
  previewCount: number;
  hasChat: boolean;
  hasSiblings: boolean;
}): PaletteMode {
  if (hasPreview) {
    return 'preview';
  }
  if (view?.presentation === 'side-preview') {
    return 'side-preview';
  }
  if (
    view?.size === 'large' ||
    view?.presentation === 'preview' ||
    (hasChat && previewCount > 0)
  ) {
    return 'preview';
  }
  if (hasSiblings) {
    return 'stacked';
  }
  return hasChat ? 'ai-chat' : 'default';
}

export function usePaletteViewMode({
  view,
  siblings,
  hasPreview,
  previewCount,
  chatOpen,
  chatId,
}: ViewModeInput) {
  const previous = useRef<PaletteMode | null>(null);
  useEffect(() => {
    const chat = view?.aiChat
      ? view
      : [...siblings].reverse().find((item) => item.aiChat);
    chatOpen.current = Boolean(chat);
    chatId.current = chat?.chatId;
    const mode = paletteMode({
      view,
      hasPreview,
      previewCount,
      hasChat: Boolean(chat),
      hasSiblings: siblings.length > 0,
    });
    if (previous.current !== mode) {
      previous.current = mode;
      window.nvm.setPaletteMode(mode);
    }
  }, [view, siblings, hasPreview, previewCount, chatOpen, chatId]);
}
