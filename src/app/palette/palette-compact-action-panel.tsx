import { Command } from 'cmdk';
import { Search } from 'lucide-react';
import type { RefObject } from 'react';
import { ActionPanel } from './action-panel';
import type { ActionPanelRow } from './ui';

export function PaletteCompactActionPanel({
  inputRef,
  query,
  onQueryChange,
  placeholder,
  rows,
}: {
  inputRef: RefObject<HTMLInputElement | null>;
  query: string;
  onQueryChange: (query: string) => void;
  placeholder: string;
  rows: ActionPanelRow[];
}) {
  return (
    <div className="paletteCompactPanelAnchor">
      <aside className="extensionWindowCompactPanel paletteCompactPanel">
        <div className="extensionWindowSearchRow">
          <Search size={15} className="extensionWindowSearchIcon" />
          <input
            ref={inputRef}
            autoFocus
            value={query}
            onChange={(event) => onQueryChange(event.currentTarget.value)}
            placeholder={placeholder}
            aria-label={placeholder}
            spellCheck={false}
          />
        </div>
        <Command.List className="extensionWindowCompactResults">
          <ActionPanel rows={rows} emptyMessage="No matching actions" />
        </Command.List>
      </aside>
    </div>
  );
}
