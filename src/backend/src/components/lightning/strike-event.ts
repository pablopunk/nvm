export const LIGHTNING_STRIKE_EVENT = 'nevermind:lightning-strike';

/** Asks the page's lightning accent to strike at a viewport position. */
export function requestLightningStrike(clientX: number, clientY: number) {
  window.dispatchEvent(
    new CustomEvent(LIGHTNING_STRIKE_EVENT, {
      detail: { x: clientX, y: clientY },
    }),
  );
}
