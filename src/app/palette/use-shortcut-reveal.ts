import { useEffect } from 'react';
import type { CommandView } from './model';

/**
 * A shortcut-opened palette stays transparent until its view has painted twice,
 * so the window never flashes an empty frame.
 */
export function useShortcutReveal(
  pending: boolean,
  view: CommandView | null,
  setPending: (pending: boolean) => void,
) {
  useEffect(
    function revealAfterPaint() {
      if (!(pending && view)) {
        return;
      }
      const frame = requestAnimationFrame(function firstFrame() {
        requestAnimationFrame(function secondFrame() {
          window.nvm.shortcutReady();
          setPending(false);
        });
      });
      return () => cancelAnimationFrame(frame);
    },
    [pending, view],
  );
}
