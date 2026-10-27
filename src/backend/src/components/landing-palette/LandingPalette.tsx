import { Command } from 'cmdk';
import {
  Activity,
  Calculator,
  Calendar,
  Clipboard,
  CircleStop,
  FileCode,
  FileText,
  Folder,
  FolderOpen,
  History,
  Image,
  Images,
  Keyboard,
  Link,
  List,
  Lock,
  Mic,
  Moon,
  NotebookPen,
  Palette,
  Power,
  RotateCcw,
  Search,
  Settings,
  SmilePlus,
  Sparkles,
  SquarePen,
  StickyNote,
  Terminal,
  Type,
  Zap,
  type LucideIcon,
} from 'lucide-react';
import {
  type KeyboardEvent,
  type ReactNode,
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from 'react';
import { createPortal } from 'react-dom';
import { requestLightningStrike } from '../lightning/strike-event';
import { paletteItems, paletteTabs } from './palette-data';
import { searchPalette } from './palette-search';
import { landingPaletteCss } from './palette-styles';
import type { PaletteItem } from './palette-types';

const ICONS: Record<string, LucideIcon> = {
  activity: Activity,
  calculator: Calculator,
  calendar: Calendar,
  clipboard: Clipboard,
  'file-code': FileCode,
  'file-text': FileText,
  folder: Folder,
  'folder-open': FolderOpen,
  history: History,
  image: Image,
  images: Images,
  keyboard: Keyboard,
  link: Link,
  list: List,
  lock: Lock,
  mic: Mic,
  moon: Moon,
  'notebook-pen': NotebookPen,
  palette: Palette,
  power: Power,
  restart: RotateCcw,
  settings: Settings,
  'smile-plus': SmilePlus,
  sparkles: Sparkles,
  'square-pen': SquarePen,
  'sticky-note': StickyNote,
  'stop-circle': CircleStop,
  terminal: Terminal,
  type: Type,
};

const TYPING_INTERVAL_MS = 55;
const DOWNLOAD_NOTICE_MS = 4000;
const DOWNLOAD_NOTICE_TEXT = 'Download the app to actually do that!';
const RELEASES_URL = 'https://github.com/pablopunk/nvm/releases';

function iconFor(item: PaletteItem): ReactNode {
  const Icon = (item.icon && ICONS[item.icon]) || Sparkles;
  return <Icon size={18} />;
}

function ResultRow({
  item,
  showNotice,
  onSelect,
}: {
  item: PaletteItem;
  showNotice: boolean;
  onSelect: () => void;
}) {
  return (
    <Command.Item value={item.id} className="result" onSelect={onSelect}>
      <span className="resultIcon">{iconFor(item)}</span>
      <span className="resultText">
        <strong>{item.title}</strong>
        {showNotice ? (
          <small className="downloadNotice" role="status">
            <a href={RELEASES_URL}>{DOWNLOAD_NOTICE_TEXT}</a>
          </small>
        ) : (
          <small>{item.subtitle}</small>
        )}
      </span>
      <span className="resultTrailing">
        <span className="keyHints">
          <span className="enterHint selectedOnlyEnter" aria-label="Enter">
            <span aria-hidden="true">↵</span>
          </span>
        </span>
      </span>
    </Command.Item>
  );
}

function strikeOnEnter(event: KeyboardEvent<HTMLElement>) {
  if (event.key !== 'Enter' || event.nativeEvent.isComposing) return;
  const selected = event.currentTarget.querySelector('[cmdk-item][data-selected="true"]');
  const rect = (selected ?? event.currentTarget).getBoundingClientRect();
  requestLightningStrike(rect.left + rect.width / 2, rect.top + rect.height / 2);
}

function PaletteApp({
  query,
  onQueryChange,
  noticeItemId,
  onSelectItem,
  inputRef,
}: {
  query: string;
  onQueryChange: (value: string) => void;
  noticeItemId: string | null;
  onSelectItem: (item: PaletteItem) => void;
  inputRef: React.RefObject<HTMLInputElement | null>;
}) {
  const results = useMemo(() => searchPalette(paletteItems, query), [query]);
  return (
    <Command
      className="palette fitContent"
      label="Nevermind"
      loop
      shouldFilter={false}
      onKeyDown={strikeOnEnter}
    >
      <div className="searchRow card searchCard">
        <Zap className="brandIcon" size={22} />
        <Command.Input
          ref={inputRef}
          value={query}
          onValueChange={onQueryChange}
          placeholder="Search anything..."
          spellCheck={false}
          autoComplete="off"
        />
        {query ? (
          <div className="tabHint">
            <Sparkles size={12} aria-hidden="true" /> <kbd>Tab</kbd> to ask AI
          </div>
        ) : null}
      </div>
      <Command.List className="results card resultsCard">
        {results.length === 0 ? (
          <div className="empty" role="status">
            <Search size={24} />
            <strong>No results</strong>
            <span>In the app, Tab asks AI to build it for you.</span>
          </div>
        ) : (
          results.map((item) => (
            <ResultRow
              key={item.id}
              item={item}
              showNotice={item.id === noticeItemId}
              onSelect={() => onSelectItem(item)}
            />
          ))
        )}
      </Command.List>
    </Command>
  );
}

export default function LandingPalette() {
  const hostRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const typingTimer = useRef<ReturnType<typeof setInterval> | null>(null);
  const [mount, setMount] = useState<HTMLElement | null>(null);
  const [query, setQuery] = useState('');
  const [activeTab, setActiveTab] = useState<string | null>(paletteTabs[0].id);
  const [noticeItemId, setNoticeItemId] = useState<string | null>(null);
  const noticeTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  useLayoutEffect(
    function attachShadowRoot() {
      const host = hostRef.current;
      if (!host) return;
      const shadow = host.shadowRoot ?? host.attachShadow({ mode: 'open' });
      shadow.replaceChildren();
      const style = document.createElement('style');
      style.textContent = landingPaletteCss;
      const container = document.createElement('div');
      shadow.append(style, container);
      setMount(container);
    },
    [],
  );

  const stopTyping = useCallback(function stopTyping() {
    if (typingTimer.current) clearInterval(typingTimer.current);
    typingTimer.current = null;
  }, []);
  useEffect(() => stopTyping, [stopTyping]);

  const hideNotice = useCallback(function hideNotice() {
    if (noticeTimer.current) clearTimeout(noticeTimer.current);
    noticeTimer.current = null;
    setNoticeItemId(null);
  }, []);
  useEffect(() => hideNotice, [hideNotice]);

  function showNotice(itemId: string) {
    if (noticeTimer.current) clearTimeout(noticeTimer.current);
    setNoticeItemId(itemId);
    noticeTimer.current = setTimeout(hideNotice, DOWNLOAD_NOTICE_MS);
  }

  const typeInto = useCallback(
    function typeInto(text: string) {
      stopTyping();
      setQuery('');
      if (!text) return;
      let length = 0;
      typingTimer.current = setInterval(function typeNextCharacter() {
        length += 1;
        setQuery(text.slice(0, length));
        if (length >= text.length) stopTyping();
      }, TYPING_INTERVAL_MS);
    },
    [stopTyping],
  );

  function chooseTab(tabId: string, tabQuery: string) {
    setActiveTab(tabId);
    hideNotice();
    typeInto(tabQuery);
    inputRef.current?.focus({ preventScroll: true });
  }

  function typeFreely(value: string) {
    stopTyping();
    setActiveTab(null);
    hideNotice();
    setQuery(value);
  }

  function selectItem(item: PaletteItem) {
    showNotice(item.id);
  }

  return (
    <div className="landing-palette">
      <div ref={hostRef} className="landing-palette__host" />
      {mount
        ? createPortal(
            <PaletteApp
              query={query}
              onQueryChange={typeFreely}
              noticeItemId={noticeItemId}
              onSelectItem={selectItem}
              inputRef={inputRef}
            />,
            mount,
          )
        : null}
      <div className="landing-palette__tabs" role="group" aria-label="Palette examples">
        {paletteTabs.map((tab) => (
          <button
            key={tab.id}
            type="button"
            aria-pressed={activeTab === tab.id}
            onClick={() => chooseTab(tab.id, tab.query)}
          >
            {tab.label}
          </button>
        ))}
      </div>
    </div>
  );
}
