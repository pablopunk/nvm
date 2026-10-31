import { Command } from 'cmdk';
import { Search } from 'lucide-react';
import { type CSSProperties, type RefObject, useRef } from 'react';
import { ActionPanel } from './action-panel';
import type { ActionPanelRow } from './ui';
import { useCompactPanelMaxHeight } from './use-compact-panel-max-height';

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
  const anchorRef = useRef<HTMLDivElement>(null);
  const maxHeight = useCompactPanelMaxHeight(anchorRef);
  return (
    <div ref={anchorRef} className="paletteCompactPanelAnchor">
      <aside
        className="extensionWindowCompactPanel paletteCompactPanel"
        style={
          { '--compact-panel-max-height': `${maxHeight}px` } as CSSProperties
        }
      >
        <div className="extensionWindowSearchRow">
          <Search size={15} className="extensionWindowSearchIcon" />
          <input
            ref={inputRef}
            autoFocus={true}
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
