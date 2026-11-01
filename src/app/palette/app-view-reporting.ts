import { useEffect, useRef, useState } from 'react';
import type { AppView } from '../shared/app-view';
import { isExtensionLoadingView } from './extension-loading-view';
import type { CommandView } from './model';

/**
 * Whether the palette window is on screen, and how it got there. A shortcut can
 * reveal a view directly, without the root palette ever being visible.
 */
export type ViewExposure = 'hidden' | 'palette' | 'shortcut';

export type ActiveViewEntry = { id: string; view: CommandView } | null;

function categoryOf(view: CommandView): AppView {
  return view.aiChat ? 'ai-chat' : 'extension';
}

/**
 * The view a person is looking at once it has settled, or null while nothing
 * countable is visible: hidden window, a shortcut still loading, or no view yet.
 * `key` changes exactly when a new visible navigation happens and stays local;
 * only the category is ever reported.
 */
export function settledAppView(
  exposure: ViewExposure,
  entry: ActiveViewEntry,
): { view: AppView; key: string } | null {
  if (exposure === 'hidden') {
    return null;
  }
  if (!entry) {
    return exposure === 'palette' ? { view: 'palette', key: 'palette' } : null;
  }
  if (isExtensionLoadingView(entry.view)) {
    return null;
  }
  const view = categoryOf(entry.view);
  return { view, key: `${view}:${entry.id}` };
}

/**
 * Reports one `$pageview` per visible navigation: opening, going back, opening
 * a shortcut's resolved view, or reopening the window. Refreshes, loading
 * placeholders and hiding never count.
 */
export function useAppViewReporting(entry: ActiveViewEntry) {
  const [exposure, setExposure] = useState<ViewExposure>('hidden');
  useEffect(function trackExposure() {
    const stopShown = window.nvm.onShown(() => setExposure('palette'));
    const stopShortcutShown = window.nvm.onShortcutShown(() =>
      setExposure((current) => (current === 'hidden' ? 'shortcut' : current)),
    );
    const stopHidden = window.nvm.onHidden(() => setExposure('hidden'));
    return () => {
      stopShown();
      stopShortcutShown();
      stopHidden();
    };
  }, []);
  const settled = settledAppView(exposure, entry);
  const key = settled?.key ?? null;
  const reportedKey = useRef<string | null>(null);
  useEffect(
    function reportSettledView() {
      if (!(settled && key)) {
        reportedKey.current = null;
        return;
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
