type WindowBounds = { x: number; y: number; width: number; height: number };

export const PALETTE_WINDOW_BLUR_MARGIN = 96;

export function boundsResizedAroundCenter(
  bounds: WindowBounds,
  size: { width: number; height: number },
): WindowBounds {
  return {
    x: Math.round(bounds.x + (bounds.width - size.width) / 2),
    y: Math.round(bounds.y + (bounds.height - size.height) / 2),
    width: size.width,
    height: size.height,
  };
}

/**
 * The palette window carries a transparent blur margin, so it may extend past the work area by that
 * margin, but the visible palette inside it must stay on screen.
 */
export function boundsFittedToWorkArea(
  bounds: WindowBounds,
  workArea: WindowBounds,
  blurMargin: number,
): WindowBounds {
  const width = Math.min(bounds.width, workArea.width + blurMargin * 2);
  const height = Math.min(bounds.height, workArea.height + blurMargin * 2);
  return {
    x: clamp(
      bounds.x,
      workArea.x - blurMargin,
      workArea.x + workArea.width + blurMargin - width,
    ),
    y: clamp(
      bounds.y,
      workArea.y - blurMargin,
      workArea.y + workArea.height + blurMargin - height,
    ),
    width,
    height,
  };
}

function clamp(value: number, min: number, max: number) {
  return Math.round(Math.min(Math.max(value, min), max));
}

export function paletteBoundsResizedInWorkArea(
  bounds: WindowBounds,
  size: { width: number; height: number },
  workArea: WindowBounds,
) {
  return boundsFittedToWorkArea(
    boundsResizedAroundCenter(bounds, size),
    workArea,
    PALETTE_WINDOW_BLUR_MARGIN,
  );
}

export function paletteBoundsPlacedInWorkArea(
  { width, height }: WindowBounds,
  workArea: WindowBounds,
): WindowBounds {
  const placed = {
    x: workArea.x + (workArea.width - width) / 2,
    y: workArea.y + Math.min(workArea.height * 0.18, 180),
    width,
    height,
  };
  return boundsFittedToWorkArea(placed, workArea, PALETTE_WINDOW_BLUR_MARGIN);
}
