import { Command } from 'cmdk';
import {
  Activity,
  AppWindow,
  Download,
  Clipboard,
  CircleStop,
  FileCode,
  FileSearch,
  FileText,
  Folder,
  History,
  Image,
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
import DictationIndicator from './DictationIndicator';
import {
  DICTATED_TEXT,
  paletteItems,
  paletteTabs,
  RELEASES_URL,
} from './palette-data';
import { searchPalette } from './palette-search';
import { landingPaletteCss } from './palette-styles';
import type { PaletteItem } from './palette-types';

const ICONS: Record<string, LucideIcon> = {
  activity: Activity,
  'app-window': AppWindow,
  download: Download,
  clipboard: Clipboard,
  'file-code': FileCode,
  'file-search': FileSearch,
  'file-text': FileText,
  folder: Folder,
  history: History,
  image: Image,
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
  'stop-circle': CircleStop,
  terminal: Terminal,
  type: Type,
};

const TYPING_INTERVAL_MS = 55;
const DOWNLOAD_NOTICE_MS = 4000;
const DOWNLOAD_NOTICE_TEXT = 'Download the app to actually do that!';
/** Notice target for the "Tab to ask AI" hint; item ids identify result rows. */
const ASK_AI_TARGET = 'ask-ai';
const START_DICTATION_ID = 'command:Start Dictation';

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
      <span className={item.image ? 'resultIcon appIcon' : 'resultIcon'}>
        {item.image ? (
          <img src={item.image} alt="" width={32} height={32} decoding="async" />
        ) : (
          iconFor(item)
        )}
      </span>
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
  noticeTarget,
  onSelectItem,
  onAskAi,
  inputRef,
}: {
  query: string;
  onQueryChange: (value: string) => void;
  noticeTarget: string | null;
  onSelectItem: (item: PaletteItem) => void;
  onAskAi: () => void;
  inputRef: React.RefObject<HTMLInputElement | null>;
}) {
  const results = useMemo(() => searchPalette(paletteItems, query), [query]);
  function handlePaletteKeyDown(event: KeyboardEvent<HTMLElement>) {
    strikeOnEnter(event);
    if (event.key === 'Tab' && !event.shiftKey && query) {
      event.preventDefault();
      onAskAi();
    }
  }
  return (
    <Command
      className="palette fitContent"
      label="Nevermind"
      loop
      shouldFilter={false}
      onKeyDown={handlePaletteKeyDown}
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
            {noticeTarget === ASK_AI_TARGET ? (
              <a href={RELEASES_URL} role="status">
                {DOWNLOAD_NOTICE_TEXT}
              </a>
            ) : (
              <>
                <Sparkles size={12} aria-hidden="true" /> <kbd>Tab</kbd> to ask
                AI
              </>
            )}
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
              showNotice={item.id === noticeTarget}
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
  const [noticeTarget, setNoticeTarget] = useState<string | null>(null);
  const [dictationRun, setDictationRun] = useState(0);
  const [dictating, setDictating] = useState(false);
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
      container.style.position = 'relative';
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
    setNoticeTarget(null);
  }, []);
  useEffect(() => hideNotice, [hideNotice]);

  function showNotice(target: string) {
    if (noticeTimer.current) clearTimeout(noticeTimer.current);
    setNoticeTarget(target);
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

  const stopDictating = useCallback(function stopDictating() {
    setDictating(false);
  }, []);

  function pasteDictation() {
    typeFreely(DICTATED_TEXT);
  }

  function selectItem(item: PaletteItem) {
    if (item.id === START_DICTATION_ID) {
      hideNotice();
      setDictationRun((run) => run + 1);
      setDictating(true);
      return;
    }
    if (item.url) {
      hideNotice();
      window.open(item.url, '_blank', 'noopener,noreferrer');
      return;
    }
    showNotice(item.id);
  }

  return (
    <div className="landing-palette">
      <div ref={hostRef} className="landing-palette__host" />
      {mount
        ? createPortal(
            <>
            <PaletteApp
              query={query}
              onQueryChange={typeFreely}
              noticeTarget={noticeTarget}
              onSelectItem={selectItem}
              onAskAi={() => showNotice(ASK_AI_TARGET)}
              inputRef={inputRef}
            />
            {dictating ? (
              <DictationIndicator
                key={dictationRun}
                onPasted={pasteDictation}
                onDone={stopDictating}
              />
            ) : null}
            </>,
            mount,
          )
        : null}
      <div className="landing-palette__tabs" role="group" aria-label="Palette examples">
        {paletteTabs.map((tab) => {
          const TabIcon = (tab.icon && ICONS[tab.icon]) || Sparkles;
          return (
            <button
              key={tab.id}
              type="button"
              aria-pressed={activeTab === tab.id}
              onClick={() => chooseTab(tab.id, tab.query)}
            >
              <TabIcon size={14} aria-hidden="true" />
              {tab.label}
            </button>
          );
        })}
      </div>
    </div>
  );
}
