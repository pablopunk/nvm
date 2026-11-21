import { type KeyboardEvent, type RefObject, useEffect } from 'react';
import { RELEASES_URL } from './palette-data';
import { DOWNLOAD_NOTICE_TEXT } from './PaletteRow';

export type ChatMode = 'chat' | 'builder';

const PAGES: Record<ChatMode, { reply: string; placeholder: string }> = {
  chat: {
    reply: 'In the app, Tab sends anything you type to AI, and the answer streams in right here.',
    placeholder: 'Message AI',
  },
  builder: {
    reply:
      'In the app, the AI builder writes this as an extension and installs it, so it becomes a command you can find here.',
    placeholder: 'Describe the extension you want',
  },
};

/**
 * The AI chat and builder page (src/native/internal/palette/aichat): the query as a user bubble, the reply, and the
 * composer above the footer. The landing page has no model, so the reply points to the app.
 */
export default function ChatPage({
  mode,
  query,
  showNotice,
  composerRef,
  onSend,
  onBack,
}: {
  mode: ChatMode;
  query: string;
  showNotice: boolean;
  composerRef: RefObject<HTMLTextAreaElement | null>;
  onSend: () => void;
  onBack: () => void;
}) {
  const page = PAGES[mode];
  useEffect(
    function focusComposer() {
      composerRef.current?.focus({ preventScroll: true });
    },
    [composerRef],
  );

  function handleComposerKey(event: KeyboardEvent<HTMLTextAreaElement>) {
    if (event.nativeEvent.isComposing) return;
    if (event.key === 'Enter' && !event.shiftKey) {
      event.preventDefault();
      onSend();
    } else if (event.key === 'Escape') {
      event.preventDefault();
      onBack();
    }
  }

  return (
    <div className="chatPage">
      <div className="chat">
        <div className="bubble">{query}</div>
        <p className="answer">{page.reply}</p>
        <p className="answer">
          <a href={RELEASES_URL}>{showNotice ? DOWNLOAD_NOTICE_TEXT : 'Download Nevermind'}</a>
        </p>
      </div>
      <textarea
        ref={composerRef}
        className="composer"
        placeholder={page.placeholder}
        aria-label={page.placeholder}
        rows={3}
        spellCheck={false}
        onKeyDown={handleComposerKey}
      />
    </div>
  );
}
