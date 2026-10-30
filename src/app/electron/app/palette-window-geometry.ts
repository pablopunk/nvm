type WindowBounds = { x: number; y: number; width: number; height: number };

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
