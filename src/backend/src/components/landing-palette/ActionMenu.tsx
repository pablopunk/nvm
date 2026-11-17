import { Command } from 'cmdk';
import { CornerDownLeft, Search } from 'lucide-react';
import { type KeyboardEvent, useEffect, useRef } from 'react';
import type { MenuAction } from './palette-actions';
import { keepFocus } from './PaletteFooter';

/**
 * The ⌘K action menu: ActionMenu and Popover in src/native/internal/design, a raised card above the footer with
 * its own search field. Escape or ⌘K closes it and gives the keyboard back to the page under it.
 */
export default function ActionMenu({
  actions,
  onRun,
  onClose,
}: {
  actions: MenuAction[];
  onRun: (action: MenuAction) => void;
  /** Closes the menu; refocus gives the keyboard back to the page, which a click elsewhere keeps for itself. */
  onClose: (refocus: boolean) => void;
}) {
  const inputRef = useRef<HTMLInputElement>(null);
  useEffect(function focusMenuSearch() {
    inputRef.current?.focus({ preventScroll: true });
  }, []);

  function closeOnEscape(event: KeyboardEvent<HTMLElement>) {
    if (event.key !== 'Escape') return;
    event.preventDefault();
    event.stopPropagation();
    onClose(true);
  }

  return (
    <Command className="menu" label="Actions" loop onKeyDown={closeOnEscape}>
      <div className="menuSearch">
        <Search className="i" aria-hidden="true" />
        <Command.Input
          ref={inputRef}
          placeholder="Search actions…"
          aria-label="Search actions"
          spellCheck={false}
          autoComplete="off"
          onBlur={() => onClose(false)}
        />
      </div>
      <div className="etched" />
      <Command.List className="menuList">
        <Command.Empty className="menuEmpty">No matching actions</Command.Empty>
        {actions.map(function renderAction(action) {
          return (
            <Command.Item
              key={action.id}
              value={action.title}
              className="menuRow"
              onMouseDown={keepFocus}
              onSelect={() => onRun(action)}
            >
              <span className="mt">{action.title}</span>
              {action.primary ? (
                <CornerDownLeft className="i" aria-label="Enter" />
              ) : action.shortcut ? (
                <span className="mk">{action.shortcut}</span>
              ) : null}
            </Command.Item>
          );
        })}
      </Command.List>
    </Command>
  );
}

