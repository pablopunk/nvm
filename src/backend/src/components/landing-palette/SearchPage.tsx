import { Command } from 'cmdk';
import { Sparkles, Zap } from 'lucide-react';
import type { KeyboardEvent, RefObject } from 'react';
import { requestLightningStrike } from '../lightning/strike-event';
import { ROOT_PLACEHOLDER } from './palette-data';
import type { PaletteItem } from './palette-types';
import { keepFocus } from './PaletteFooter';
import { PaletteRow } from './PaletteRow';

function strikeOnEnter(event: KeyboardEvent<HTMLElement>) {
  if (event.key !== 'Enter' || event.nativeEvent.isComposing) return;
  const selected = event.currentTarget.querySelector('[cmdk-item][data-selected="true"]');
  const rect = (selected ?? event.currentTarget).getBoundingClientRect();
  requestLightningStrike(rect.left + rect.width / 2, rect.top + rect.height / 2);
}

/** The "Tab to ask AI" chip of SearchRow (src/native/internal/design/chip.go). */
function AskAiChip({ onAskAi }: { onAskAi: () => void }) {
  return (
    <button
      type="button"
      className="chip"
      tabIndex={-1}
      onMouseDown={keepFocus}
      onClick={onAskAi}
    >
      <Sparkles className="i" aria-hidden="true" />
      <b>Tab</b>to ask AI
    </button>
  );
}

/** The search page: SearchRow with the gold bolt, then the result rows. */
export default function SearchPage({
  query,
  rows,
  selectedId,
  noticeTarget,
  inputRef,
  onQueryChange,
  onSelectedChange,
  onRun,
  onAskAi,
}: {
  query: string;
  rows: PaletteItem[];
  selectedId: string | undefined;
  noticeTarget: string | null;
  inputRef: RefObject<HTMLInputElement | null>;
  onQueryChange: (value: string) => void;
  onSelectedChange: (id: string) => void;
  onRun: (item: PaletteItem) => void;
  onAskAi: () => void;
}) {
  function handleKeyDown(event: KeyboardEvent<HTMLElement>) {
    strikeOnEnter(event);
    if (event.key === 'Tab' && !event.shiftKey && query.trim()) {
      event.preventDefault();
      onAskAi();
    }
  }
  return (
    <Command
      className="searchPage"
      label="Nevermind"
      loop
      shouldFilter={false}
      value={selectedId ?? ''}
      onValueChange={onSelectedChange}
      onKeyDown={handleKeyDown}
    >
      <div className="search">
        <Zap className="i zap" aria-hidden="true" />
        <div className="field">
          <Command.Input
            ref={inputRef}
            value={query}
            onValueChange={onQueryChange}
            placeholder={ROOT_PLACEHOLDER}
            spellCheck={false}
            autoComplete="off"
          />
        </div>
        {query.trim() ? <AskAiChip onAskAi={onAskAi} /> : null}
      </div>
      <Command.List className="list" label="Results">
        {rows.map(function renderRow(item) {
          return (
            <PaletteRow
              key={item.id}
              item={item}
              selected={item.id === selectedId}
              showNotice={item.id === noticeTarget}
              onSelect={() => onRun(item)}
            />
          );
        })}
      </Command.List>
    </Command>
  );
}
