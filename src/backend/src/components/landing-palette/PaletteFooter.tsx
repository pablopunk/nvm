import { ChevronsUpDown, Zap } from 'lucide-react';
import type { MouseEvent } from 'react';

export const ACTIONS_SHORTCUT = '⌘K';

function Hint({
  title,
  keycap,
  emphasized = false,
  onClick,
}: {
  title: string;
  keycap: string;
  emphasized?: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      className={emphasized ? 'hint emph' : 'hint'}
      tabIndex={-1}
      onMouseDown={keepFocus}
      onClick={onClick}
    >
      <span className="ht">{title}</span>
      <span className="keycap">{keycap}</span>
    </button>
  );
}

/** Clicks on chips, hints and menu rows act without taking focus from the field or composer, as in the app. */
export function keepFocus(event: MouseEvent) {
  event.preventDefault();
}

/** The chat page's chips: the query it was asked from, which leads back to search, and the model. */
function ChatChips({ query, onBack }: { query: string; onBack: () => void }) {
  return (
    <>
      <button
        type="button"
        className="plate name"
        title="Back to search"
        tabIndex={-1}
        onMouseDown={keepFocus}
        onClick={onBack}
      >
        <Zap className="i" aria-hidden="true" />
        <span className="ht">{query}</span>
      </button>
      <span className="plate model">
        <b>Fast</b>
        <ChevronsUpDown className="i" aria-hidden="true" />
      </span>
    </>
  );
}

/** Footer and Keycap in src/native/internal/design/footer.go: hints on a plate at the bottom right. */
export default function PaletteFooter({
  primary,
  chat,
  onPrimary,
  onActions,
}: {
  primary: string | null;
  chat?: { query: string; onBack: () => void };
  onPrimary: () => void;
  onActions: () => void;
}) {
  return (
    <div className="footer">
      {chat ? <ChatChips query={chat.query} onBack={chat.onBack} /> : null}
      <div className="grow" />
      <div className="plate hints">
        {primary ? (
          <Hint title={primary} keycap="↵" emphasized={Boolean(chat)} onClick={onPrimary} />
        ) : null}
        <Hint title="Actions" keycap={ACTIONS_SHORTCUT} onClick={onActions} />
      </div>
    </div>
  );
}
