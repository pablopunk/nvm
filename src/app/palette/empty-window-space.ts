const PALETTE_SURFACE_SELECTOR =
  '.card, .extensionWindowCompactPanel, .shortcutRegistrationNotice';

type ClosestCapable = { closest: (selector: string) => unknown };

export function mouseDownLandsOnEmptyWindowSpace(
  target: ClosestCapable | null,
) {
  return !target?.closest(PALETTE_SURFACE_SELECTOR);
}
