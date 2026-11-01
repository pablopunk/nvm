export type StatusItemAnchor = {
  x: number;
  y: number;
  width: number;
  height: number;
};

let lastAnchor: StatusItemAnchor | null = null;

/** Where the user last clicked a menu bar or tray icon, so popovers open under it. */
export function rememberStatusItemAnchor(anchor: StatusItemAnchor | null) {
  lastAnchor = anchor;
}

export function lastStatusItemAnchor() {
  return lastAnchor;
}

export function statusItemAnchorFrom(
  bounds: Partial<StatusItemAnchor> | undefined,
  cursor: { x: number; y: number },
): StatusItemAnchor {
  if (bounds?.width && bounds.height)
    return {
      x: Number(bounds.x) || 0,
      y: Number(bounds.y) || 0,
      width: bounds.width,
      height: bounds.height,
    };
  return { x: cursor.x, y: cursor.y, width: 1, height: 1 };
}
