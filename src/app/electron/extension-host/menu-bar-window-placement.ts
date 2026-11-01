import type { StatusItemAnchor } from '../os/status-item-anchor';

type Rect = { x: number; y: number; width: number; height: number };

const POPOVER_GAP = 6;

function clamp(value: number, min: number, max: number) {
  return Math.min(Math.max(value, min), Math.max(min, max));
}

/** A taskbar at the bottom (Windows default) opens popovers upward. */
function anchorSitsLow(anchor: StatusItemAnchor, workArea: Rect) {
  return anchor.y > workArea.y + workArea.height / 2;
}

/** Centers a popover under (or above) the icon and keeps it on the display. */
export function menuBarWindowBounds(
  anchor: StatusItemAnchor,
  size: { width: number; height: number },
  workArea: Rect,
): Rect {
  const x = clamp(
    Math.round(anchor.x + anchor.width / 2 - size.width / 2),
    workArea.x + POPOVER_GAP,
    workArea.x + workArea.width - size.width - POPOVER_GAP,
  );
  const y = anchorSitsLow(anchor, workArea)
    ? anchor.y - size.height - POPOVER_GAP
    : Math.max(
        workArea.y + POPOVER_GAP,
        anchor.y + anchor.height + POPOVER_GAP,
      );
  return { x, y, width: size.width, height: size.height };
}

/** Without a click anchor, popovers open at the top-right of the display. */
export function fallbackMenuBarAnchor(workArea: Rect): StatusItemAnchor {
  return {
    x: workArea.x + workArea.width - 120,
    y: workArea.y - 1,
    width: 1,
    height: 1,
  };
}
