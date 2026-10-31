import { type RefObject, useLayoutEffect, useState } from 'react';
import {
  COMPACT_PANEL_PREFERRED_MAX_HEIGHT,
  compactPanelMaxHeight,
} from './compact-panel-placement';

export function useCompactPanelMaxHeight(
  anchorRef: RefObject<HTMLElement | null>,
): number {
  const [maxHeight, setMaxHeight] = useState(
    COMPACT_PANEL_PREFERRED_MAX_HEIGHT,
  );
  useLayoutEffect(() => {
    const anchor = anchorRef.current;
    const palette = anchor?.parentElement;
    if (!(anchor && palette)) return;
    function measure() {
      if (!(anchor && palette)) return;
      setMaxHeight(
        compactPanelMaxHeight({
          paletteTop: palette.getBoundingClientRect().top,
          cardBottom: anchor.getBoundingClientRect().top,
        }),
      );
    }
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(palette);
    for (const sibling of Array.from(palette.children)) {
      if (sibling !== anchor) observer.observe(sibling);
    }
    return () => observer.disconnect();
  }, [anchorRef]);
  return maxHeight;
}
