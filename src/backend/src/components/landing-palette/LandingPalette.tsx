import {
  type KeyboardEvent,
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from 'react';
import { createPortal } from 'react-dom';
import ActionMenu from './ActionMenu';
import ChatPage, { type ChatMode } from './ChatPage';
import DictationIndicator from './DictationIndicator';
import PaletteFooter from './PaletteFooter';
import SearchPage from './SearchPage';
import {
  CHAT_ACTIONS,
  isAutomate,
  type MenuAction,
  menuActions,
  primaryTitle,
  withQueryFallbacks,
} from './palette-actions';
import { DICTATE_ID, DICTATED_TEXT, paletteItems, paletteTabs } from './palette-data';
import { iconNamed } from './palette-icons';
import { searchPalette } from './palette-search';
import { landingPaletteCss } from './palette-styles';
import type { PaletteItem } from './palette-types';

const TYPING_INTERVAL_MS = 55;
const DOWNLOAD_NOTICE_MS = 4000;
/** Notice target of the chat page's Send; item ids identify result rows. */
const SEND_TARGET = 'send';

interface ChatState {
  mode: ChatMode;
  query: string;
}

function isActionsShortcut(event: KeyboardEvent<HTMLElement>) {
  return (event.metaKey || event.ctrlKey) && !event.altKey && event.key.toLowerCase() === 'k';
}

export default function LandingPalette() {
  const hostRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const composerRef = useRef<HTMLTextAreaElement>(null);
  const typingTimer = useRef<ReturnType<typeof setInterval> | null>(null);
  const noticeTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  /** Set when the chat page closes, so the remounted search field takes the keyboard back. */
  const leavingChat = useRef(false);
  const [mount, setMount] = useState<HTMLElement | null>(null);
  const [query, setQuery] = useState('');
  const [selectedId, setSelectedId] = useState<string>();
  const [chat, setChat] = useState<ChatState | null>(null);
  const [menuOpen, setMenuOpen] = useState(false);
  const [activeTab, setActiveTab] = useState<string | null>(paletteTabs[0].id);
  const [noticeTarget, setNoticeTarget] = useState<string | null>(null);
  const [dictationRun, setDictationRun] = useState(0);
  const [dictating, setDictating] = useState(false);

  const rows = useMemo(
    () => withQueryFallbacks(searchPalette(paletteItems, query), query),
    [query],
  );
  const selected = rows.find((row) => row.id === selectedId) ?? rows[0];

  useLayoutEffect(function attachShadowRoot() {
    const host = hostRef.current;
    if (!host) return;
    const shadow = host.shadowRoot ?? host.attachShadow({ mode: 'open' });
    shadow.replaceChildren();
    const style = document.createElement('style');
    style.textContent = landingPaletteCss;
    const container = document.createElement('div');
    container.className = 'stage';
    shadow.append(style, container);
    setMount(container);
  }, []);

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

  function focusPage() {
    const field = chat ? composerRef.current : inputRef.current;
    field?.focus({ preventScroll: true });
  }

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
  }

  function chooseTab(tabId: string, tabQuery: string) {
    setActiveTab(tabId);
    setChat(null);
    setMenuOpen(false);
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

  function openChat(mode: ChatMode) {
    const text = query.trim();
    if (!text) return;
    stopTyping();
    hideNotice();
    setMenuOpen(false);
    setChat({ mode, query: text });
  }

  function backToSearch() {
    hideNotice();
    setMenuOpen(false);
    setChat(null);
    leavingChat.current = true;
  }
  useEffect(
    function focusFieldAfterLeavingChat() {
      if (chat || !leavingChat.current) return;
      leavingChat.current = false;
      inputRef.current?.focus({ preventScroll: true });
    },
    [chat],
  );

  function runItem(item: PaletteItem) {
    if (item.id === DICTATE_ID) {
      hideNotice();
      setDictationRun((run) => run + 1);
      setDictating(true);
      return;
    }
    if (isAutomate(item)) {
      openChat('builder');
      return;
    }
    if (item.url) {
      hideNotice();
      window.open(item.url, '_blank', 'noopener,noreferrer');
      return;
    }
    showNotice(item.id);
  }

  function runPrimary() {
    if (chat) showNotice(SEND_TARGET);
    else if (selected) runItem(selected);
  }

  function closeMenu(refocus: boolean) {
    setMenuOpen(false);
    if (refocus) focusPage();
  }

  function runMenuAction(action: MenuAction) {
    closeMenu(true);
    if (action.primary) runPrimary();
    else showNotice(chat ? SEND_TARGET : (selected?.id ?? SEND_TARGET));
  }

  function toggleMenuOnShortcut(event: KeyboardEvent<HTMLElement>) {
    if (!isActionsShortcut(event)) return;
    event.preventDefault();
    if (menuOpen) closeMenu(true);
    else setMenuOpen(true);
  }

  const actions = chat ? CHAT_ACTIONS : selected ? menuActions(selected) : [];
  const footerPrimary = chat ? 'Send' : selected ? primaryTitle(selected) : null;

  return (
    <div className="landing-palette">
      <div ref={hostRef} className="landing-palette__host" />
      {mount
        ? createPortal(
            <>
              <div className="pane" onKeyDown={toggleMenuOnShortcut}>
                {chat ? (
                  <ChatPage
                    mode={chat.mode}
                    query={chat.query}
                    showNotice={noticeTarget === SEND_TARGET}
                    composerRef={composerRef}
                    onSend={() => showNotice(SEND_TARGET)}
                    onBack={backToSearch}
                  />
                ) : (
                  <SearchPage
                    query={query}
                    rows={rows}
                    selectedId={selected?.id}
                    noticeTarget={noticeTarget}
                    inputRef={inputRef}
                    onQueryChange={typeFreely}
                    onSelectedChange={setSelectedId}
                    onRun={runItem}
                    onAskAi={() => openChat('chat')}
                  />
                )}
                <PaletteFooter
                  primary={footerPrimary}
                  chat={chat ? { query: chat.query, onBack: backToSearch } : undefined}
                  onPrimary={runPrimary}
                  onActions={() => (menuOpen ? closeMenu(true) : setMenuOpen(true))}
                />
                {menuOpen && actions.length > 0 ? (
                  <ActionMenu actions={actions} onRun={runMenuAction} onClose={closeMenu} />
                ) : null}
              </div>
              {dictating ? (
                <DictationIndicator
                  key={dictationRun}
                  onPasted={() => typeFreely(DICTATED_TEXT)}
                  onDone={() => setDictating(false)}
                />
              ) : null}
            </>,
            mount,
          )
        : null}
      <div className="landing-palette__tabs" role="group" aria-label="Palette examples">
        {paletteTabs.map(function renderTab(tab) {
          const TabIcon = iconNamed(tab.icon);
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
