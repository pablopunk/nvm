export const COMPACT_PANEL_PREFERRED_MAX_HEIGHT = 360;
export const COMPACT_PANEL_BOTTOM_INSET = 12;
const COMPACT_PANEL_TOP_MARGIN = 12;
const COMPACT_PANEL_MIN_HEIGHT = 96;

interface CompactPanelPlacementInput {
  paletteTop: number;
  cardBottom: number;
}

export function compactPanelMaxHeight({
  paletteTop,
  cardBottom,
}: CompactPanelPlacementInput): number {
  const roomAboveCardEdge =
    cardBottom -
    paletteTop -
    COMPACT_PANEL_BOTTOM_INSET -
    COMPACT_PANEL_TOP_MARGIN;
  return Math.round(
    Math.max(
      COMPACT_PANEL_MIN_HEIGHT,
      Math.min(COMPACT_PANEL_PREFERRED_MAX_HEIGHT, roomAboveCardEdge),
    ),
  );
}
