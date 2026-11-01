import { useEffect, useRef, useState } from 'react';
import type { AppView } from '../shared/app-view';
import { isExtensionLoadingView } from './extension-loading-view';
import { paneView, type PaneFrame } from './extension-view/pane-navigation';
import type { CommandView } from './model';

/**
 * Whether the palette window is on screen, and how it got there. A shortcut can
 * reveal a view directly, without the root palette ever being visible. While
 * the window is revealed by a shortcut, `staleEntryId` names the view that was
 * retained from before and must not count as the shortcut's own view.
 */
export type ViewExposure = {
  mode: 'hidden' | 'palette' | 'shortcut';
  staleEntryId?: string;
};

const HIDDEN: ViewExposure = { mode: 'hidden' };
const PALETTE: ViewExposure = { mode: 'palette' };

function categoryOf(view: CommandView): AppView {
  return view.aiChat ? 'ai-chat' : 'extension';
}

/**
 * The view a person is looking at once it has settled, or null while nothing
 * countable is visible: hidden window, a shortcut still loading, or no view yet.
 * `key` changes exactly when a new visible navigation happens (a different pane,
 * a deeper or shallower history, or another view) and stays local; replacing a
 * view with a fresh copy of itself is a refresh and keeps the key. Only the
 * category is ever reported.
 */
export function settledAppView(
  exposure: ViewExposure,
  frame: PaneFrame | undefined,
): { view: AppView; key: string } | null {
  if (exposure.mode === 'hidden') {
    return null;
  }
  const entry = paneView(frame);
  if (!(frame && entry)) {
    return exposure.mode === 'palette'
      ? { view: 'palette', key: 'palette' }
      : null;
  }
  if (
    isExtensionLoadingView(entry.view) ||
    (exposure.mode === 'shortcut' && entry.id === exposure.staleEntryId)
  ) {
    return null;
  }
  const view = categoryOf(entry.view);
  return {
    view,
    key: `${view}:${frame.id}:${frame.history.length}:${entry.view.id}`,
  };
}

/**
 * Reports one `$pageview` per visible navigation: opening, going back, opening
 * a shortcut's resolved view, or reopening the window. Refreshes, loading
 * placeholders and hiding never count.
 */
export function useAppViewReporting(frame: PaneFrame | undefined) {
  const [exposure, setExposure] = useState<ViewExposure>(HIDDEN);
  const currentEntryId = useRef<string | undefined>(undefined);
  currentEntryId.current = paneView(frame)?.id;
  useEffect(function trackExposure() {
    const stopShown = window.nvm.onShown(() => setExposure(PALETTE));
    const stopShortcutShown = window.nvm.onShortcutShown(() =>
      setExposure((current) =>
        current.mode === 'hidden'
          ? { mode: 'shortcut', staleEntryId: currentEntryId.current }
          : current,
      ),
    );
    const stopHidden = window.nvm.onHidden(() => setExposure(HIDDEN));
    return () => {
      stopShown();
      stopShortcutShown();
      stopHidden();
    };
  }, []);
  const settled = settledAppView(exposure, frame);
  const key = settled?.key ?? null;
  const reportedKey = useRef<string | null>(null);
  useEffect(
    function reportSettledView() {
      if (!(settled && key)) {
        reportedKey.current = null;
        return;
      }
      if (exposure.mode === 'shortcut') {
        // Once the shortcut's own view is up, the window behaves like any
        // visible palette, so later root visits count too.
        setExposure(PALETTE);
      }
      if (reportedKey.current === key) {
        return;
      }
      reportedKey.current = key;
      window.nvm.reportAppView(settled.view);
    },
    [key],
  );
}
